package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/jackc/pgx/v5"

	"github.com/gopherex/courier/internal/postgres/gen/db"
	"github.com/gopherex/courier/pkg/api"
)

func enabled(preferences *api.Preferences, key string, channel api.Channel, fallback bool) bool {
	for _, rule := range preferences.Rules {
		if rule.NotificationKey == key && rule.Channel == channel {
			return rule.Enabled
		}
	}

	return fallback
}

func (service *Service) preferences(ctx context.Context, project, recipient string) (*api.Preferences, error) {
	result := &api.Preferences{Rules: []api.Preference{}}
	if recipient == "" {
		return result, nil
	}

	row, err := service.queries.GetPreferences(ctx, db.GetPreferencesParams{ProjectID: project, RecipientID: recipient})
	if errors.Is(err, pgx.ErrNoRows) {
		return result, nil
	}

	if err != nil {
		return nil, fmt.Errorf("read preferences: %w", err)
	}

	if err = json.Unmarshal(row.Rules, result); err != nil {
		return nil, fmt.Errorf("decode preferences: %w", err)
	}

	return result, nil
}

// GetPreferences returns only rules explicitly set for the authenticated project's recipient.
func (service *Service) GetPreferences(ctx context.Context, params api.GetPreferencesParams) (*api.Preferences, error) {
	return service.preferences(ctx, projectID(ctx), params.RecipientID)
}

// PutPreferences atomically replaces a recipient's complete preference document.
func (service *Service) PutPreferences(ctx context.Context,
	request *api.Preferences,
	params api.PutPreferencesParams,
) error {
	seen := make(map[string]bool)

	for _, rule := range request.Rules {
		key := rule.NotificationKey + "/" + string(rule.Channel)
		if seen[key] {
			return fail(http.StatusUnprocessableEntity, "duplicate_preference")
		}

		seen[key] = true
	}

	raw, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("encode preferences: %w", err)
	}

	if err = service.queries.PutPreferences(ctx,
		db.PutPreferencesParams{
			ProjectID:   projectID(ctx),
			RecipientID: params.RecipientID,
			Rules:       raw,
		}); err != nil {
		return fmt.Errorf("save preferences: %w", err)
	}

	return nil
}
