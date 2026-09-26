package affiliate

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

func (s *Service) GetProfile(ctx context.Context, userID string) (*AffiliateProfile, error) {
	if strings.TrimSpace(userID) == "" {
		return nil, errors.New("user ID is required")
	}
	return s.repo.GetProfileByUserID(ctx, userID)
}

func (s *Service) GetLinks(ctx context.Context, userID string) ([]AffiliateLinkItem, error) {
	if strings.TrimSpace(userID) == "" {
		return nil, errors.New("user ID is required")
	}
	return s.repo.GetLinksByUserID(ctx, userID)
}

func (s *Service) GetClicks(ctx context.Context, userID string) (int64, error) {
	return s.repo.GetClicks(ctx, userID)
}

func (s *Service) GetEarnings(ctx context.Context, userID string) (float64, error) {
	return s.repo.GetEarnings(ctx, userID)
}

func (s *Service) Apply(ctx context.Context, userID string) (string, string, error) {
	if strings.TrimSpace(userID) == "" {
		return "", "", errors.New("user ID is required")
	}
	return s.repo.Apply(ctx, userID)
}

func (s *Service) CreateLink(ctx context.Context, userID, productID, fullURL string) (string, string, error) {
	if strings.TrimSpace(userID) == "" {
		return "", "", errors.New("user ID is required")
	}
	if strings.TrimSpace(fullURL) == "" {
		return "", "", errors.New("full_url is required")
	}
	return s.repo.CreateLink(ctx, userID, productID, fullURL)
}

func (s *Service) Withdraw(ctx context.Context, userID string, amount float64) (string, error) {
	if strings.TrimSpace(userID) == "" {
		return "", errors.New("user ID is required")
	}
	if amount <= 0 {
		return "", errors.New("amount must be greater than 0")
	}
	return s.repo.RequestWithdrawal(ctx, userID, amount)
}

func (s *Service) UpdateProfile(ctx context.Context, userID string) error {
	return s.repo.UpdateProfile(ctx, userID)
}

func (s *Service) DeleteLink(ctx context.Context, linkID, userID string) error {
	return s.repo.DeleteLink(ctx, linkID, userID)
}

func (s *Service) ListApplicationsAdmin(ctx context.Context, status string) ([]AffiliateAdminApplicationItem, error) {
	return s.repo.ListApplicationsAdmin(ctx, status)
}

func (s *Service) ReviewApplicationAdmin(ctx context.Context, profileID, status string) (int64, error) {
	return s.repo.ReviewApplicationAdmin(ctx, profileID, status)
}

