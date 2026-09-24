package dispute

import (
	"context"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) OpenDispute(ctx context.Context, customerID, reason, description string) (string, string, error) {
	return s.repo.OpenDispute(ctx, customerID, reason, description)
}

func (s *Service) GetMyTickets(ctx context.Context, customerID string) ([]DisputeItem, error) {
	return s.repo.GetTicketsByCustomer(ctx, customerID)
}

func (s *Service) GetAllDisputes(ctx context.Context) ([]DisputeItem, error) {
	return s.repo.GetAllDisputes(ctx)
}

func (s *Service) GetDisputeByID(ctx context.Context, id string) (*DisputeItem, error) {
	return s.repo.GetDisputeByID(ctx, id)
}

func (s *Service) ResolveDispute(ctx context.Context, ticketID, status, resolution string) error {
	return s.repo.ResolveDispute(ctx, ticketID, status, resolution)
}
