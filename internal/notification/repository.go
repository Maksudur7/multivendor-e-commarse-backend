package notification

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

type NotificationItem struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Message   string `json:"message"`
	IsRead    bool   `json:"is_read"`
	CreatedAt string `json:"created_at"`
}

func (r *Repository) GetUserNotifications(ctx context.Context, userID string) ([]NotificationItem, error) {
	if r.db == nil {
		return []NotificationItem{}, nil
	}
	rows, err := r.db.Query(ctx, `
		SELECT id::text, title, message, is_read, created_at FROM notifications WHERE user_id::text = $1 ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var notifs []NotificationItem
	for rows.Next() {
		var item NotificationItem
		var dt time.Time
		if err := rows.Scan(&item.ID, &item.Title, &item.Message, &item.IsRead, &dt); err == nil {
			item.CreatedAt = dt.Format(time.RFC3339)
			notifs = append(notifs, item)
		}
	}
	if notifs == nil { notifs = []NotificationItem{} }
	return notifs, nil
}
