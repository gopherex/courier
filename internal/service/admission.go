package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	outbox "github.com/gopherex/pg-outbox"
	"github.com/gopherex/pgtx/pkg/tx"
	"github.com/jackc/pgx/v5"

	"github.com/gopherex/courier/internal/delivery"
	"github.com/gopherex/courier/internal/postgres/gen/db"
	"github.com/gopherex/courier/internal/render"
	"github.com/gopherex/courier/pkg/api"
)

// Send durably admits a service request under the authenticated project.
func (service *Service) Send(ctx context.Context, request *api.SendRequest) (*api.Accepted, error) {
	return service.admit(ctx, projectID(ctx), request)
}

// TestSend exercises the same admission and delivery path using administrator credentials.
func (service *Service) TestSend(ctx context.Context,
	request *api.SendRequest,
	params api.TestSendParams) (*api.Accepted,
	error,
) {
	return service.admit(ctx, params.ProjectID, request)
}

func fingerprint(request *api.SendRequest) ([]byte, error) {
	raw, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("encode request: %w", err)
	}

	var value any

	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()

	if err = decoder.Decode(&value); err != nil {
		return nil, fmt.Errorf("decode request: %w", err)
	}

	canonical, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("canonical request: %w", err)
	}

	sum := sha256.Sum256(canonical)

	return sum[:], nil
}

func (service *Service) admit(ctx context.Context, project string, request *api.SendRequest) (*api.Accepted, error) {
	fingerprint, fingerprintErr := fingerprint(request)
	if fingerprintErr != nil {
		return nil, fingerprintErr
	}

	result,
		transactionErr := tx.DoReadCommittedRet(ctx,
		service.transactions,
		func(ctx context.Context) (*api.Accepted,
			error,
		) {
			if err := service.queries.LockAdmission(ctx, project+"/"+request.IdempotencyKey); err != nil {
				return nil, fmt.Errorf("lock admission: %w", err)
			}

			existing,
				err := service.queries.GetAdmission(ctx,
				db.GetAdmissionParams{
					ProjectID:      project,
					IdempotencyKey: request.IdempotencyKey,
				})
			if err == nil {
				if !bytes.Equal(existing.Fingerprint, fingerprint) {
					return nil, fail(http.StatusConflict, "idempotency_conflict")
				}

				var result api.Accepted
				if err = json.Unmarshal(existing.Result, &result); err != nil {
					return nil, fmt.Errorf("decode admission: %w", err)
				}

				return &result, nil
			}

			if !errors.Is(err, pgx.ErrNoRows) {
				return nil, fmt.Errorf("read admission: %w", err)
			}

			return service.prepare(ctx, project, request, fingerprint)
		})
	if transactionErr != nil {
		service.admissionOutcome(ctx, "rejected")
		return nil, fmt.Errorf("admit message: %w", transactionErr)
	}

	outcome := "accepted"
	if len(result.QueuedChannels) == 0 {
		outcome = "skipped"
	}

	service.admissionOutcome(ctx, outcome)

	return result, nil
}

func (service *Service) prepare(ctx context.Context,
	project string,
	request *api.SendRequest,
	hash []byte) (*api.Accepted,
	error,
) {
	if expiry, ok := request.ExpiresAt.Get(); ok && !expiry.After(time.Now()) {
		return nil, fail(http.StatusUnprocessableEntity, "expired")
	}

	row, err := service.queries.LockProject(ctx, project)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fail(http.StatusNotFound, "project_not_found")
	}

	if err != nil {
		return nil, fmt.Errorf("read project: %w", err)
	}

	var config api.ProjectConfig
	if err = json.Unmarshal(row.Config, &config); err != nil {
		return nil, fmt.Errorf("decode project: %w", err)
	}

	notification, err := render.Notification(&config, request.NotificationKey)
	if err != nil {
		return nil, fail(http.StatusUnprocessableEntity, "notification_unavailable")
	}

	preferences, err := service.preferences(ctx, project, request.RecipientID.Value)
	if err != nil {
		return nil, err
	}

	result := &api.Accepted{MessageID: uuid.New(), Accepted: true, QueuedChannels: []api.Channel{}}

	seen := make(map[api.Channel]bool)

	for index := range request.Deliveries {
		input := &request.Deliveries[index]
		if seen[input.Channel] {
			return nil, fail(http.StatusUnprocessableEntity, "duplicate_channel")
		}

		seen[input.Channel] = true
		if !enabled(preferences, request.NotificationKey, input.Channel, input.DefaultEnabled) {
			continue
		}

		if enqueueErr := service.enqueue(ctx,
			project,
			request,
			input,
			&config,
			notification,
			result.MessageID); enqueueErr != nil {
			return nil, enqueueErr
		}

		result.QueuedChannels = append(result.QueuedChannels, input.Channel)
	}

	encoded, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("encode acceptance: %w", err)
	}

	err = service.queries.PutAdmission(ctx, db.PutAdmissionParams{
		ProjectID: project, IdempotencyKey: request.IdempotencyKey,
		Fingerprint: hash, Result: encoded, RetainUntil: time.Now().Add(service.config.DedupRetention),
	})
	if err != nil {
		return nil, fmt.Errorf("save acceptance: %w", err)
	}

	return result, nil
}

