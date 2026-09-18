//go:build integration

package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gopherex/courier/pkg/api"
)

func adminRequest(t *testing.T, handler http.Handler, method, path, body string,
	cookie *http.Cookie,
) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequestWithContext(t.Context(), method, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")

	if cookie != nil {
		request.AddCookie(cookie)
	}

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	return response
}

func TestAdminSessionLifecycle(t *testing.T) {
	t.Parallel()
	service := setup(t)
	service.config.Development = false

	handler, err := service.Handler()
	if err != nil {
		t.Fatal(err)
	}

	login := adminRequest(t, handler, http.MethodPost, "/admin/session", `{"key":"`+strings.Repeat("a", 32)+`"}`, nil)
	if login.Code != http.StatusNoContent {
		t.Fatalf("login status: %d", login.Code)
	}

	response := login.Result()
	if err = response.Body.Close(); err != nil {
		t.Fatal(err)
	}

	cookies := response.Cookies()
	if len(cookies) != 1 {
		t.Fatal("session cookie missing")
	}

	cookie := cookies[0]
	if !cookie.Secure || !cookie.HttpOnly || cookie.SameSite != http.SameSiteStrictMode || cookie.Path != "/admin" {
		t.Fatal("unsafe session cookie attributes")
	}

	if result := adminRequest(t, handler, http.MethodGet, "/admin/projects", "", cookie); result.Code != http.StatusOK {
		t.Fatalf("session rejected: %d", result.Code)
	}

	logout := adminRequest(t, handler, http.MethodDelete, "/admin/session", "", cookie)
	if logout.Code != http.StatusNoContent {
		t.Fatalf("logout failed: %d", logout.Code)
	}

	revoked := adminRequest(t, handler, http.MethodGet, "/admin/projects", "", cookie)
	if revoked.Code != http.StatusUnauthorized {
		t.Fatalf("revoked session accepted: %d", revoked.Code)
	}
}

func TestLoginLimitSharedAcrossInstances(t *testing.T) {
	t.Parallel()
	service := setup(t)

	other, err := New(service.pool, &service.config)
	if err != nil {
		t.Fatal(err)
	}

	first, err := service.Handler()
	if err != nil {
		t.Fatal(err)
	}

	second, err := other.Handler()
	if err != nil {
		t.Fatal(err)
	}

	for attempt := range maxLoginAttempts {
		handler := first
		if attempt%2 == 0 {
			handler = second
		}

		result := adminRequest(t, handler, http.MethodPost, "/admin/session", `{"key":"`+strings.Repeat("b", 32)+`"}`, nil)
		if result.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d: %d", attempt, result.Code)
		}
	}

	result := adminRequest(t, second, http.MethodPost, "/admin/session", `{"key":"`+strings.Repeat("a", 32)+`"}`, nil)
	if result.Code != http.StatusTooManyRequests {
		t.Fatalf("shared limit bypassed: %d", result.Code)
	}
}

func TestDeduplicationSurvivesConfigChangesAndPendingJobs(t *testing.T) {
	t.Parallel()
	service := setup(t)
	fixture(t, service, "project")
	ctx := context.WithValue(t.Context(), projectContext, "project")

	accepted, err := service.Send(ctx, request(t, "retention"))
	if err != nil {
		t.Fatal(err)
	}

	config := &api.ProjectConfig{
		Title:         api.Translations{"en": "Empty"},
		DefaultLocale: "en", Notifications: []api.Notification{},
	}
	if err = service.PutProject(ctx, config, api.PutProjectParams{ProjectID: "project"}); err != nil {
		t.Fatal(err)
	}

	repeated, err := service.Send(ctx, request(t, "retention"))
	if err != nil || repeated.MessageID != accepted.MessageID {
		t.Fatalf("lost original admission: %v", err)
	}

	if _, err = service.pool.Exec(ctx, "UPDATE admissions SET retain_until=now()-interval '1 hour'"); err != nil {
		t.Fatal(err)
	}

	if err = service.Cleanup(ctx); err != nil {
		t.Fatal(err)
	}

	assertAdmissionCount(t, service, "completed_at IS NULL", 1)

	if _, err = service.pool.Exec(ctx,
		"UPDATE outbox_messages SET status='published',payload=''::bytea,finished_at=now()"); err != nil {
		t.Fatal(err)
	}

	if err = service.Cleanup(ctx); err != nil {
		t.Fatal(err)
	}

	assertAdmissionCount(t, service, "completed_at IS NOT NULL AND retain_until>now()+interval '59 minutes'", 1)

	if _, err = service.pool.Exec(ctx, "UPDATE admissions SET retain_until=now()-interval '1 second'"); err != nil {
		t.Fatal(err)
	}

	if err = service.Cleanup(ctx); err != nil {
		t.Fatal(err)
	}

	assertAdmissionCount(t, service, "true", 0)
}

func assertAdmissionCount(t *testing.T, service *Service, condition string, expected int) {
	t.Helper()

	var count int
	if err := service.pool.QueryRow(t.Context(),
		"SELECT count(*) FROM admissions WHERE "+condition).Scan(&count); err != nil {
		t.Fatal(err)
	}

	if count != expected {
		t.Fatalf("admissions matching %s: %d, expected %d", condition, count, expected)
	}
}
