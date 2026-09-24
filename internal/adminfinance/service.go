package adminfinance

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

func (s *Service) GetOverview(ctx context.Context) (*FinancialOverview, error) {
	return s.repo.GetOverview(ctx)
}

func (s *Service) ListPayoutRequests(ctx context.Context, status string) ([]AdminPayoutItem, error) {
	return s.repo.ListPayoutRequests(ctx, status)
}

func (s *Service) ListEscrowHoldings(ctx context.Context) ([]EscrowHoldingItem, error) {
	return s.repo.ListEscrowHoldings(ctx)
}

func (s *Service) ListCommissionsEarned(ctx context.Context) ([]CommissionItem, error) {
	return s.repo.ListCommissionsEarned(ctx)
}

func (s *Service) GetFinancialReports(ctx context.Context) ([]ReportJobItem, error) {
	return s.repo.GetFinancialReports(ctx)
}

func (s *Service) ApprovePayout(ctx context.Context, payoutID string) error {
	if strings.TrimSpace(payoutID) == "" {
		return errors.New("payout_id is required")
	}
	return s.repo.ApprovePayout(ctx, payoutID)
}

func (s *Service) ProcessPayout(ctx context.Context, payoutID, proofRef string) error {
	if strings.TrimSpace(payoutID) == "" {
		return errors.New("payout_id is required")
	}
	return s.repo.ProcessPayout(ctx, payoutID, proofRef)
}

func (s *Service) ReleaseEscrow(ctx context.Context, userID string, amount float64) error {
	if strings.TrimSpace(userID) == "" || amount <= 0 {
		return errors.New("user_id and amount > 0 are required")
	}
	return s.repo.ReleaseEscrow(ctx, userID, amount)
}

func (s *Service) ManualCreditWallet(ctx context.Context, userID, typ string, amount float64) error {
	if strings.TrimSpace(userID) == "" || amount <= 0 {
		return errors.New("user_id and amount > 0 are required")
	}
	if typ == "" {
		typ = "CREDIT"
	}
	return s.repo.ManualCreditWallet(ctx, userID, typ, amount)
}

func (s *Service) UpdatePayoutStatus(ctx context.Context, payoutID, status string) error {
	if strings.TrimSpace(payoutID) == "" || strings.TrimSpace(status) == "" {
		return errors.New("payout_id and status are required")
	}
	return s.repo.UpdatePayoutStatus(ctx, payoutID, status)
}

func (s *Service) UpdateCommissionRates(ctx context.Context, categoryID string, rate float64) error {
	if rate <= 0 || rate > 50 {
		return errors.New("commission_rate must be between 0 and 50")
	}
	return s.repo.UpdateCommissionRates(ctx, categoryID, rate)
}
