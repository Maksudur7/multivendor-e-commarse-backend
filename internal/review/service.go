package review

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

func (s *Service) GetProductReviews(ctx context.Context, productID string) ([]ReviewItem, error) {
	if strings.TrimSpace(productID) == "" {
		return nil, errors.New("product_id is required")
	}
	return s.repo.GetProductReviews(ctx, productID)
}

func (s *Service) GetMyReviews(ctx context.Context, userID string) ([]UserReviewItem, error) {
	if strings.TrimSpace(userID) == "" {
		return nil, errors.New("user ID is required")
	}
	return s.repo.GetMyReviews(ctx, userID)
}

func (s *Service) CreateReview(ctx context.Context, productID, userID string, rating int, title, comment string) (string, error) {
	if strings.TrimSpace(productID) == "" {
		return "", errors.New("product_id is required")
	}
	if rating < 1 || rating > 5 {
		return "", errors.New("rating must be between 1 and 5")
	}
	return s.repo.CreateReview(ctx, productID, userID, rating, title, comment)
}

func (s *Service) MarkHelpful(ctx context.Context, reviewID string) error {
	if strings.TrimSpace(reviewID) == "" {
		return errors.New("review_id is required")
	}
	return s.repo.MarkHelpful(ctx, reviewID)
}

func (s *Service) UpdateReview(ctx context.Context, reviewID, userID string, rating int, title, comment string) error {
	if strings.TrimSpace(reviewID) == "" {
		return errors.New("review_id is required")
	}
	return s.repo.UpdateReview(ctx, reviewID, userID, rating, title, comment)
}

func (s *Service) DeleteReview(ctx context.Context, reviewID, userID string) error {
	if strings.TrimSpace(reviewID) == "" {
		return errors.New("review_id is required")
	}
	return s.repo.DeleteReview(ctx, reviewID, userID)
}
