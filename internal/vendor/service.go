package vendor

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

func (s *Service) GetStorePublic(ctx context.Context, slug string) (*PublicStore, error) {
	if strings.TrimSpace(slug) == "" {
		return nil, errors.New("store slug is required")
	}
	return s.repo.GetStoreBySlug(ctx, slug)
}

func (s *Service) GetMyStore(ctx context.Context, userID string) (*VendorProfile, error) {
	if strings.TrimSpace(userID) == "" {
		return nil, errors.New("user ID is required")
	}
	return s.repo.GetVendorByUserID(ctx, userID)
}

func (s *Service) RegisterStore(ctx context.Context, p RegisterStoreParams) (string, error) {
	if strings.TrimSpace(p.UserID) == "" {
		return "", errors.New("user ID is required")
	}
	if strings.TrimSpace(p.StoreName) == "" || strings.TrimSpace(p.StoreSlug) == "" {
		return "", errors.New("store_name and store_slug are required")
	}
	return s.repo.RegisterStore(ctx, p)
}

func (s *Service) UpdateStore(ctx context.Context, userID, name, desc string) error {
	if strings.TrimSpace(userID) == "" {
		return errors.New("user ID is required")
	}
	return s.repo.UpdateStore(ctx, userID, name, desc)
}

func (s *Service) GetVendorAnalytics(ctx context.Context, userID string) (*VendorAnalytics, error) {
	if strings.TrimSpace(userID) == "" {
		return nil, errors.New("user ID is required")
	}
	return s.repo.GetVendorAnalytics(ctx, userID)
}

func (s *Service) ListVendorsAdmin(ctx context.Context) ([]AdminVendorItem, error) {
	return s.repo.ListVendorsAdmin(ctx)
}

func (s *Service) VerifyVendor(ctx context.Context, vendorID, status string, rate float64) error {
	if strings.TrimSpace(vendorID) == "" {
		return errors.New("vendor ID is required")
	}
	if strings.TrimSpace(status) == "" {
		status = "APPROVED"
	}
	if rate <= 0 {
		rate = 5.0
	}
	return s.repo.VerifyVendor(ctx, vendorID, status, rate)
}
