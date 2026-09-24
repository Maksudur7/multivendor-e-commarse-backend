package review

import (
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/yourusername/ecom-backend/pkg/response"
)

type Handler struct {
	repo    *Repository
	service *Service
}

func NewHandler(db *pgxpool.Pool) *Handler {
	repo := NewRepository(db)
	service := NewService(repo)
	return &Handler{repo: repo, service: service}
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
	reviews, err := h.service.GetProductReviews(c.Context(), productID)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to load reviews", nil)
	}
	return response.Success(c, fiber.StatusOK, "Product reviews loaded", fiber.Map{
		"product_id": productID, "reviews": reviews, "count": len(reviews),
	})
}

func (h *Handler) GetSellerReviews(c *fiber.Ctx) error {
	sellerID := c.Params("sellerId")
	return response.Success(c, fiber.StatusOK, "Seller reviews loaded", fiber.Map{"seller_id": sellerID, "reviews": []fiber.Map{}})
}

func (h *Handler) GetMyReviews(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)
	reviews, err := h.service.GetMyReviews(c.Context(), userID)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to load reviews", nil)
	}
	return response.Success(c, fiber.StatusOK, "User reviews loaded", fiber.Map{"reviews": reviews})
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

	reviewID, err := h.service.CreateReview(c.Context(), req.ProductID, userID, req.Rating, req.Title, req.Comment)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to submit review: "+err.Error(), nil)
	}
	return response.Created(c, "Review submitted successfully", fiber.Map{"review_id": reviewID, "rating": req.Rating})
}

func (h *Handler) MarkHelpful(c *fiber.Ctx) error {
	reviewID := c.Params("id")
	if err := h.service.MarkHelpful(c.Context(), reviewID); err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to mark helpful", nil)
	}
	return response.Success(c, fiber.StatusOK, "Review marked as helpful", fiber.Map{"review_id": reviewID})
}

func (h *Handler) UpdateReview(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)
	reviewID := c.Params("id")
	var req CreateReviewReq
	_ = c.BodyParser(&req)
	if err := h.service.UpdateReview(c.Context(), reviewID, userID, req.Rating, req.Title, req.Comment); err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to update review", nil)
	}
	return response.Success(c, fiber.StatusOK, "Review updated", fiber.Map{"review_id": reviewID})
}

func (h *Handler) DeleteReview(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)
	reviewID := c.Params("id")
	if err := h.service.DeleteReview(c.Context(), reviewID, userID); err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to delete review", nil)
	}
	return response.Success(c, fiber.StatusOK, "Review deleted", fiber.Map{"review_id": reviewID})
}
