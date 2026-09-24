package wallet

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

func (s *Service) GetWalletBalance(ctx context.Context, userID string) (*WalletSummary, error) {
	if strings.TrimSpace(userID) == "" {
		return nil, errors.New("user ID is required")
	}
	return s.repo.GetWalletBalance(ctx, userID)
}

func (s *Service) GetLedgerHistory(ctx context.Context, userID string) ([]TransactionItem, error) {
	if strings.TrimSpace(userID) == "" {
		return nil, errors.New("user ID is required")
	}
	return s.repo.GetLedgerHistory(ctx, userID)
}

func (s *Service) RequestWithdrawal(ctx context.Context, userID string, amount float64, method, details string) (string, error) {
	if strings.TrimSpace(userID) == "" {
		return "", errors.New("user ID is required")
	}
	if amount <= 0 || strings.TrimSpace(method) == "" || strings.TrimSpace(details) == "" {
		return "", errors.New("amount > 0, payment_method, and account_details are required")
	}
	return s.repo.RequestWithdrawal(ctx, userID, amount, method, details)
}

func (s *Service) ListWithdrawalRequestsAdmin(ctx context.Context) ([]AdminPayoutRequestItem, error) {
	return s.repo.ListWithdrawalRequestsAdmin(ctx)
}

func (s *Service) ProcessWithdrawalAdmin(ctx context.Context, withdrawalID, status, proofRef string) error {
	if strings.TrimSpace(withdrawalID) == "" {
		return errors.New("withdrawal_id is required")
	}
	if status == "" {
		status = "APPROVED"
	}
	return s.repo.ProcessWithdrawalAdmin(ctx, withdrawalID, status, proofRef)
}
