package adminsettings

import (
	"context"
	"errors"
	"strings"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) GetSettings(ctx context.Context) ([]SettingItem, error) {
	return s.repo.GetSettings(ctx)
}

func (s *Service) GetAuditLogs(ctx context.Context, limit int) ([]AuditLogItem, error) {
	if limit <= 0 {
		limit = 50
	}
	return s.repo.GetAuditLogs(ctx, limit)
}

// wellKnownDefaults defines default values for platform settings that should
// always return a value even if not yet stored in the DB.
var wellKnownDefaults = map[string]string{
	"default_commission_rate": "5.0",
	"maintenance_mode":        "false",
	"return_window_days":      "7",
	"min_order_amount":        "100",
	"max_order_amount":        "100000",
	"currency":                "BDT",
}

func (s *Service) GetSettingByKey(ctx context.Context, key string) (*SettingItem, error) {
	if strings.TrimSpace(key) == "" {
		return nil, errors.New("key is required")
	}
	item, err := s.repo.GetSettingByKey(ctx, key)
	if err != nil {
		// Fall back to well-known defaults before returning 404
		if defaultVal, ok := wellKnownDefaults[key]; ok {
			return &SettingItem{Key: key, Value: defaultVal, Description: "Platform default"}, nil
		}
		return nil, err
	}
	return item, nil
}

func (s *Service) UpdateSettingsBatch(ctx context.Context, settings map[string]string) ([]string, error) {
	if len(settings) == 0 {
		return nil, errors.New("at least one setting required")
	}
	return s.repo.UpdateSettingsBatch(ctx, settings)
}

func (s *Service) ToggleMaintenanceMode(ctx context.Context, enabled bool) error {
	return s.repo.ToggleMaintenanceMode(ctx, enabled)
}

func (s *Service) UpdateSettingByKey(ctx context.Context, key, value, desc string) error {
	if strings.TrimSpace(key) == "" || strings.TrimSpace(value) == "" {
		return errors.New("key and value are required")
	}
	return s.repo.UpdateSettingByKey(ctx, key, value, desc)
}
