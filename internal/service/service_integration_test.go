//go:build integration

package service

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	outbox "github.com/gopherex/pg-outbox"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gopherex/courier/internal/postgres"
	"github.com/gopherex/courier/pkg/api"
	"github.com/gopherex/courier/pkg/sdk"
)

//nolint:gosec // Docker arguments are fixed test fixtures and IDs from the local Docker daemon.
func docker(t *testing.T, image, port string, environment ...string) string {
	t.Helper()

	args := make([]string, 0, 6+2*len(environment))

	args = append(args, "run", "--rm", "-d", "-p", "127.0.0.1::"+port)
	for _, variable := range environment {
		args = append(args, "-e", variable)
	}

	args = append(args, image)

	raw, err := exec.CommandContext(t.Context(), "docker", args...).CombinedOutput()
	if err != nil {
		t.Fatalf("docker: %s: %v", raw, err)
	}

	id := strings.TrimSpace(string(raw))

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()

		if output, removeErr := exec.CommandContext(ctx, "docker", "rm", "-f", id).CombinedOutput(); removeErr != nil {
			t.Errorf("remove container: %s %v", output, removeErr)
		}
	})

	raw, err = exec.CommandContext(t.Context(), "docker", "port", id, port).CombinedOutput()
	if err != nil {
		t.Fatalf("container port: %s %v", raw, err)
	}

	return strings.TrimSpace(string(raw))
}

func setup(t *testing.T) *Service {
	t.Helper()
	address := docker(t, "postgres:16-alpine", "5432", "POSTGRES_PASSWORD=courier", "POSTGRES_DB=courier")

	pool, err := pgxpool.New(t.Context(), "postgres://postgres:courier@"+address+"/courier?sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(pool.Close)

	deadline := time.Now().Add(time.Minute)
	for pool.Ping(t.Context()) != nil {
		if time.Now().After(deadline) {
			t.Fatal("PostgreSQL did not become ready")
		}

		time.Sleep(100 * time.Millisecond)
	}

	if err = postgres.Migrate(t.Context(), pool); err != nil {
		t.Fatal(err)
	}

	config := Config{
		AdminKey: strings.Repeat("a", 32), EncryptionKey: []byte(strings.Repeat("k", 32)),
		Development: true, PublicURL: "http://localhost:8080", Workers: 2, MaxAttempts: 3,
		AttemptTimeout: time.Second,
		LeaseDuration:  5 * time.Second,
		DedupRetention: time.Hour,
		DeadRetention:  time.Hour,
	}

	service, err := New(pool, &config)
	if err != nil {
		t.Fatal(err)
	}

	return service
}

func fixture(t *testing.T, service *Service, project string) {
	t.Helper()

	var config api.ProjectConfig

	raw := `{"title":{"en":"Test","ru":"Тест"},"default_locale":"en","notifications":[{
 "key":"registration","active":true,"title":{"en":"Registration"},"description":{},"channels":[{
 "channel":"email",
  "schema":{"id":{"name":"registration"},
  "strict":true,
  "fields":[{"name":"name",
  "required":true,
  "string":{}}]},

 "templates":[{"locale":"en","subject":"Hello {{.name}}","text":"Original {{.name}}"}]}]}]}`
	if err := json.Unmarshal([]byte(raw), &config); err != nil {
		t.Fatal(err)
	}

	if err := service.PutProject(t.Context(), &config, api.PutProjectParams{ProjectID: project}); err != nil {
		t.Fatal(err)
	}

	configureSMTP(t, service, project, "127.0.0.1:1")
}

func configureSMTP(t *testing.T, service *Service, project, address string) {
	t.Helper()

	host, port, err := net.SplitHostPort(address)
	if err != nil {
		t.Fatal(err)
	}

	number, err := strconv.Atoi(port)
	if err != nil {
		t.Fatal(err)
	}

	settings := api.ProviderSettings{Channel: api.ChannelEmail, SMTP: api.NewOptSMTPSettings(api.SMTPSettings{
		Host: host, Port: number, TLS: api.SMTPSettingsTLSPlain, From: "sender@example.test",
	})}
	if err = service.PutProvider(t.Context(), &settings, api.PutProviderParams{ProjectID: project}); err != nil {
		t.Fatal(err)
	}
}

func request(t *testing.T, key string) *api.SendRequest {
	t.Helper()

	var result api.SendRequest

	raw := `{"notification_key":"registration","idempotency_key":"` + key + `","recipient_id":"recipient","deliveries":[{
 "channel":"email","default_enabled":true,"email":{"address":"recipient@example.test"},"data":{"name":"Alex"}}]}`
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatal(err)
	}

	return &result
}

