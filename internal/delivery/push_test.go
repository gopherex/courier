package delivery

import (
	"context"
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	webpush "github.com/SherClockHolmes/webpush-go"
	outbox "github.com/gopherex/pg-outbox"

	"github.com/gopherex/courier/pkg/api"
)

type transportFunc func(*http.Request) (*http.Response, error)

func (transport transportFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return transport(request)
}

func TestWebPushEncryptionAndVAPIDContact(t *testing.T) {
	t.Parallel()

	private, public, err := webpush.GenerateVAPIDKeys()
	if err != nil {
		t.Fatal(err)
	}

	recipient, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	endpoint, err := url.Parse("https://push.example.test/subscription")
	if err != nil {
		t.Fatal(err)
	}

	snapshot := &api.Snapshot{Channel: api.ChannelWebpush, Webpush: api.NewOptWebPushTarget(api.WebPushTarget{
		Endpoint: *endpoint, P256dh: base64.RawURLEncoding.EncodeToString(recipient.PublicKey().Bytes()),
		Auth: base64.RawURLEncoding.EncodeToString(make([]byte, 16)),
	}), Rendered: api.Rendered{Title: "Private title", Body: "Private body"}}
	settings := &api.WebPushSettings{Subject: "mailto:operator@example.test", PublicKey: public, PrivateKey: private}
	observed := false
	client := &http.Client{Transport: transportFunc(func(request *http.Request) (*http.Response, error) {
		observed = true

		body, readErr := io.ReadAll(request.Body)
		if readErr != nil {
			return nil, fmt.Errorf("read encrypted body: %w", readErr)
		}

		if strings.Contains(string(body), "Private body") || request.Header.Get("Content-Encoding") != "aes128gcm" {
			t.Error("unencrypted WebPush body")
		}

		header := request.Header.Get("Authorization")

		token, _, ok := strings.Cut(strings.TrimPrefix(header, "vapid t="), ", k=")
		if !ok {
			t.Error("missing VAPID authorization")
		}

		segments := strings.Split(token, ".")
		if len(segments) != 3 {
			return nil, errors.New("invalid VAPID token")
		}

		raw, decodeErr := base64.RawURLEncoding.DecodeString(segments[1])
		if decodeErr != nil {
			return nil, fmt.Errorf("decode VAPID: %w", decodeErr)
		}

		claims := make(map[string]any)
		if decodeErr = json.Unmarshal(raw, &claims); decodeErr != nil {
			return nil, fmt.Errorf("decode claims: %w", decodeErr)
		}

		if claims["sub"] != settings.Subject || claims["aud"] != "https://push.example.test" {
			t.Errorf("unexpected VAPID claims: %v", claims)
		}

		return &http.Response{
			StatusCode: http.StatusCreated, Header: make(http.Header),
			Body: io.NopCloser(strings.NewReader("")), Request: request,
		}, nil
	})}

	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()

	if err = sendWebPush(ctx, settings, snapshot, client); err != nil {
		t.Fatal(err)
	}

	if !observed {
		t.Fatal("push transport was never invoked")
	}
}

func TestProviderRetryAfterAndPermanentErrors(t *testing.T) {
	t.Parallel()

	headers := http.Header{"Retry-After": {"120"}}
	retry := httpResult(http.StatusTooManyRequests, headers)

	var delayed *outbox.RetryAfterError
	if !errors.As(retry, &delayed) {
		t.Fatal("rate limit did not return delayed retry")
	}

	var permanent *outbox.PermanentError
	if !errors.As(httpResult(http.StatusGone, nil), &permanent) {
		t.Fatal("expired push subscription should be permanent")
	}

	if errors.As(httpResult(http.StatusRequestTimeout, nil), &permanent) {
		t.Fatal("request timeout must retry")
	}

	if err := httpResult(http.StatusOK, nil); err != nil {
		t.Fatal(err)
	}
}
