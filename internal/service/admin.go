package service

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gopherex/pgtx/pkg/tx"
	"github.com/jackc/pgx/v5"

	"github.com/gopherex/courier/internal/postgres/gen/db"
	"github.com/gopherex/courier/internal/render"
	"github.com/gopherex/courier/pkg/api"
)

// ListProjects lists configuration without credentials.
func (service *Service) ListProjects(ctx context.Context) (*api.Projects, error) {
	rows, err := service.queries.ListProjects(ctx)
	if err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}

	result := &api.Projects{Items: make([]api.Project, 0, len(rows))}
	for _, row := range rows {
		item := api.Project{ID: row.ID}
		if err = json.Unmarshal(row.Config, &item.Config); err != nil {
			return nil, fmt.Errorf("decode project: %w", err)
		}

		result.Items = append(result.Items, item)
	}

	return result, nil
}

// PutProject validates the complete project before applying it atomically.
func (service *Service) PutProject(ctx context.Context, request *api.ProjectConfig, params api.PutProjectParams) error {
	if err := render.ValidateProject(request); err != nil {
		return fail(http.StatusUnprocessableEntity, err.Error())
	}

	raw, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("encode project: %w", err)
	}

	if err = service.queries.PutProject(ctx, db.PutProjectParams{ID: params.ProjectID, Config: raw}); err != nil {
		return fmt.Errorf("save project: %w", err)
	}

	return nil
}

func (service *Service) provider(ctx context.Context,
	project string,
	channel api.Channel) (*api.ProviderSettings,
	error,
) {
	row, err := service.queries.GetProvider(ctx, db.GetProviderParams{ProjectID: project, Channel: string(channel)})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fail(http.StatusUnprocessableEntity, "provider_missing")
	}

	if err != nil {
		return nil, fmt.Errorf("read provider: %w", err)
	}

	raw, err := service.secrets.Open(nil, nil, row.Ciphertext, []byte(project+"/"+string(channel)))
	if err != nil {
		return nil, fail(http.StatusInternalServerError, "provider_decryption_failed")
	}

	result := &api.ProviderSettings{}
	if err = json.Unmarshal(raw, result); err != nil {
		return nil, fmt.Errorf("decode provider: %w", err)
	}

	return result, nil
}

// PutProvider encrypts credentials and serializes provider updates against admission.
func (service *Service) PutProvider(ctx context.Context,
	request *api.ProviderSettings,
	params api.PutProviderParams,
) error {
	if err := service.sender.Validate(request); err != nil {
		return fail(http.StatusUnprocessableEntity, "invalid_provider")
	}

	raw, encodeErr := json.Marshal(request)
	if encodeErr != nil {
		return fmt.Errorf("encode provider: %w", encodeErr)
	}

	encrypted := service.secrets.Seal(nil, nil, raw, []byte(params.ProjectID+"/"+string(request.Channel)))

	transactionErr := tx.DoReadCommitted(ctx, service.transactions, func(ctx context.Context) error {
		if _, err := service.queries.LockProjectWrite(ctx, params.ProjectID); err != nil {
			return fmt.Errorf("lock project: %w", err)
		}

		if err := service.queries.PutProvider(ctx,
			db.PutProviderParams{
				ProjectID:  params.ProjectID,
				Channel:    string(request.Channel),
				Ciphertext: encrypted,
			}); err != nil {
			return fmt.Errorf("save provider: %w", err)
		}

		return nil
	})
	if transactionErr != nil {
		return fmt.Errorf("update provider: %w", transactionErr)
	}

	return nil
}

// ListProviders reports only configuration presence, never stored secrets.
func (service *Service) ListProviders(ctx context.Context, params api.ListProvidersParams) (*api.Providers, error) {
	rows, err := service.queries.ListProviders(ctx, params.ProjectID)
	if err != nil {
		return nil, fmt.Errorf("list providers: %w", err)
	}

	result := &api.Providers{Items: make([]api.ProviderStatus, 0, len(rows))}
	for _, row := range rows {
		result.Items = append(result.Items, api.ProviderStatus{Channel: api.Channel(row.Channel), Configured: true})
	}

	return result, nil
}

