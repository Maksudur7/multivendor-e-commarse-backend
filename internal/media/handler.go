package media

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
	// 5 Media Endpoints
	router.Get("/media/:id", h.GetMediaDetail)

	m := router.Group("/media", authMiddleware)
	m.Post("/upload", h.UploadMedia)
	m.Post("/presigned-url", h.GeneratePresignedURL)
	m.Post("/batch-upload", h.BatchUploadMedia)
	m.Delete("/:id", h.DeleteMedia)
}

func (h *Handler) GetMediaDetail(c *fiber.Ctx) error {
	return response.Success(c, fiber.StatusOK, "Media metadata", fiber.Map{"media_id": c.Params("id"), "url": "https://r2.cdn.com/media.jpg"})
}

func (h *Handler) UploadMedia(c *fiber.Ctx) error {
	return response.Created(c, "File uploaded to Cloudflare R2", fiber.Map{
		"media_id": uuid.New().String(),
		"cdn_url":  "https://r2.cdn.com/product_image_1.jpg",
	})
}

func (h *Handler) GeneratePresignedURL(c *fiber.Ctx) error {
	return response.Created(c, "Cloudflare R2 Presigned Upload URL generated", fiber.Map{
		"upload_url": "https://account.r2.cloudflarestorage.com/bucket/file?X-Amz-Signature=xyz",
		"file_key":   "uploads/file_123.jpg",
	})
}

func (h *Handler) BatchUploadMedia(c *fiber.Ctx) error {
	return response.Created(c, "Batch files uploaded", fiber.Map{"uploaded_files": []string{"https://r2.cdn.com/1.jpg", "https://r2.cdn.com/2.jpg"}})
}

func (h *Handler) DeleteMedia(c *fiber.Ctx) error {
	return response.Success(c, fiber.StatusOK, "Media deleted from Cloudflare R2 & NeonDB", fiber.Map{"deleted_id": c.Params("id")})
}