func TestPostgresAdmissionAndDelivery(t *testing.T) {
	t.Parallel()
	service := setup(t)
	fixture(t, service, "project")
	checkConcurrentAdmission(t, service)
	checkAtomicAdmission(t, service)
	checkPreferencesAndReplay(t, service)
	checkHTTPAuth(t, service)
	checkLiveProviderAndCleanup(t, service)
}

func checkConcurrentAdmission(t *testing.T, service *Service) {
	t.Helper()
	ctx := context.WithValue(t.Context(), projectContext, "project")

	var workers sync.WaitGroup

	results := make(chan *api.Accepted, 8)
	failures := make(chan error, 8)

	for range 8 {
		workers.Go(func() {
			result,
				err := service.Send(ctx,
				request(t,
					"concurrent"))
			results <- result

			failures <- err
		})
	}

	workers.Wait()
	close(results)
	close(failures)

	for err := range failures {
		if err != nil {
			t.Fatal(err)
		}
	}

	var id uuid.UUID
	for result := range results {
		if id != uuid.Nil && id != result.MessageID {
			t.Fatal("duplicate admission")
		}

		id = result.MessageID
	}

	var count int
	if err := service.pool.QueryRow(t.Context(),
		"SELECT count(*) FROM outbox_messages").Scan(&count); err != nil || count != 1 {
		t.Fatalf("jobs: %d %v", count, err)
	}

	changed := request(t, "concurrent")

	changed.RecipientID = api.NewOptString("other")
	if _, err := service.Send(ctx, changed); service.NewError(ctx, err).StatusCode != http.StatusConflict {
		t.Fatalf("expected conflict, got %v", err)
	}
}

func checkAtomicAdmission(t *testing.T, service *Service) {
	t.Helper()
	ctx := context.WithValue(t.Context(), projectContext, "project")
	invalid := request(t, "atomic")

	invalid.Deliveries = append(invalid.Deliveries, api.DeliveryInput{
		Channel: api.ChannelFcm, DefaultEnabled: true,
		Data: api.Data{}, Fcm: api.NewOptFCMTarget(api.FCMTarget{Token: "device"}),
	})
	if _, err := service.Send(ctx, invalid); err == nil {
		t.Fatal("partially valid request accepted")
	}

	var count int
	if err := service.pool.QueryRow(ctx,
		"SELECT count(*) FROM admissions WHERE idempotency_key='atomic'").Scan(&count); err != nil || count != 0 {
		t.Fatalf("rejected admission retained: %d %v", count, err)
	}

	if err := service.pool.QueryRow(ctx,
		"SELECT count(*) FROM outbox_messages").Scan(&count); err != nil || count != 1 {
		t.Fatalf("partial queue insertion: %d %v", count, err)
	}

	blocked := request(t, "blocked")
	blocked.Deliveries[0].DefaultEnabled = false

	accepted, err := service.Send(ctx, blocked)
	if err != nil || len(accepted.QueuedChannels) != 0 {
		t.Fatalf("all blocked: %+v %v", accepted, err)
	}
}

func checkPreferencesAndReplay(t *testing.T, service *Service) {
	t.Helper()
	ctx := context.WithValue(t.Context(), projectContext, "project")

	preferences := &api.Preferences{Rules: []api.Preference{{
		NotificationKey: "registration",
		Channel:         api.ChannelEmail,
		Enabled:         false,
	}}}
	if err := service.PutPreferences(ctx, preferences, api.PutPreferencesParams{RecipientID: "recipient"}); err != nil {
		t.Fatal(err)
	}

	var payload []byte
	if err := service.pool.QueryRow(ctx, "SELECT payload FROM outbox_messages LIMIT 1").Scan(&payload); err != nil {
		t.Fatal(err)
	}
	// Exercise the publisher's admission-independent late preference check.
	err := service.Publish(ctx, []outbox.Message{{Payload: payload}})

	var discarded *outbox.DiscardError
	if !errors.As(err, &discarded) {
		t.Fatalf("late preference did not discard: %v", err)
	}

	if err = service.PutPreferences(ctx,
		&api.Preferences{Rules: []api.Preference{}},
		api.PutPreferencesParams{RecipientID: "recipient"}); err != nil {
		t.Fatal(err)
	}

	expired := request(t, "expired")

	expired.ExpiresAt = api.NewOptDateTime(time.Now().Add(-time.Second))
	if _, err = service.Send(ctx, expired); service.NewError(ctx, err).StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("expired accepted: %v", err)
	}
}

