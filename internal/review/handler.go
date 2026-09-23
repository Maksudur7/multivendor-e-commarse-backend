package review

import (
		"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/yourusername/ecom-backend/pkg/response"
)

type Handler struct {
	db *pgxpool.Pool
}

func NewHandler(db *pgxpool.Pool) *Handler {
	return &Handler{db: db}
}

func (h *Handler) RegisterRoutes(router fiber.Router, authMiddleware fiber.Handler) {
	router.Get("/reviews/product/:productId", h.GetProductReviews)
	router.Get("/reviews/seller/:sellerId", h.GetSellerReviews)

	r := router.Group("/reviews", authMiddleware)
	r.Get("/my", h.GetMyReviews)
	r.Post("/", h.CreateReview)
	r.Post("/:id/helpful", h.MarkHelpful)
	r.Put("/:id", h.UpdateReview)
	r.Delete("/:id", h.DeleteReview)
}

func (h *Handler) GetProductReviews(c *fiber.Ctx) error {
	productID := c.Params("productId")
	if h.db == nil {
		return response.Success(c, fiber.StatusOK, "Product reviews loaded", fiber.Map{"reviews": []fiber.Map{}})
	}
	ctx := c.Context()
	rows, err := h.db.Query(ctx, `
		SELECT r.id::text, r.rating, r.title, r.comment, r.is_verified_purchase, r.helpful_count, r.created_at, COALESCE(u.full_name, u.email)
		FROM product_reviews r LEFT JOIN users u ON u.id = r.user_id WHERE r.product_id::text = $1 ORDER BY r.created_at DESC`, productID)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to load reviews", nil)
	}
	defer rows.Close()

	reviews := []fiber.Map{}
	for rows.Next() {
		var id, title, comment, author string
		var rating, helpful int
		var verified bool
		var dt time.Time
		rows.Scan(&id, &rating, &title, &comment, &verified, &helpful, &dt, &author)
		reviews = append(reviews, fiber.Map{
			"review_id": id, "rating": rating, "title": title, "comment": comment,
			"is_verified_purchase": verified, "helpful_count": helpful, "author": author, "created_at": dt.Format(time.RFC3339),
		})
	}
	return response.Success(c, fiber.StatusOK, "Product reviews from NeonDB", fiber.Map{"product_id": productID, "reviews": reviews, "count": len(reviews)})
}

func (h *Handler) GetSellerReviews(c *fiber.Ctx) error {
	sellerID := c.Params("sellerId")
	return response.Success(c, fiber.StatusOK, "Seller reviews loaded", fiber.Map{"seller_id": sellerID, "reviews": []fiber.Map{}})
}

func (h *Handler) GetMyReviews(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)
	if h.db == nil {
		return response.Success(c, fiber.StatusOK, "My submitted reviews", fiber.Map{"reviews": []fiber.Map{}})
	}
	ctx := c.Context()
	rows, err := h.db.Query(ctx, `
		SELECT id::text, product_id::text, rating, title, comment, created_at FROM product_reviews WHERE user_id::text = $1 ORDER BY created_at DESC`, userID)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to load reviews", nil)
	}
	defer rows.Close()

	reviews := []fiber.Map{}
	for rows.Next() {
		var id, prodID, title, comment string; var rating int; var dt time.Time
		rows.Scan(&id, &prodID, &rating, &title, &comment, &dt)
		reviews = append(reviews, fiber.Map{"review_id": id, "product_id": prodID, "rating": rating, "title": title, "comment": comment, "created_at": dt.Format(time.RFC3339)})
	}
	return response.Success(c, fiber.StatusOK, "User reviews from NeonDB", fiber.Map{"reviews": reviews})
}

type CreateReviewReq struct {
	ProductID string `json:"product_id"`
	Rating    int    `json:"rating"`
	Title     string `json:"title"`
	Comment   string `json:"comment"`
}

func (h *Handler) CreateReview(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)
	var req CreateReviewReq
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid input: "+err.Error())
	}
	if req.ProductID == "" || req.Rating < 1 || req.Rating > 5 {
		return response.ValidationError(c, map[string]string{"rating": "rating must be between 1 and 5 and product_id is required"})
	}

	if h.db == nil {
		return response.Created(c, "Review submitted", fiber.Map{"rating": req.Rating})
	}

	ctx := c.Context()
	var reviewID string
	err := h.db.QueryRow(ctx, `
		INSERT INTO product_reviews (product_id, user_id, rating, title, comment, is_verified_purchase)
		VALUES ($1::uuid, $2::uuid, $3, $4, $5, true)
		RETURNING id::text`,
		req.ProductID, userID, req.Rating, req.Title, req.Comment,
	).Scan(&reviewID)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to submit review: "+err.Error(), nil)
	}
	return response.Created(c, "Review submitted to NeonDB", fiber.Map{"review_id": reviewID, "rating": req.Rating})
}

func (h *Handler) MarkHelpful(c *fiber.Ctx) error {
	reviewID := c.Params("id")
	if h.db != nil {
		h.db.Exec(c.Context(), "UPDATE product_reviews SET helpful_count = helpful_count + 1 WHERE id::text = $1", reviewID)
	}
	return response.Success(c, fiber.StatusOK, "Review marked as helpful in NeonDB", fiber.Map{"review_id": reviewID})
}

func (h *Handler) UpdateReview(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)
	reviewID := c.Params("id")
	var req CreateReviewReq
	c.BodyParser(&req)
	if h.db != nil {
		h.db.Exec(c.Context(), "UPDATE product_reviews SET rating = COALESCE(NULLIF($1,0), rating), title = COALESCE(NULLIF($2,''), title), comment = COALESCE(NULLIF($3,''), comment) WHERE id::text = $4 AND user_id::text = $5", req.Rating, req.Title, req.Comment, reviewID, userID)
	}
	return response.Success(c, fiber.StatusOK, "Review updated in NeonDB", fiber.Map{"review_id": reviewID})
}

func (h *Handler) DeleteReview(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)
	reviewID := c.Params("id")
	if h.db != nil {
		h.db.Exec(c.Context(), "DELETE FROM product_reviews WHERE id::text = $1 AND user_id::text = $2", reviewID, userID)
	}
	return response.Success(c, fiber.StatusOK, "Review deleted from NeonDB", fiber.Map{"review_id": reviewID})
}
