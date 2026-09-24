package inventory

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

func (s *Service) GetStockLevels(ctx context.Context) ([]StockItem, error) {
	return s.repo.GetStockLevels(ctx)
}

func (s *Service) ListWarehouses(ctx context.Context) ([]WarehouseItem, error) {
	return s.repo.ListWarehouses(ctx)
}

func (s *Service) AdjustStock(ctx context.Context, productID string, quantity int, warehouse string) (string, error) {
	if strings.TrimSpace(productID) == "" {
		return "", errors.New("product_id is required")
	}
	if strings.TrimSpace(warehouse) == "" {
		warehouse = "Main Hub Dhaka"
	}
	return s.repo.AdjustStock(ctx, productID, quantity, warehouse)
}
