package reseller

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) GetPublicStore(ctx context.Context, slug string) (*PublicResellerStore, error) {
	if strings.TrimSpace(slug) == "" {
		return nil, errors.New("slug is required")
	}
	return s.repo.GetPublicStore(ctx, slug)
}

func (s *Service) GetMyStore(ctx context.Context, userID string) (*ResellerProfile, error) {
	if strings.TrimSpace(userID) == "" {
		return nil, errors.New("user ID is required")
	}
	return s.repo.GetResellerByUserID(ctx, userID)
}

func (s *Service) CreateStore(ctx context.Context, userID, name, slug string) (string, error) {
	if strings.TrimSpace(userID) == "" {
		return "", errors.New("user ID is required")
	}
	if name == "" && slug == "" {
		return "", errors.New("store_name or store_slug is required")
	}
	if name == "" {
		name = slug
	}
	return s.repo.CreateStore(ctx, userID, name)
}

type MarginCalculation struct {
	WholesalePrice   float64 `json:"wholesale_price"`
	TargetPrice      float64 `json:"target_price"`
	ExpectedProfit   float64 `json:"expected_profit"`
	ProfitPercentage string  `json:"profit_percentage"`
}

func (s *Service) CalculateMargin(wholesale, target float64) MarginCalculation {
	profit := target - wholesale
	pct := 0.0
	if wholesale > 0 {
		pct = (profit / wholesale) * 100
	}
	return MarginCalculation{
		WholesalePrice:   wholesale,
		TargetPrice:      target,
		ExpectedProfit:   profit,
		ProfitPercentage: fmt.Sprintf("%.2f%%", pct),
	}
}

func (s *Service) GenerateShareableLink(userID, productID, channel string) (string, string) {
	pID := productID
	if len(pID) > 8 {
		pID = pID[:8]
	}
	uID := userID
	if len(uID) > 8 {
		uID = uID[:8]
	}
	refCode := fmt.Sprintf("RES-%s-%s", uID, pID)
	shareLink := fmt.Sprintf("https://buy.platform.com/p/%s?ref=%s", productID, refCode)
	return shareLink, refCode
}
