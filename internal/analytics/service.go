package analytics

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

func (s *Service) GetOverview(ctx context.Context) (*AnalyticsOverview, error) {
	return s.repo.GetOverview(ctx)
}

func (s *Service) GetSalesAnalytics(ctx context.Context, days int) (*SalesAnalytics, error) {
	if days <= 0 {
		days = 30
	}
	if days > 90 {
		days = 90
	}
	return s.repo.GetSalesAnalytics(ctx, days)
}

func (s *Service) GetCustomerAnalytics(ctx context.Context) (*CustomerAnalytics, error) {
	return s.repo.GetCustomerAnalytics(ctx)
}

func (s *Service) GetProductAnalytics(ctx context.Context) (*ProductAnalytics, error) {
	return s.repo.GetProductAnalytics(ctx)
}

func (s *Service) GetTrafficAnalytics(ctx context.Context) (*TrafficAnalytics, error) {
	return s.repo.GetTrafficAnalytics(ctx)
}

func (s *Service) QueueCustomReport(ctx context.Context, reportType string, params map[string]interface{}, userID string) (string, error) {
	if strings.TrimSpace(reportType) == "" {
		return "", errors.New("report_type is required")
	}
	return s.repo.QueueCustomReport(ctx, reportType, params, userID)
}
