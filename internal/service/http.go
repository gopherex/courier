package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/ogen-go/ogen/ogenerrors"
	"go.opentelemetry.io/otel/trace/noop"

	"github.com/gopherex/courier/pkg/api"
)

// Handler constructs the generated router with bounded input and sanitized failures.
func (service *Service) Handler() (http.Handler, error) {
	router,
		err := api.NewServer(service,
		service,
		api.WithErrorHandler(service.httpError),
		api.WithTracerProvider(noop.NewTracerProvider()))
	if err != nil {
		return nil, fmt.Errorf("HTTP router: %w", err)
	}

	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Cache-Control", "no-store")
		writer.Header().Set("X-Content-Type-Options", "nosniff")

		request.Body = http.MaxBytesReader(writer, request.Body, maxRequestBytes)
		if strings.HasPrefix(request.URL.Path, "/admin/") && request.Method != http.MethodGet {
			origin := request.Header.Get("Origin")
			if (origin != "" && origin != service.config.PublicURL) || request.Header.Get("Sec-Fetch-Site") == "cross-site" {
				service.httpError(request.Context(), writer, request, fail(http.StatusForbidden, "origin_rejected"))
				return
			}
		}

		ctx := context.WithValue(request.Context(), exchangeContext, &exchange{writer: writer, request: request})
		router.ServeHTTP(writer, request.WithContext(ctx))
	}), nil
}

func (service *Service) httpError(ctx context.Context, writer http.ResponseWriter, _ *http.Request, err error) {
	result := service.NewError(ctx, err)

	var security *ogenerrors.SecurityError
	if errors.As(err, &security) && result.StatusCode == http.StatusInternalServerError {
		result.StatusCode = http.StatusUnauthorized
		result.Response.Code = "unauthorized"
	}

	var decode *ogenerrors.DecodeRequestError
	if errors.As(err, &decode) {
		result.StatusCode = http.StatusBadRequest
		result.Response.Code = "invalid_request"
	}

	result.Response.Message = result.Response.Code

	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(result.StatusCode)
	// A disconnected client cannot receive a second error response.
	if writeErr := json.NewEncoder(writer).Encode(result.Response); writeErr != nil {
		slog.DebugContext(ctx, "HTTP response write failed")
	}
}

// Cleanup removes expired operational metadata; payload retention is owned by pg-outbox.
func (service *Service) Cleanup(ctx context.Context) error {
	if err := service.queries.CompleteAdmissions(ctx, time.Now().Add(service.config.DedupRetention)); err != nil {
		return fmt.Errorf("complete admissions: %w", err)
	}

	for _, cleanup := range []func(context.Context) error{
		service.queries.CleanupAdmissions,
		service.queries.CleanupSessions,
		service.queries.CleanupLoginLimits,
	} {
		if err := cleanup(ctx); err != nil {
			return fmt.Errorf("clean metadata: %w", err)
		}
	}

	return nil
}

const maxRequestBytes = 1024 * 1024
