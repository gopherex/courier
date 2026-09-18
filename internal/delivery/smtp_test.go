package delivery

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/textproto"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	outbox "github.com/gopherex/pg-outbox"

	"github.com/gopherex/courier/pkg/api"
)

func TestSMTPSuccessSurvivesQuitFailure(t *testing.T) {
	t.Parallel()

	listener, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() { _ = listener.Close() })

	received := make(chan string, 1)

	serverErrors := make(chan error, 1)
	go func() { serverErrors <- smtpFixture(listener, received) }()

	host, port, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}

	number, err := strconv.Atoi(port)
	if err != nil {
		t.Fatal(err)
	}

	snapshot := &api.Snapshot{
		MessageID: uuid.New(), Channel: api.ChannelEmail, CreatedAt: time.Now().UTC(),
		Email:    api.NewOptEmailTarget(api.EmailTarget{Address: "recipient@example.test"}),
		Rendered: api.Rendered{Subject: "Hello", Text: "Exact body", HTML: "<b>Exact body</b>"},
	}
	settings := &api.SMTPSettings{Host: host, Port: number, TLS: api.SMTPSettingsTLSPlain, From: "sender@example.test"}

	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()

	if err = sendSMTP(ctx, settings, snapshot); err != nil {
		t.Fatalf("successful DATA was retried after failed QUIT: %v", err)
	}

	if err = <-serverErrors; err != nil {
		t.Fatal(err)
	}

	content := <-received
	if !strings.Contains(content, "Exact body") || !strings.Contains(content, snapshot.MessageID.String()) {
		t.Fatal("invalid MIME payload")
	}

	first, err := emailBytes(settings, snapshot)
	if err != nil {
		t.Fatal(err)
	}

	second, err := emailBytes(settings, snapshot)
	if err != nil || !bytes.Equal(first, second) {
		t.Fatal("retry changed MIME, Message-ID or date")
	}
}

func smtpFixture(listener net.Listener, received chan<- string) error {
	connection, err := listener.Accept()
	if err != nil {
		return fmt.Errorf("SMTP fixture: %w", err)
	}
	defer func() { _ = connection.Close() }()

	if writeErr := connection.SetDeadline(time.Now().Add(3 * time.Second)); writeErr != nil {
		return fmt.Errorf("SMTP fixture write: %w", writeErr)
	}

	wire := textproto.NewConn(connection)
	if writeErr := wire.PrintfLine("220 local SMTP"); writeErr != nil {
		return fmt.Errorf("SMTP fixture write: %w", writeErr)
	}

	for {
		line, readErr := wire.ReadLine()
		if readErr != nil {
			return fmt.Errorf("SMTP fixture: %w", readErr)
		}

		switch {
		case strings.HasPrefix(line, "DATA"):
			if writeErr := wire.PrintfLine("354 send data"); writeErr != nil {
				return fmt.Errorf("SMTP fixture write: %w", writeErr)
			}

			raw, dataErr := io.ReadAll(wire.DotReader())
			if dataErr != nil {
				return fmt.Errorf("SMTP fixture: %w", dataErr)
			}

			received <- string(raw)

			if writeErr := wire.PrintfLine("250 accepted"); writeErr != nil {
				return fmt.Errorf("SMTP fixture write: %w", writeErr)
			}
		case strings.HasPrefix(line, "QUIT"):
			return nil // Disconnect deliberately without a QUIT response.
		default:
			if writeErr := wire.PrintfLine("250 OK"); writeErr != nil {
				return fmt.Errorf("SMTP fixture write: %w", writeErr)
			}
		}
	}
}

func TestSMTPTemporaryAndPermanentClassification(t *testing.T) {
	t.Parallel()

	var permanent *outbox.PermanentError
	if !errors.As(smtpResult(&textproto.Error{Code: 550, Msg: "secret@example.test"}), &permanent) {
		t.Fatal("550 is not permanent")
	}

	retry := smtpResult(&textproto.Error{Code: 450, Msg: "secret@example.test"})
	if errors.As(retry, &permanent) || strings.Contains(retry.Error(), "secret") {
		t.Fatal("transient failure leaked provider text or became permanent")
	}
}
