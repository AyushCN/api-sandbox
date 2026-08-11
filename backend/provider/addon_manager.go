package provider

import (
	"context"
	"fmt"
	"github.com/api-sandbox/backend/models"
)

// AddonManager manages provisioning and deprovisioning of addons
type AddonManager struct {
	providers map[string]AddonProvider
}

// NewAddonManager initializes the AddonManager with supported providers
func NewAddonManager() *AddonManager {
	return &AddonManager{
		providers: map[string]AddonProvider{
			"postgres": NewPostgresProvider(),
			"mongodb":  NewMongoProvider(),
			"redis":    NewRedisProvider(),
		},
	}
}

// Provision delegates the addon creation to the specific provider
func (m *AddonManager) Provision(ctx context.Context, addon *models.Addon) (string, error) {
	provider, exists := m.providers[addon.Type]
	if !exists {
		return "", fmt.Errorf("unsupported addon type: %s", addon.Type)
	}
	return provider.Provision(ctx, addon)
}

// Deprovision delegates the addon removal to the specific provider
func (m *AddonManager) Deprovision(ctx context.Context, addon *models.Addon) error {
	provider, exists := m.providers[addon.Type]
	if !exists {
		return fmt.Errorf("unsupported addon type: %s", addon.Type)
	}
	return provider.Deprovision(ctx, addon)
}
