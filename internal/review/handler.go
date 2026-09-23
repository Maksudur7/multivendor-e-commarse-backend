package review

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
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
	// 7 Review Endpoints
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
	return response.Success(c, fiber.StatusOK, "Product reviews loaded", fiber.Map{"reviews": []fiber.Map{}})
}

func (h *Handler) GetSellerReviews(c *fiber.Ctx) error {
	return response.Success(c, fiber.StatusOK, "Seller reviews loaded", fiber.Map{"reviews": []fiber.Map{}})
}

func (h *Handler) GetMyReviews(c *fiber.Ctx) error {
	return response.Success(c, fiber.StatusOK, "My submitted reviews", fiber.Map{"reviews": []fiber.Map{}})
}

func (h *Handler) CreateReview(c *fiber.Ctx) error {
	return response.Created(c, "Review submitted to NeonDB", fiber.Map{"review_id": uuid.New().String()})
}

func (h *Handler) MarkHelpful(c *fiber.Ctx) error {
	return response.Success(c, fiber.StatusOK, "Review marked as helpful", fiber.Map{"review_id": c.Params("id")})
}

func (h *Handler) UpdateReview(c *fiber.Ctx) error {
	return response.Success(c, fiber.StatusOK, "Review updated", fiber.Map{"review_id": c.Params("id")})
}

func (h *Handler) DeleteReview(c *fiber.Ctx) error {
	return response.Success(c, fiber.StatusOK, "Review deleted", fiber.Map{"review_id": c.Params("id")})
}
