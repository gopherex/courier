// Package delivery implements the SMTP, WebPush and FCM transports.
package delivery

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net"
	"net/http"
	"net/mail"
	"net/smtp"
	"net/textproto"
	"strconv"
	"strings"
	"time"

	outbox "github.com/gopherex/pg-outbox"

	"github.com/gopherex/courier/pkg/api"
)

// ErrTransport is safe to persist and log: provider responses may contain recipients or secrets.
var (
	ErrTransport = errors.New("provider_unavailable")
	// ErrRejected identifies a permanent provider rejection without preserving its body.
	ErrRejected = errors.New("provider_rejected")
	// ErrSettings identifies malformed provider configuration.
	ErrSettings = errors.New("invalid_provider")
)

// Sender contains runtime transport policy. Configuration is supplied on every attempt.
type Sender struct {
	AllowPlainSMTP bool
	HTTPClient     *http.Client
}

// Send executes exactly one provider attempt.
func (sender *Sender) Send(ctx context.Context, settings *api.ProviderSettings, snapshot *api.Snapshot) error {
	switch snapshot.Channel {
	case api.ChannelEmail:
		return sendSMTP(ctx, &settings.SMTP.Value, snapshot)
	case api.ChannelWebpush:
		return sendWebPush(ctx, &settings.Webpush.Value, snapshot, sender.HTTPClient)
	case api.ChannelFcm:
		return sendFCM(ctx, &settings.Fcm.Value, snapshot)
	default:
		return fmt.Errorf("delivery: %w", outbox.Permanent(ErrSettings))
	}
}

func smtpResult(err error) error {
	if err == nil {
		return nil
	}

	var response *textproto.Error
	if errors.As(err, &response) && response.Code >= 500 {
		return fmt.Errorf("delivery: %w", outbox.Permanent(ErrRejected))
	}

	return ErrTransport
}

func connectSMTP(ctx context.Context, settings *api.SMTPSettings) (*smtp.Client, func(), error) {
	address := net.JoinHostPort(settings.Host, strconv.Itoa(settings.Port))

	connection, err := (&net.Dialer{}).DialContext(ctx, "tcp", address)
	if err != nil {
		return nil, nil, ErrTransport
	}

	socket := connection
	stop := context.AfterFunc(ctx, func() { _ = socket.Close() })
	cleanup := func() { stop(); _ = socket.Close() }

	deadline, ok := ctx.Deadline()
	if !ok {
		deadline = time.Now().Add(smtpTimeout)
	}

	if err = connection.SetDeadline(deadline); err != nil {
		cleanup()
		return nil, nil, ErrTransport
	}

	tlsConfig := &tls.Config{ServerName: settings.Host, MinVersion: tls.VersionTLS12}
	if settings.TLS == api.SMTPSettingsTLSTLS {
		connection = tls.Client(connection, tlsConfig)
	}

	client, err := smtp.NewClient(connection, settings.Host)
	if err != nil {
		cleanup()
		return nil, nil, smtpResult(err)
	}

	if settings.TLS == api.SMTPSettingsTLSStarttls {
		if err = client.StartTLS(tlsConfig); err != nil {
			cleanup()
			return nil, nil, smtpResult(err)
		}
	}

	if settings.Username.Value != "" {
		if err = client.Auth(smtp.PlainAuth("",
			settings.Username.Value,
			settings.Password.Value,
			settings.Host)); err != nil {
			cleanup()
			return nil, nil, smtpResult(err)
		}
	}

	return client, cleanup, nil
}

func sendSMTP(ctx context.Context, settings *api.SMTPSettings, snapshot *api.Snapshot) error {
	content, err := emailBytes(settings, snapshot)
	if err != nil {
		return fmt.Errorf("delivery: %w", outbox.Permanent(ErrSettings))
	}

	client, cleanup, err := connectSMTP(ctx, settings)
	if err != nil {
		return err
	}
	defer cleanup()

	from, err := mail.ParseAddress(settings.From)
	if err != nil {
		return fmt.Errorf("delivery: %w", outbox.Permanent(ErrSettings))
	}

	if err = client.Mail(from.Address); err != nil {
		return smtpResult(err)
	}

	if err = client.Rcpt(snapshot.Email.Value.Address); err != nil {
		return smtpResult(err)
	}

	writer, err := client.Data()
	if err != nil {
		return smtpResult(err)
	}

	if _, err = writer.Write(content); err != nil {
		return smtpResult(err)
	}

	if err = writer.Close(); err != nil {
		return smtpResult(err)
	}
	// A successful DATA reply is the delivery boundary. QUIT failure must not retry the message.
	_ = client.Quit()

	return nil
}

func emailBytes(settings *api.SMTPSettings, snapshot *api.Snapshot) ([]byte, error) {
	var content bytes.Buffer

	from, err := mail.ParseAddress(settings.From)
	if err != nil {
		return nil, ErrSettings
	}

	headers := [][2]string{
		{"From", from.String()},
		{"To", snapshot.Email.Value.Address},
		{"Subject", mime.QEncoding.Encode("UTF-8", snapshot.Rendered.Subject)},
		{"Date", snapshot.CreatedAt.UTC().Format(time.RFC1123Z)},
		{"Message-ID", "<" + snapshot.MessageID.String() + "@courier.local>"},
		{"MIME-Version", "1.0"},
	}
	if snapshot.Rendered.ReplyTo != "" {
		headers = append(headers, [2]string{"Reply-To", snapshot.Rendered.ReplyTo})
	}

	for _, header := range headers {
		if strings.ContainsAny(header[1], "\r\n") {
			return nil, ErrSettings
		}

		content.WriteString(header[0] + ": " + header[1] + "\r\n")
	}

	multi := multipart.NewWriter(&content)
	if err = multi.SetBoundary("courier-" + snapshot.MessageID.String()); err != nil {
		return nil, ErrSettings
	}

	content.WriteString("Content-Type: multipart/alternative; boundary=" + multi.Boundary() + "\r\n\r\n")

	for _, part := range [][2]string{{"text/plain", snapshot.Rendered.Text}, {"text/html", snapshot.Rendered.HTML}} {
		if part[1] == "" {
			continue
		}

		if partErr := writePart(multi, part[0], part[1]); partErr != nil {
			return nil, partErr
		}
	}

	if err = multi.Close(); err != nil {
		return nil, fmt.Errorf("close MIME: %w", err)
	}

	return content.Bytes(), nil
}

func writePart(writer *multipart.Writer, kind, body string) error {
	part, err := writer.CreatePart(textproto.MIMEHeader{
		"Content-Type": {kind + "; charset=UTF-8"}, "Content-Transfer-Encoding": {"quoted-printable"},
	})
	if err != nil {
		return fmt.Errorf("create MIME: %w", err)
	}

	encoded := quotedprintable.NewWriter(part)
	if _, err = io.WriteString(encoded, body); err != nil {
		return fmt.Errorf("encode MIME: %w", err)
	}

	if err = encoded.Close(); err != nil {
		return fmt.Errorf("close MIME encoding: %w", err)
	}

	return nil
}

const smtpTimeout = 30 * time.Second
