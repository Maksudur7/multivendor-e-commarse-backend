package cart

import (
	"context"
	"fmt"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// --- Business logic helpers ---

func (s *Service) CalculateShipping(subtotal float64) float64 {
	if subtotal >= 1000.0 {
		return 0.0
	}
	return 60.0
}

func (s *Service) ResolveUnitPrice(givenPrice float64, fetchedPrice float64) float64 {
	if givenPrice > 0 {
		return givenPrice
	}
	if fetchedPrice > 0 {
		return fetchedPrice
	}
	return 1200.0
}

// --- Cart operations (wrapping repo) ---

type CartResult struct {
	Items      []CartItem
	GrandTotal float64
}

func (s *Service) GetCart(ctx context.Context, userID string) (*CartResult, error) {
	items, grandTotal, err := s.repo.GetUserCart(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to load cart: %w", err)
	}
	return &CartResult{Items: items, GrandTotal: grandTotal}, nil
}

type CartSummaryResult struct {
	Subtotal          float64
	ItemCount         int
	EstimatedShipping float64
	GrandTotal        float64
	FreeShippingFrom  float64
}

func (s *Service) GetCartSummary(ctx context.Context, userID string) (*CartSummaryResult, error) {
	subtotal, count, err := s.repo.GetCartSummary(ctx, userID)
	if err != nil {
		return nil, err
	}
	shipping := s.CalculateShipping(subtotal)
	return &CartSummaryResult{
		Subtotal:          subtotal,
		ItemCount:         count,
		EstimatedShipping: shipping,
		GrandTotal:        subtotal + shipping,
		FreeShippingFrom:  1000,
	}, nil
}

func (s *Service) GetCartCount(ctx context.Context, userID string) (int, error) {
	count, err := s.repo.GetCartCount(ctx, userID)
	return count, err
}

type AddItemInput struct {
	UserID       string
	ProductID    string
	SKUID        string
	ProductTitle string
	ImageURL     string
	UnitPrice    float64
	Quantity     int
}

func (s *Service) AddItem(ctx context.Context, input AddItemInput) (string, float64, error) {
	fetchedPrice := s.repo.FetchSKUPrice(ctx, input.SKUID)
	resolvedPrice := s.ResolveUnitPrice(input.UnitPrice, fetchedPrice)
	qty := input.Quantity
	if qty <= 0 {
		qty = 1
	}
	cartItemID, err := s.repo.AddItem(ctx, input.UserID, input.ProductID, input.SKUID, input.ProductTitle, input.ImageURL, resolvedPrice, qty)
	if err != nil {
		return "", 0, fmt.Errorf("failed to add item: %w", err)
	}
	return cartItemID, resolvedPrice, nil
}

func (s *Service) ClearCart(ctx context.Context, userID string) error {
	return s.repo.ClearCart(ctx, userID)
}

func (s *Service) UpdateQuantity(ctx context.Context, userID, itemID string, quantity int) (int64, error) {
	return s.repo.UpdateQuantity(ctx, userID, itemID, quantity)
}

func (s *Service) RemoveItem(ctx context.Context, userID, itemID string) (int64, error) {
	return s.repo.RemoveItem(ctx, userID, itemID)
}
