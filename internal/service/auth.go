package service

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/gopherex/courier/internal/postgres/gen/db"
	"github.com/gopherex/courier/pkg/api"
)

type contextKey string

const (
	projectContext  contextKey = "project"
	exchangeContext contextKey = "exchange"
)

type exchange struct {
	writer  http.ResponseWriter
	request *http.Request
}

func projectID(ctx context.Context) string {
	project, _ := ctx.Value(projectContext).(string)
	return project
}

// HandleServiceKey authenticates a rotatable project-scoped service key.
func (service *Service) HandleServiceKey(ctx context.Context,
	_ api.OperationName,
	token api.ServiceKey) (context.Context,
	error,
) {
	result, err := service.queries.Authenticate(ctx, digest(token.Token))
	if errors.Is(err, pgx.ErrNoRows) {
		return ctx, fail(http.StatusUnauthorized, "unauthorized")
	}

	if err != nil {
		return ctx, fmt.Errorf("authenticate: %w", err)
	}

	return context.WithValue(ctx, projectContext, result.ProjectID), nil
}

// HandleAdminSession checks the shared PostgreSQL session store.
func (service *Service) HandleAdminSession(ctx context.Context,
	_ api.OperationName,
	token api.AdminSession) (context.Context,
	error,
) {
	if _, err := service.queries.CheckSession(ctx, digest(token.APIKey)); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ctx, fail(http.StatusUnauthorized, "unauthorized")
		}

		return ctx, fmt.Errorf("session lookup: %w", err)
	}

	return ctx, nil
}

const sessionLifetime = 8 * time.Hour

// Login rate-limits attempts across replicas and exchanges the master key for a revocable session.
func (service *Service) Login(ctx context.Context, request *api.LoginRequest) error {
	attempts, err := service.queries.LoginAttempt(ctx, "admin")
	if err != nil {
		return fmt.Errorf("login limiter: %w", err)
	}

	if attempts.Attempts > maxLoginAttempts {
		return fail(http.StatusTooManyRequests, "rate_limited")
	}

	if subtle.ConstantTimeCompare(digest(request.Key), digest(service.config.AdminKey)) != 1 {
		return fail(http.StatusUnauthorized, "unauthorized")
	}

	transport, ok := ctx.Value(exchangeContext).(*exchange)
	if !ok {
		return fail(http.StatusInternalServerError, "missing_transport")
	}

	token := rand.Text()
	if err = service.queries.AddSession(ctx,
		db.AddSessionParams{
			Digest:    digest(token),
			ExpiresAt: time.Now().Add(sessionLifetime),
		}); err != nil {
		return fmt.Errorf("save session: %w", err)
	}

	http.SetCookie(transport.writer, service.cookie(token, int(sessionLifetime.Seconds())))

	return nil
}

// Logout revokes the current browser session.
func (service *Service) Logout(ctx context.Context) error {
	transport, ok := ctx.Value(exchangeContext).(*exchange)
	if !ok {
		return fail(http.StatusInternalServerError, "missing_transport")
	}

	cookie, err := transport.request.Cookie("courier_session")
	if err != nil {
		return fail(http.StatusUnauthorized, "unauthorized")
	}

	if err = service.queries.DeleteSession(ctx, digest(cookie.Value)); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}

	http.SetCookie(transport.writer, service.cookie("", -1))

	return nil
}

func (service *Service) cookie(value string, maxAge int) *http.Cookie {
	cookie := &http.Cookie{
		Name: "courier_session", Value: value, Path: "/admin", MaxAge: maxAge, HttpOnly: true,
		Secure: true, SameSite: http.SameSiteStrictMode,
	}
	cookie.Secure = !service.config.Development

	return cookie
}

const maxLoginAttempts = 10
