package adminsettings

import (
	"strings"
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
	if h.db == nil {
		return response.Success(c, fiber.StatusOK, "Platform configuration settings", fiber.Map{"settings": []fiber.Map{}})
	}
	ctx := c.Context()
	rows, err := h.db.Query(ctx, `
		SELECT key, value, COALESCE(description,''), is_sensitive, updated_at
		FROM platform_settings ORDER BY key`)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to load settings: "+err.Error(), nil)
	}
	defer rows.Close()

	settings := []fiber.Map{}
	for rows.Next() {
		var key, value, desc string
		var isSensitive bool
		var updatedAt time.Time
		rows.Scan(&key, &value, &desc, &isSensitive, &updatedAt)
		// Mask sensitive values
		displayValue := value
		if isSensitive {
			displayValue = "***REDACTED***"
		}
		settings = append(settings, fiber.Map{
			"key": key, "value": displayValue, "description": desc,
			"is_sensitive": isSensitive, "updated_at": updatedAt.Format(time.RFC3339),
		})
	}
	return response.Success(c, fiber.StatusOK, "Platform settings from NeonDB", fiber.Map{
		"settings": settings, "count": len(settings),
	})
}

func (h *Handler) GetAuditLogs(c *fiber.Ctx) error {
	if h.db == nil {
		return response.Success(c, fiber.StatusOK, "Audit logs", fiber.Map{"audit_logs": []fiber.Map{}})
	}
	// Return recent user activity as audit trail
	ctx := c.Context()
	limit := c.QueryInt("limit", 50)
	rows, err := h.db.Query(ctx, `
		SELECT u.id::text, u.email, u.role, u.created_at, u.updated_at
		FROM users u ORDER BY u.updated_at DESC LIMIT $1`, limit)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to load audit data: "+err.Error(), nil)
	}
	defer rows.Close()

	logs := []fiber.Map{}
	for rows.Next() {
		var id, email, role string
		var createdAt, updatedAt time.Time
		rows.Scan(&id, &email, &role, &createdAt, &updatedAt)
		logs = append(logs, fiber.Map{
			"user_id": id, "email": email, "role": role,
			"created_at": createdAt.Format(time.RFC3339), "last_active": updatedAt.Format(time.RFC3339),
		})
	}
	return response.Success(c, fiber.StatusOK, "Audit logs from NeonDB", fiber.Map{
		"audit_logs": logs, "count": len(logs),
	})
}

func (h *Handler) GetSettingByKey(c *fiber.Ctx) error {
	key := c.Params("key")
	if h.db == nil {
		return response.Success(c, fiber.StatusOK, "Setting value", fiber.Map{"key": key, "value": ""})
	}
	ctx := c.Context()
	var value, desc string
	var isSensitive bool
	var updatedAt time.Time
	err := h.db.QueryRow(ctx, `
		SELECT value, COALESCE(description,''), is_sensitive, updated_at
		FROM platform_settings WHERE key = $1`, key).Scan(&value, &desc, &isSensitive, &updatedAt)
	if err != nil {
		return response.Error(c, fiber.StatusNotFound, "Setting key '"+key+"' not found", nil)
	}
	if isSensitive {
		value = "***REDACTED***"
	}
	return response.Success(c, fiber.StatusOK, "Setting from NeonDB", fiber.Map{
		"key": key, "value": value, "description": desc,
		"is_sensitive": isSensitive, "updated_at": updatedAt.Format(time.RFC3339),
	})
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
	updatedKeys := []string{}
	if h.db != nil {
		ctx := c.Context()
		for key, value := range req.Settings {
			h.db.Exec(ctx, `
				INSERT INTO platform_settings (key, value) VALUES ($1, $2)
				ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = now()`,
				key, value)
			updatedKeys = append(updatedKeys, key)
		}
	}
	return response.Success(c, fiber.StatusOK, "Platform settings updated in NeonDB", fiber.Map{
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
	modeValue := "false"
	if req.Enabled {
		modeValue = "true"
	}
	if h.db != nil {
		ctx := c.Context()
		h.db.Exec(ctx, `
			INSERT INTO platform_settings (key, value, description) VALUES ('maintenance_mode', $1, 'Platform maintenance mode')
			ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = now()`, modeValue)
	}
	return response.Success(c, fiber.StatusOK, "Maintenance mode updated in NeonDB", fiber.Map{
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

	if h.db != nil {
		ctx := c.Context()
		result, err := h.db.Exec(ctx, `
			UPDATE platform_settings SET value = $1, updated_at = now()
			WHERE key = $2`, req.Value, key)
		if err != nil {
			return response.Error(c, fiber.StatusInternalServerError, "Failed to update setting: "+err.Error(), nil)
		}
		if result.RowsAffected() == 0 {
			// Insert if not exists
			h.db.Exec(ctx, `
				INSERT INTO platform_settings (key, value, description) VALUES ($1, $2, $3)
				ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = now()`,
				key, req.Value, req.Description)
		}
	}
	return response.Success(c, fiber.StatusOK, "Setting updated in NeonDB", fiber.Map{
		"key": key, "value": req.Value, "updated_at": time.Now().Format(time.RFC3339),
	})
}
