// Package sdk constructs Courier's generated, project-scoped HTTP client.
package sdk

import (
	"context"
	"errors"
	"fmt"

	"github.com/gopherex/courier/pkg/api"
)

var errAdminSession = errors.New("service SDK cannot authenticate as an administrator")

type credentials struct{ token string }

// New creates a service client. Request and response types are generated in pkg/api.
func New(endpoint, token string, options ...api.ClientOption) (*api.Client, error) {
	client, err := api.NewClient(endpoint, credentials{token: token}, options...)
	if err != nil {
		return nil, fmt.Errorf("courier client: %w", err)
	}

	return client, nil
}

func (source credentials) ServiceKey(_ context.Context, _ api.OperationName) (api.ServiceKey, error) {
	return api.ServiceKey{Token: source.token}, nil
}

func (credentials) AdminSession(_ context.Context, _ api.OperationName) (api.AdminSession, error) {
	return api.AdminSession{}, errAdminSession
}
