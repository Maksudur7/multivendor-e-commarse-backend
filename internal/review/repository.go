package review

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

type ReviewItem struct {
	ReviewID           string `json:"review_id"`
	Rating             int    `json:"rating"`
	Title              string `json:"title"`
	Comment            string `json:"comment"`
	IsVerifiedPurchase bool   `json:"is_verified_purchase"`
	HelpfulCount       int    `json:"helpful_count"`
	Author             string `json:"author"`
	CreatedAt          string `json:"created_at"`
}

func (r *Repository) GetProductReviews(ctx context.Context, productID string) ([]ReviewItem, error) {
	if r.db == nil {
		return []ReviewItem{}, nil
	}
	rows, err := r.db.Query(ctx, `
		SELECT r.id::text, r.rating, r.title, r.comment, r.is_verified_purchase, r.helpful_count, r.created_at, COALESCE(u.full_name, u.email)
		FROM product_reviews r LEFT JOIN users u ON u.id = r.user_id WHERE r.product_id::text = $1 ORDER BY r.created_at DESC`, productID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []ReviewItem
	for rows.Next() {
		var item ReviewItem
		var dt time.Time
		if err := rows.Scan(&item.ReviewID, &item.Rating, &item.Title, &item.Comment, &item.IsVerifiedPurchase, &item.HelpfulCount, &dt, &item.Author); err == nil {
			item.CreatedAt = dt.Format(time.RFC3339)
			list = append(list, item)
		}
	}
	if list == nil { list = []ReviewItem{} }
	return list, nil
}

type UserReviewItem struct {
	ReviewID  string `json:"review_id"`
	ProductID string `json:"product_id"`
	Rating    int    `json:"rating"`
	Title     string `json:"title"`
	Comment   string `json:"comment"`
	CreatedAt string `json:"created_at"`
}

func (r *Repository) GetMyReviews(ctx context.Context, userID string) ([]UserReviewItem, error) {
	if r.db == nil {
		return []UserReviewItem{}, nil
	}
	rows, err := r.db.Query(ctx, `
		SELECT id::text, product_id::text, rating, title, comment, created_at FROM product_reviews WHERE user_id::text = $1 ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []UserReviewItem
	for rows.Next() {
		var item UserReviewItem
		var dt time.Time
		if err := rows.Scan(&item.ReviewID, &item.ProductID, &item.Rating, &item.Title, &item.Comment, &dt); err == nil {
			item.CreatedAt = dt.Format(time.RFC3339)
			list = append(list, item)
		}
	}
	if list == nil { list = []UserReviewItem{} }
	return list, nil
}

func (r *Repository) CreateReview(ctx context.Context, productID, userID string, rating int, title, comment string) (string, error) {
	if r.db == nil {
		return "", errors.New("database connection unavailable")
	}
	var reviewID string
	err := r.db.QueryRow(ctx, `
		INSERT INTO product_reviews (product_id, user_id, rating, title, comment, is_verified_purchase)
		VALUES ($1::uuid, $2::uuid, $3, $4, $5, true)
		RETURNING id::text`,
		productID, userID, rating, title, comment,
	).Scan(&reviewID)
	if err != nil {
		return "", err
	}
	return reviewID, nil
}

func (r *Repository) MarkHelpful(ctx context.Context, reviewID string) error {
	if r.db == nil { return nil }
	_, err := r.db.Exec(ctx, "UPDATE product_reviews SET helpful_count = helpful_count + 1 WHERE id::text = $1", reviewID)
	return err
}

func (r *Repository) UpdateReview(ctx context.Context, reviewID, userID string, rating int, title, comment string) error {
	if r.db == nil { return nil }
	_, err := r.db.Exec(ctx, "UPDATE product_reviews SET rating = COALESCE(NULLIF($1,0), rating), title = COALESCE(NULLIF($2,''), title), comment = COALESCE(NULLIF($3,''), comment) WHERE id::text = $4 AND user_id::text = $5", rating, title, comment, reviewID, userID)
	return err
}

func (r *Repository) DeleteReview(ctx context.Context, reviewID, userID string) error {
	if r.db == nil { return nil }
	_, err := r.db.Exec(ctx, "DELETE FROM product_reviews WHERE id::text = $1 AND user_id::text = $2", reviewID, userID)
	return err
}
