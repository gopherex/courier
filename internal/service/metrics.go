package service

import (
	"context"
	"errors"
	"fmt"

	outbox "github.com/gopherex/pg-outbox"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

func (service *Service) registerMetrics() error {
	meter := otel.Meter("courier")

	jobs, err := meter.Int64ObservableGauge("courier.queue.jobs")
	if err != nil {
		return fmt.Errorf("queue metric: %w", err)
	}

	age, err := meter.Float64ObservableGauge("courier.queue.oldest_age", metric.WithUnit("s"))
	if err != nil {
		return fmt.Errorf("queue age metric: %w", err)
	}

	_, err = meter.RegisterCallback(func(ctx context.Context, observer metric.Observer) error {
		rows, queryErr := service.queries.QueueStats(ctx)
		if queryErr != nil {
			return errQueueStats
		}

		for _, status := range []string{"pending", "processing", "dead"} {
			var (
				total  int64
				oldest float64
			)

			for _, row := range rows {
				if row.Status == status {
					total = row.Total
					oldest = row.OldestSeconds
				}
			}

			labels := metric.WithAttributes(attribute.String("status", status))
			observer.ObserveInt64(jobs, total, labels)
			observer.ObserveFloat64(age, oldest, labels)
		}

		return nil
	}, jobs, age)
	if err != nil {
		return fmt.Errorf("register queue metrics: %w", err)
	}

	return nil
}

// OnPublished records committed successful deliveries without payloads or identities.
func (service *Service) OnPublished(ctx context.Context, messages []outbox.Message) {
	for index := range messages {
		message := &messages[index]
		service.outcome(ctx, message.MessageType, "published")
	}
}

// OnFailed records provider attempt failures using bounded labels.
//
//nolint:gocritic // pg-outbox Hooks requires a value parameter.
func (service *Service) OnFailed(ctx context.Context, message outbox.Message, _ error) {
	service.outcome(ctx, message.MessageType, "failed")
}

// OnDead records deliveries committed to the dead-letter queue.
//
//nolint:gocritic // pg-outbox Hooks requires a value parameter.
func (service *Service) OnDead(ctx context.Context, message outbox.Message) {
	service.outcome(ctx, message.MessageType, "dead")
}

// OnDiscarded records committed preference skips.
func (service *Service) OnDiscarded(ctx context.Context, messages []outbox.Message, _ error) {
	for index := range messages {
		message := &messages[index]
		service.outcome(ctx, message.MessageType, "skipped")
	}
}

// OnCleanup does not duplicate the queue's own cleanup reporting.
func (*Service) OnCleanup(_ context.Context, _ int) {}

func (service *Service) outcome(ctx context.Context, channel, result string) {
	service.completed.Add(ctx,
		1,
		metric.WithAttributes(attribute.String("channel",
			channel),
			attribute.String("result",
				result)))
}

var errQueueStats = errors.New("queue stats unavailable")

func (service *Service) admissionOutcome(ctx context.Context, result string) {
	service.admissions.Add(ctx, 1, metric.WithAttributes(attribute.String("result", result)))
}
