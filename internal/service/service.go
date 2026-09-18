// Package service implements Courier's transactional admission and administration.
package service

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	outbox "github.com/gopherex/pg-outbox"
	"github.com/gopherex/pgtx"
	"github.com/gopherex/pgtx/pkg/tx"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"

	"github.com/gopherex/courier/internal/delivery"
	"github.com/gopherex/courier/internal/postgres/gen/db"
	"github.com/gopherex/courier/pkg/api"
)

// Config contains process-level policy, never queued message content.
type Config struct {
	AdminKey       string
	EncryptionKey  []byte
	PublicURL      string
	Development    bool
	Workers        int
	MaxAttempts    int
	AttemptTimeout time.Duration
	LeaseDuration  time.Duration
	DedupRetention time.Duration
	DeadRetention  time.Duration
}

// Service implements the generated HTTP handler and the outbox publisher.
type Service struct {
	pool         *pgxpool.Pool
	queries      *db.Queries
	transactions tx.Trm
	queue        *outbox.Outbox
	sender       *delivery.Sender
	secrets      cipher.AEAD
	config       Config
	counter      metric.Int64Counter
	completed    metric.Int64Counter
	admissions   metric.Int64Counter
	tracer       trace.Tracer
}

// New builds a service with a transaction-aware enqueue executor.
func New(pool *pgxpool.Pool, config *Config) (*Service, error) {
	manager, err := pgtx.NewTxManager(pool, tx.ReadCommitted())
	if err != nil {
		return nil, fmt.Errorf("transaction manager: %w", err)
	}

	block, err := aes.NewCipher(config.EncryptionKey)
	if err != nil {
		return nil, fmt.Errorf("encryption key: %w", err)
	}

	secrets, err := cipher.NewGCMWithRandomNonce(block)
	if err != nil {
		return nil, fmt.Errorf("encryption: %w", err)
	}

	executor := pgtx.NewTxDB(pool)

	counter, err := otel.Meter("courier").Int64Counter("courier.delivery.attempts")
	if err != nil {
		return nil, fmt.Errorf("attempt metric: %w", err)
	}

	service := &Service{
		pool: pool, queries: db.New(executor), transactions: manager, secrets: secrets, config: *config,
		sender: &delivery.Sender{AllowPlainSMTP: config.Development}, counter: counter, tracer: otel.Tracer("courier"),
	}

	queue, err := outbox.New(pool, executor, service,
		outbox.WithBatchSize(1), outbox.WithConcurrency(config.Workers), outbox.WithMaxAttempts(config.MaxAttempts),
		outbox.WithLeaseDuration(config.LeaseDuration), outbox.WithPublishTimeout(config.AttemptTimeout),
		outbox.WithRetryBackoff(outbox.ExpBackoff(time.Second, time.Hour, true)),
		outbox.WithClearPayloadOnComplete(true), outbox.WithRetention(time.Hour),
		outbox.WithDiscardRetention(time.Hour), outbox.WithDeadRetention(config.DeadRetention),
		outbox.WithLogger(slog.Default()), outbox.WithHooks(service))
	if err != nil {
		return nil, fmt.Errorf("outbox: %w", err)
	}

	service.queue = queue

	service.completed, err = otel.Meter("courier").Int64Counter("courier.delivery.outcomes")
	if err != nil {
		return nil, fmt.Errorf("outcome metric: %w", err)
	}

	service.admissions, err = otel.Meter("courier").Int64Counter("courier.admissions")
	if err != nil {
		return nil, fmt.Errorf("admission metric: %w", err)
	}

	if metricErr := service.registerMetrics(); metricErr != nil {
		return nil, metricErr
	}

	return service, nil
}

// Run executes the durable delivery relay until cancellation.
func (service *Service) Run(ctx context.Context) error {
	if err := service.queue.Run(ctx); err != nil {
		return fmt.Errorf("delivery relay: %w", err)
	}

	return nil
}

func digest(value string) []byte { sum := sha256.Sum256([]byte(value)); return sum[:] }

type fault struct {
	status int
	code   string
}

func (failure *fault) Error() string     { return failure.code }
func fail(status int, code string) error { return &fault{status: status, code: code} }

// NewError returns only stable, non-sensitive error codes to clients.
func (service *Service) NewError(_ context.Context, err error) *api.ProblemStatusCode {
	response := &api.ProblemStatusCode{
		StatusCode: http.StatusInternalServerError,
		Response: api.Problem{
			Code:    "internal_error",
			Message: "Internal service error",
		},
	}

	var failure *fault
	if errors.As(err, &failure) {
		response.StatusCode = failure.status
		response.Response.Code = failure.code
		response.Response.Message = failure.code
	}

	return response
}

func (service *Service) observe(ctx context.Context, channel api.Channel, result string) {
	service.counter.Add(ctx,
		1,
		metric.WithAttributes(attribute.String("channel",
			string(channel)),
			attribute.String("result",
				result)))
}