func checkHTTPAuth(t *testing.T, service *Service) {
	t.Helper()
	fixture(t, service, "other")

	key,
		err := service.IssueKey(t.Context(),
		&api.APIKeyRequest{Name: "test"},
		api.IssueKeyParams{ProjectID: "project"})
	if err != nil {
		t.Fatal(err)
	}

	handler, err := service.Handler()
	if err != nil {
		t.Fatal(err)
	}

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	client, err := sdk.New(server.URL, key.Secret)
	if err != nil {
		t.Fatal(err)
	}

	result, err := client.Send(t.Context(), request(t, "sdk"))
	if err != nil || !bool(result.Accepted) {
		t.Fatalf("generated client: %+v %v", result, err)
	}

	if err = service.RevokeKey(t.Context(), api.RevokeKeyParams{ProjectID: "project", KeyID: key.ID}); err != nil {
		t.Fatal(err)
	}

	if _, err = client.Send(t.Context(), request(t, "revoked")); err == nil {
		t.Fatal("revoked key accepted")
	}

	response := httptest.NewRecorder()
	loginBody := strings.NewReader(`{"key":"` + strings.Repeat("a", 32) + `"}`)
	login := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/admin/session", loginBody)
	login.Header.Set("Content-Type", "application/json")
	login.Header.Set("Origin", "https://evil.example")
	handler.ServeHTTP(response, login)

	if response.Code != http.StatusForbidden {
		t.Fatal("cross-origin login accepted")
	}
}

func checkLiveProviderAndCleanup(t *testing.T, service *Service) {
	t.Helper()
	address := docker(t, "axllent/mailpit@sha256:c96991d9bef73594c246d89ca81411d4e916f03e76a7d2d72fa2ab5dd3c9ce24", "1025")
	configureSMTP(t, service, "project", address)
	ctx, cancel := context.WithCancel(t.Context())

	done := make(chan error, 1)
	go func() { done <- service.Run(ctx) }()

	t.Cleanup(func() { cancel(); <-done })

	deadline := time.Now().Add(20 * time.Second)

	for {
		var count int
		if err := service.pool.QueryRow(t.Context(),
			`SELECT count(*) FROM outbox_messages
 WHERE status='published' AND octet_length(payload)=0`).Scan(&count); err != nil {
			t.Fatal(err)
		}

		if count == 2 {
			break
		}

		if time.Now().After(deadline) {
			t.Fatalf("pending messages did not use updated SMTP configuration: %d", count)
		}

		time.Sleep(100 * time.Millisecond)
	}

	var secrets int
	if err := service.pool.QueryRow(t.Context(),
		`SELECT count(*) FROM providers
 WHERE position('sender@example.test'::bytea IN ciphertext)>0`).Scan(&secrets); err != nil || secrets != 0 {
		t.Fatalf("plaintext provider: %d %v", secrets, err)
	}
}

func TestDeadLettersRetryExpiryAndReplay(t *testing.T) {
	t.Parallel()
	service := setup(t)
	fixture(t, service, "project")

	ctx := context.WithValue(t.Context(), projectContext, "project")
	if _, err := service.Send(ctx, request(t, "retry")); err != nil {
		t.Fatal(err)
	}

	expired := request(t, "expiry")

	expired.ExpiresAt = api.NewOptDateTime(time.Now().Add(100 * time.Millisecond))
	if _, err := service.Send(ctx, expired); err != nil {
		t.Fatal(err)
	}

	time.Sleep(150 * time.Millisecond)

	runCtx, cancel := context.WithCancel(ctx)

	done := make(chan error, 1)
	go func() { done <- service.Run(runCtx) }()

	t.Cleanup(func() { cancel(); <-done })

	var dead *api.DeadLetters

	deadline := time.Now().Add(15 * time.Second)

	for {
		result, err := service.ListDeadLetters(ctx, api.ListDeadLettersParams{ProjectID: "project"})
		if err != nil {
			t.Fatal(err)
		}

		if len(result.Items) == 2 {
			dead = result
			break
		}

		if time.Now().After(deadline) {
			t.Fatal("retry exhaustion did not reach DLQ")
		}

		time.Sleep(100 * time.Millisecond)
	}

	for _, item := range dead.Items {
		if item.Snapshot.ExpiresAt.Set {
			err := service.Replay(ctx, api.ReplayParams{ProjectID: "project", DeliveryID: item.ID})
			if service.NewError(ctx, err).StatusCode != http.StatusConflict {
				t.Fatalf("expired replay: %v", err)
			}

			continue
		}

		if item.Attempts != 3 {
			t.Fatalf("wrong attempts: %d", item.Attempts)
		}

		err := service.Replay(ctx, api.ReplayParams{ProjectID: "other", DeliveryID: item.ID})
		if service.NewError(ctx, err).StatusCode != http.StatusNotFound {
			t.Fatal("cross-project replay allowed")
		}

		if err = service.Replay(ctx, api.ReplayParams{ProjectID: "project", DeliveryID: item.ID}); err != nil {
			t.Fatal(err)
		}
	}
}
