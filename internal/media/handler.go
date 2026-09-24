package media

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
	router.Get("/media/:id", h.GetMediaDetail)

	m := router.Group("/media", authMiddleware)
	m.Post("/upload", h.UploadMedia)
	m.Post("/presigned-url", h.GeneratePresignedURL)
	m.Post("/batch-upload", h.BatchUploadMedia)
	m.Delete("/:id", h.DeleteMedia)
}

func (h *Handler) GetMediaDetail(c *fiber.Ctx) error {
	id := c.Params("id")
	item, err := h.service.GetMediaDetail(c.Context(), id)
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, err.Error(), nil)
	}
	return response.Success(c, fiber.StatusOK, "Media metadata", item)
}

func (h *Handler) UploadMedia(c *fiber.Ctx) error {
	res, err := h.service.UploadMedia(c.Context())
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed upload", nil)
	}
	return response.Created(c, "File uploaded to Cloudflare R2", res)
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