func (service *Service) enqueue(ctx context.Context, project string, request *api.SendRequest, input *api.DeliveryInput,
	config *api.ProjectConfig, notification *api.Notification, messageID uuid.UUID,
) error {
	if err := delivery.ValidateTarget(input); err != nil {
		return fail(http.StatusUnprocessableEntity, "invalid_target")
	}

	channel, err := render.Channel(notification, input.Channel)
	if err != nil {
		return fail(http.StatusUnprocessableEntity, "channel_unavailable")
	}

	if _, err = service.provider(ctx, project, input.Channel); err != nil {
		return err
	}

	rendered, err := render.Render(channel, request.Locale.Value, config.DefaultLocale, input.Data)
	if err != nil {
		return fail(http.StatusUnprocessableEntity, err.Error())
	}

	snapshot := api.Snapshot{
		ProjectID: project, MessageID: messageID, NotificationKey: request.NotificationKey,
		RecipientID: request.RecipientID.Value, Channel: input.Channel, DefaultEnabled: input.DefaultEnabled,
		Email:     input.Email,
		Webpush:   input.Webpush,
		Fcm:       input.Fcm,
		Rendered:  *rendered,
		ExpiresAt: request.ExpiresAt,
		CreatedAt: time.Now().UTC(),
	}

	payload, err := json.Marshal(&snapshot)
	if err != nil {
		return fmt.Errorf("encode delivery: %w", err)
	}

	if err = service.queue.Enqueue(ctx, outbox.Message{
		ID: uuid.NewString(), Topic: project, PartitionKey: messageID.String(),
		Payload: payload, ContentType: "application/json", MessageType: string(input.Channel),
	}); err != nil {
		return fmt.Errorf("enqueue delivery: %w", err)
	}

	return nil
}

// Publish rechecks expiration and preferences and reads current provider credentials on every attempt.
func (service *Service) Publish(ctx context.Context, messages []outbox.Message) error {
	if len(messages) != 1 {
		return fmt.Errorf("delivery: %w", outbox.Permanent(fail(http.StatusInternalServerError, "invalid_batch")))
	}

	var snapshot api.Snapshot
	if err := json.Unmarshal(messages[0].Payload, &snapshot); err != nil {
		return fmt.Errorf("delivery: %w", outbox.Permanent(fail(http.StatusInternalServerError, "invalid_snapshot")))
	}

	ctx, span := service.tracer.Start(ctx, "courier.deliver")
	defer span.End()

	if expiry, ok := snapshot.ExpiresAt.Get(); ok {
		if !expiry.After(time.Now()) {
			service.observe(ctx, snapshot.Channel, "expired")
			return fmt.Errorf("delivery: %w", outbox.Permanent(fail(http.StatusUnprocessableEntity, "expired")))
		}

		var cancel context.CancelFunc

		ctx, cancel = context.WithDeadline(ctx, expiry)
		defer cancel()
	}

	preferences, err := service.preferences(ctx, snapshot.ProjectID, snapshot.RecipientID)
	if err != nil {
		return fail(http.StatusServiceUnavailable, "preferences_unavailable")
	}

	if !enabled(preferences, snapshot.NotificationKey, snapshot.Channel, snapshot.DefaultEnabled) {
		service.observe(ctx, snapshot.Channel, "skipped")
		return fmt.Errorf("delivery: %w", outbox.Discard(fail(http.StatusOK, "preference_blocked")))
	}

	provider, err := service.provider(ctx, snapshot.ProjectID, snapshot.Channel)
	if err != nil {
		return fail(http.StatusServiceUnavailable, "provider_unavailable")
	}

	err = service.sender.Send(ctx, provider, &snapshot)

	result := "success"
	if err != nil {
		result = "failure"
	}

	service.observe(ctx, snapshot.Channel, result)

	if err != nil {
		return fmt.Errorf("send: %w", err)
	}

	return nil
}
