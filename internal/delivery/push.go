package delivery

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	webpush "github.com/SherClockHolmes/webpush-go"
	outbox "github.com/gopherex/pg-outbox"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/fcm/v1"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"

	"github.com/gopherex/courier/pkg/api"
)

func pushTTL(snapshot *api.Snapshot) int {
	const defaultTTL = 3600
	if expiry, ok := snapshot.ExpiresAt.Get(); ok {
		return max(0, min(defaultTTL, int(time.Until(expiry).Seconds())))
	}

	return defaultTTL
}

func sendWebPush(ctx context.Context, settings *api.WebPushSettings,
	snapshot *api.Snapshot, client *http.Client,
) error {
	if client == nil {
		client = &http.Client{CheckRedirect: rejectRedirect}
	}

	data, err := json.Marshal(snapshot.Rendered)
	if err != nil {
		return fmt.Errorf("delivery: %w", outbox.Permanent(ErrSettings))
	}

	subscription := &webpush.Subscription{
		Endpoint: snapshot.Webpush.Value.Endpoint.String(),
		Keys:     webpush.Keys{P256dh: snapshot.Webpush.Value.P256dh, Auth: snapshot.Webpush.Value.Auth},
	}

	response, err := webpush.SendNotificationWithContext(ctx, data, subscription, &webpush.Options{
		Subscriber:     strings.TrimPrefix(settings.Subject, "mailto:"),
		VAPIDPublicKey: settings.PublicKey, VAPIDPrivateKey: settings.PrivateKey,
		TTL: pushTTL(snapshot), HTTPClient: client,
	})
	if err != nil {
		return ErrTransport
	}
	defer func() { _ = response.Body.Close() }()

	return httpResult(response.StatusCode, response.Header)
}

func rejectRedirect(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }

func sendFCM(ctx context.Context, settings *api.FCMSettings, snapshot *api.Snapshot) error {
	raw, err := json.Marshal(settings.ServiceAccount)
	if err != nil {
		return fmt.Errorf("delivery: %w", outbox.Permanent(ErrSettings))
	}

	credentials, err := google.JWTConfigFromJSON(raw, fcm.FirebaseMessagingScope)
	if err != nil {
		return fmt.Errorf("delivery: %w", outbox.Permanent(ErrSettings))
	}

	service, err := fcm.NewService(ctx, option.WithHTTPClient(credentials.Client(ctx)))
	if err != nil {
		return ErrTransport
	}

	expires := time.Now().Add(time.Duration(pushTTL(snapshot)) * time.Second).Unix()
	request := &fcm.SendMessageRequest{Message: &fcm.Message{
		Token:        snapshot.Fcm.Value.Token,
		Notification: &fcm.Notification{Title: snapshot.Rendered.Title, Body: snapshot.Rendered.Body},
		Data:         map[string]string{"url": snapshot.Rendered.URL, "locale": snapshot.Rendered.Locale},
		Android:      &fcm.AndroidConfig{Ttl: strconv.Itoa(pushTTL(snapshot)) + "s"},
		Apns: &fcm.ApnsConfig{Headers: map[string]string{"apns-expiration": strconv.FormatInt(expires,
			10)}},
	}}

	_, err = service.Projects.Messages.Send("projects/"+settings.ProjectID, request).Context(ctx).Do()
	if err == nil {
		return nil
	}

	var response *googleapi.Error
	if errors.As(err, &response) {
		return httpResult(response.Code, response.Header)
	}

	return ErrTransport
}

func httpResult(status int, headers http.Header) error {
	if status >= 200 && status < 300 {
		return nil
	}

	if status == http.StatusTooManyRequests || status == http.StatusRequestTimeout || status >= 500 {
		minimum := time.Minute
		if seconds,
			err := strconv.Atoi(headers.Get("Retry-After")); err == nil && seconds > 0 &&
			seconds < int(time.Duration(1<<63-1)/time.Second) {
			minimum = max(minimum, time.Duration(seconds)*time.Second)
		} else if date, parseErr := http.ParseTime(headers.Get("Retry-After")); parseErr == nil {
			minimum = max(minimum, time.Until(date))
		}

		return fmt.Errorf("delivery: %w", outbox.RetryAfter(ErrTransport, minimum))
	}

	return fmt.Errorf("delivery: %w", outbox.Permanent(ErrRejected))
}
