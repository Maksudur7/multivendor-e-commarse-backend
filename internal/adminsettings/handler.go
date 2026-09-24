package adminsettings

import (
	"strings"
	"time"

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
	stg := router.Group("/admin/settings", authMiddleware)

	// 6 Admin Settings Endpoints
	stg.Get("/", h.GetSettings)
	stg.Get("/audit-logs", h.GetAuditLogs)
	stg.Get("/:key", h.GetSettingByKey)
	stg.Put("/", h.UpdateSettingsBatch)
	stg.Put("/maintenance-mode", h.ToggleMaintenanceMode)
	stg.Put("/:key", h.UpdateSettingByKey)
}

func (h *Handler) GetSettings(c *fiber.Ctx) error {
	settings, err := h.service.GetSettings(c.Context())
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to load settings: "+err.Error(), nil)
	}
	return response.Success(c, fiber.StatusOK, "Platform settings loaded", fiber.Map{
		"settings": settings, "count": len(settings),
	})
}

func (h *Handler) GetAuditLogs(c *fiber.Ctx) error {
	limit := c.QueryInt("limit", 50)
	logs, err := h.service.GetAuditLogs(c.Context(), limit)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to load audit data: "+err.Error(), nil)
	}
	return response.Success(c, fiber.StatusOK, "Audit logs loaded", fiber.Map{
		"audit_logs": logs, "count": len(logs),
	})
}

func (h *Handler) GetSettingByKey(c *fiber.Ctx) error {
	key := c.Params("key")
	setting, err := h.service.GetSettingByKey(c.Context(), key)
	if err != nil {
		return response.Error(c, fiber.StatusNotFound, "Setting key '"+key+"' not found", nil)
	}
	return response.Success(c, fiber.StatusOK, "Setting loaded", setting)
}

type BatchSettingsReq struct {
	Settings map[string]string `json:"settings"`
}

func (h *Handler) UpdateSettingsBatch(c *fiber.Ctx) error {
	var req BatchSettingsReq
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid input: "+err.Error())
	}
	if len(req.Settings) == 0 {
		return response.ValidationError(c, map[string]string{"settings": "at least one setting required"})
	}

	updatedKeys, err := h.service.UpdateSettingsBatch(c.Context(), req.Settings)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, err.Error(), nil)
	}

	return response.Success(c, fiber.StatusOK, "Platform settings updated", fiber.Map{
		"updated_keys": updatedKeys, "count": len(updatedKeys),
	})
}

type MaintenanceModeReq struct {
	Enabled bool   `json:"enabled"`
	Message string `json:"message"`
}

func (h *Handler) ToggleMaintenanceMode(c *fiber.Ctx) error {
	var req MaintenanceModeReq
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid input: "+err.Error())
	}

	if err := h.service.ToggleMaintenanceMode(c.Context(), req.Enabled); err != nil {
		return response.Error(c, fiber.StatusInternalServerError, err.Error(), nil)
	}

	return response.Success(c, fiber.StatusOK, "Maintenance mode updated", fiber.Map{
		"maintenance_mode": req.Enabled,
		"message":          req.Message,
	})
}

type UpdateSettingReq struct {
	Value       string `json:"value"`
	Description string `json:"description"`
}

func (h *Handler) UpdateSettingByKey(c *fiber.Ctx) error {
	key := strings.TrimSpace(c.Params("key"))
	var req UpdateSettingReq
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid input: "+err.Error())
	}
	if req.Value == "" {
		return response.ValidationError(c, map[string]string{"value": "required"})
	}

	if err := h.service.UpdateSettingByKey(c.Context(), key, req.Value, req.Description); err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to update setting: "+err.Error(), nil)
	}

	return response.Success(c, fiber.StatusOK, "Setting updated", fiber.Map{
		"key": key, "value": req.Value, "updated_at": time.Now().Format(time.RFC3339),
	})
}
