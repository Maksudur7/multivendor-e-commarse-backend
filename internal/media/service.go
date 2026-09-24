package media

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) GetMediaDetail(ctx context.Context, id string) (*MediaItem, error) {
	if strings.TrimSpace(id) == "" {
		return nil, errors.New("media_id is required")
	}
	return s.repo.GetMediaDetail(ctx, id)
}

type UploadResult struct {
	MediaID string `json:"media_id"`
	CDNURL  string `json:"cdn_url"`
}

func (s *Service) UploadMedia(ctx context.Context) (*UploadResult, error) {
	mediaID := uuid.New().String()
	return &UploadResult{
		MediaID: mediaID,
		CDNURL:  "https://r2.cdn.com/product_image_1.jpg",
	}, nil
}
