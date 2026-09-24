package catalog

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) ListPublicProducts(ctx context.Context) ([]PublicProductItem, error) {
	return s.repo.ListPublicProducts(ctx)
}

func (s *Service) GetPublicProductDetail(ctx context.Context, slug string) (*ProductDetail, error) {
	if strings.TrimSpace(slug) == "" {
		return nil, errors.New("slug is required")
	}
	return s.repo.GetPublicProductBySlug(ctx, slug)
}

func (s *Service) ListPublicCategories(ctx context.Context) ([]CategoryItem, error) {
	return s.repo.ListPublicCategories(ctx)
}

func (s *Service) CreateSellerProduct(ctx context.Context, title, slug string, price float64) (string, string, error) {
	if strings.TrimSpace(title) == "" {
		return "", "", errors.New("title is required")
	}
	if strings.TrimSpace(slug) == "" {
		slug = fmt.Sprintf("prod-%d", time.Now().UnixNano()/1e6)
	}
	id, err := s.repo.CreateSellerProduct(ctx, title, slug, price)
	return id, slug, err
}

func (s *Service) CreateCategoryAdmin(ctx context.Context, name, slug string) (string, string, error) {
	if strings.TrimSpace(name) == "" {
		name = "New Category"
	}
	if strings.TrimSpace(slug) == "" {
		slug = fmt.Sprintf("cat-%d", time.Now().UnixNano()/1e6)
	}
	id, err := s.repo.CreateCategory(ctx, name, slug)
	return id, slug, err
}
