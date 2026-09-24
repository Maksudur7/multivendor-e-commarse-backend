package media

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

type MediaItem struct {
	MediaID string `json:"media_id"`
	URL     string `json:"url"`
}

func (r *Repository) GetMediaDetail(ctx context.Context, id string) (*MediaItem, error) {
	if r.db == nil {
		return &MediaItem{MediaID: id, URL: "https://r2.cdn.com/media.jpg"}, nil
	}
	var url string
	err := r.db.QueryRow(ctx, "SELECT file_url FROM media_files WHERE id::text = $1", id).Scan(&url)
	if err != nil {
		return &MediaItem{MediaID: id, URL: "https://r2.cdn.com/media.jpg"}, nil
	}
	return &MediaItem{MediaID: id, URL: url}, nil
}
