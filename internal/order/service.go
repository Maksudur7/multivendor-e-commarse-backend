package order

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

func (s *Service) GetCustomerOrders(ctx context.Context, userID string) ([]OrderSummary, error) {
	if strings.TrimSpace(userID) == "" {
		return nil, errors.New("user ID is required")
	}
	return s.repo.GetCustomerOrders(ctx, userID)
}

func (s *Service) GetCustomerOrderDetail(ctx context.Context, orderID, userID string) (*OrderDetail, error) {
	if strings.TrimSpace(orderID) == "" || strings.TrimSpace(userID) == "" {
		return nil, errors.New("order ID and user ID are required")
	}
	return s.repo.GetCustomerOrderDetail(ctx, orderID, userID)
}

func (s *Service) ProcessCheckout(ctx context.Context, userID, shippingAddress, paymentMethod string) (*CheckoutResult, error) {
	if strings.TrimSpace(userID) == "" {
		return nil, errors.New("user ID is required")
	}
	if strings.TrimSpace(shippingAddress) == "" {
		shippingAddress = "Default Delivery Address, Dhaka, Bangladesh"
	}
	if strings.TrimSpace(paymentMethod) == "" {
		paymentMethod = "COD"
	}
	return s.repo.ProcessCheckout(ctx, userID, shippingAddress, paymentMethod)
}

func (s *Service) CancelCustomerOrder(ctx context.Context, orderID, userID string) error {
	if strings.TrimSpace(orderID) == "" || strings.TrimSpace(userID) == "" {
		return errors.New("order ID and user ID are required")
	}
	return s.repo.CancelCustomerOrder(ctx, orderID, userID)
}

func (s *Service) ListSellerOrders(ctx context.Context) ([]map[string]interface{}, error) {
	return s.repo.ListSellerOrders(ctx)
}

func (s *Service) ListAdminOrders(ctx context.Context) ([]map[string]interface{}, error) {
	return s.repo.ListAdminOrders(ctx)
}

func (s *Service) UpdateOrderStatusAdmin(ctx context.Context, orderID, status string) error {
	if strings.TrimSpace(orderID) == "" {
		return errors.New("order ID is required")
	}
	if strings.TrimSpace(status) == "" {
		return errors.New("status is required")
	}
	return s.repo.UpdateOrderStatusAdmin(ctx, orderID, status)
}

func (s *Service) GetOrderItems(ctx context.Context, orderID string) ([]map[string]interface{}, error) {
	if strings.TrimSpace(orderID) == "" {
		return nil, errors.New("order ID is required")
	}
	return s.repo.GetOrderItems(ctx, orderID)
}