// IssueKey returns a random service credential once and stores only its digest.
func (service *Service) IssueKey(ctx context.Context,
	request *api.APIKeyRequest,
	params api.IssueKeyParams) (*api.IssuedKey,
	error,
) {
	result := &api.IssuedKey{ID: uuid.New(), Secret: rand.Text()}
	if err := service.queries.IssueKey(ctx,
		db.IssueKeyParams{
			ID:        result.ID,
			ProjectID: params.ProjectID,
			Name:      request.Name,
			Digest:    digest(result.Secret),
		}); err != nil {
		return nil, fmt.Errorf("issue API key: %w", err)
	}

	return result, nil
}

// ListKeys returns service-key metadata without secret material.
func (service *Service) ListKeys(ctx context.Context, params api.ListKeysParams) (*api.APIKeys, error) {
	rows, err := service.queries.ListKeys(ctx, params.ProjectID)
	if err != nil {
		return nil, fmt.Errorf("list keys: %w", err)
	}

	result := &api.APIKeys{Items: make([]api.APIKey, 0, len(rows))}
	for _, row := range rows {
		result.Items = append(result.Items, api.APIKey{ID: row.ID, Name: row.Name, CreatedAt: row.CreatedAt})
	}

	return result, nil
}

// RevokeKey immediately revokes a service key in the specified project.
func (service *Service) RevokeKey(ctx context.Context, params api.RevokeKeyParams) error {
	if err := service.queries.RevokeKey(ctx,
		db.RevokeKeyParams{
			ID:        params.KeyID,
			ProjectID: params.ProjectID,
		}); err != nil {
		return fmt.Errorf("revoke key: %w", err)
	}

	return nil
}

// Preview uses the admission renderer without enqueuing or sending anything.
func (service *Service) Preview(ctx context.Context,
	request *api.PreviewRequest,
	params api.PreviewParams) (*api.Rendered,
	error,
) {
	row, err := service.queries.GetProject(ctx, params.ProjectID)
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

	channel, err := render.Channel(notification, request.Channel)
	if err != nil {
		return nil, fail(http.StatusUnprocessableEntity, "channel_unavailable")
	}

	result, err := render.Render(channel, request.Locale.Value, config.DefaultLocale, request.Data)
	if err != nil {
		return nil, fail(http.StatusUnprocessableEntity, err.Error())
	}

	return result, nil
}

// ListDeadLetters exposes retained dead-letter payloads only to administrators.
func (service *Service) ListDeadLetters(ctx context.Context,
	params api.ListDeadLettersParams) (*api.DeadLetters,
	error,
) {
	rows, err := service.queries.ListDead(ctx, db.ListDeadParams{Topic: params.ProjectID, After: params.After.Value})
	if err != nil {
		return nil, fmt.Errorf("list dead letters: %w", err)
	}

	result := &api.DeadLetters{Items: make([]api.DeadLetter, 0, len(rows))}
	for _, row := range rows {
		item := api.DeadLetter{ID: row.ID, Attempts: int(row.Attempts)}
		if row.LastError != nil {
			item.ErrorCode = *row.LastError
		}

		if row.FinishedAt != nil {
			item.FinishedAt = *row.FinishedAt
		}

		if err = json.Unmarshal(row.Payload, &item.Snapshot); err != nil {
			return nil, fmt.Errorf("decode dead letter: %w", err)
		}

		result.Items = append(result.Items, item)
	}

	return result, nil
}

// Replay returns an unexpired dead letter to pending under the same locked transaction.
func (service *Service) Replay(ctx context.Context, params api.ReplayParams) error {
	err := tx.DoReadCommitted(ctx, service.transactions, func(ctx context.Context) error {
		row, err := service.queries.LockDead(ctx, db.LockDeadParams{ID: params.DeliveryID, Topic: params.ProjectID})
		if errors.Is(err, pgx.ErrNoRows) {
			return fail(http.StatusNotFound, "delivery_not_found")
		}

		if err != nil {
			return fmt.Errorf("lock dead letter: %w", err)
		}

		var snapshot api.Snapshot
		if err = json.Unmarshal(row.Payload, &snapshot); err != nil {
			return fmt.Errorf("decode dead letter: %w", err)
		}

		if expiry, ok := snapshot.ExpiresAt.Get(); ok && !expiry.After(time.Now()) {
			return fail(http.StatusConflict, "expired")
		}

		requeued, err := service.queue.Requeue(ctx, row.ID.String())
		if err != nil {
			return fmt.Errorf("requeue delivery: %w", err)
		}

		if !requeued {
			return fail(http.StatusConflict, "delivery_not_dead")
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("replay delivery: %w", err)
	}

	return nil
}
