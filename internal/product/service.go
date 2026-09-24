package product

import (
	"context"
	"fmt"
	"strings"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// --- Pure helpers ---

func (s *Service) NormalizeLimitOffset(limit, offset int) (int, int) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}

func (s *Service) IsValidApprovalStatus(status string) bool {
	validStatuses := map[string]bool{"APPROVED": true, "REJECTED": true, "SUSPENDED": true}
	return validStatuses[status]
}

// --- Product operations ---

func (s *Service) ListProducts(ctx context.Context, search, categorySlug string, limit, offset int) ([]ProductSummary, int, error) {
	return s.repo.ListProducts(ctx, search, categorySlug, limit, offset)
}

func (s *Service) GetProductDetailBySlug(ctx context.Context, slug string) (*ProductDetail, error) {
	return s.repo.GetProductDetailBySlug(ctx, slug)
}

func (s *Service) GetCategoryTree(ctx context.Context) ([]CategoryItem, error) {
	return s.repo.GetCategoryTree(ctx)
}

type CreateProductInput struct {
	UserID           string
	CategoryID       string
	Title            string
	Slug             string
	Description      string
	ShortDescription string
}

func (s *Service) CreateProduct(ctx context.Context, input CreateProductInput) (string, error) {
	productID, err := s.repo.CreateProduct(ctx, input.UserID, input.CategoryID, input.Title, input.Slug, input.Description, input.ShortDescription)
	if err != nil {
		return "", err
	}
	return productID, nil
}

type AddSKUInput struct {
	ProductID      string
	SKUCode        string
	WholesalePrice float64
	RetailPrice    float64
	StockQuantity  int
}

func (s *Service) AddSKU(ctx context.Context, input AddSKUInput) (string, error) {
	skuID, err := s.repo.AddSKU(ctx, input.ProductID, input.SKUCode, input.WholesalePrice, input.RetailPrice, input.StockQuantity)
	if err != nil {
		if err.Error() == "product_not_found" {
			return "", fmt.Errorf("product_not_found")
		}
		if strings.Contains(err.Error(), "unique") {
			return "", fmt.Errorf("sku_code_exists")
		}
		return "", err
	}
	return skuID, nil
}

type CreateCategoryInput struct {
	Name           string
	Slug           string
	ParentID       string
	IconURL        string
	CommissionRate float64
}

func (s *Service) CreateCategory(ctx context.Context, input CreateCategoryInput) (string, float64, error) {
	catID, err := s.repo.CreateCategory(ctx, input.Name, input.Slug, input.ParentID, input.IconURL, input.CommissionRate)
	if err != nil {
		return "", 0, err
	}
	rate := input.CommissionRate
	if rate <= 0 {
		rate = 5.0
	}
	return catID, rate, nil
}

func (s *Service) ApproveProduct(ctx context.Context, productID, approvalStatus string) (int64, error) {
	return s.repo.ApproveProduct(ctx, productID, approvalStatus)
}
