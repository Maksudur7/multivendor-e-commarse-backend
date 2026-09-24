package adminsettings

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

type SettingItem struct {
	Key         string `json:"key"`
	Value       string `json:"value"`
	Description string `json:"description"`
	IsSensitive bool   `json:"is_sensitive"`
	UpdatedAt   string `json:"updated_at"`
}

func (r *Repository) GetSettings(ctx context.Context) ([]SettingItem, error) {
	if r.db == nil {
		return []SettingItem{}, nil
	}
	rows, err := r.db.Query(ctx, `
		SELECT key, value, COALESCE(description,''), is_sensitive, updated_at
		FROM platform_settings ORDER BY key`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []SettingItem
	for rows.Next() {
		var item SettingItem
		var dt time.Time
		if err := rows.Scan(&item.Key, &item.Value, &item.Description, &item.IsSensitive, &dt); err == nil {
			if item.IsSensitive {
				item.Value = "***REDACTED***"
			}
			item.UpdatedAt = dt.Format(time.RFC3339)
			list = append(list, item)
		}
	}
	if list == nil { list = []SettingItem{} }
	return list, nil
}

type AuditLogItem struct {
	UserID     string `json:"user_id"`
	Email      string `json:"email"`
	Role       string `json:"role"`
	CreatedAt  string `json:"created_at"`
	LastActive string `json:"last_active"`
}

func (r *Repository) GetAuditLogs(ctx context.Context, limit int) ([]AuditLogItem, error) {
	if r.db == nil {
		return []AuditLogItem{}, nil
	}
	rows, err := r.db.Query(ctx, `
		SELECT u.id::text, u.email, u.role, u.created_at, u.updated_at
		FROM users u ORDER BY u.updated_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []AuditLogItem
	for rows.Next() {
		var item AuditLogItem
		var dt1, dt2 time.Time
		if err := rows.Scan(&item.UserID, &item.Email, &item.Role, &dt1, &dt2); err == nil {
			item.CreatedAt = dt1.Format(time.RFC3339)
			item.LastActive = dt2.Format(time.RFC3339)
			list = append(list, item)
		}
	}
	if list == nil { list = []AuditLogItem{} }
	return list, nil
}

func (r *Repository) GetSettingByKey(ctx context.Context, key string) (*SettingItem, error) {
	if r.db == nil {
		return &SettingItem{Key: key, Value: ""}, nil
	}
	var item SettingItem
	item.Key = key
	var dt time.Time
	err := r.db.QueryRow(ctx, `
		SELECT value, COALESCE(description,''), is_sensitive, updated_at
		FROM platform_settings WHERE key = $1`, key).Scan(&item.Value, &item.Description, &item.IsSensitive, &dt)
	if err != nil {
		return nil, err
	}
	if item.IsSensitive {
		item.Value = "***REDACTED***"
	}
	item.UpdatedAt = dt.Format(time.RFC3339)
	return &item, nil
}

func (r *Repository) UpdateSettingsBatch(ctx context.Context, settings map[string]string) ([]string, error) {
	var updatedKeys []string
	if r.db == nil {
		for k := range settings {
			updatedKeys = append(updatedKeys, k)
		}
		return updatedKeys, nil
	}
	for key, value := range settings {
		_, _ = r.db.Exec(ctx, `
			INSERT INTO platform_settings (key, value) VALUES ($1, $2)
			ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = now()`,
			key, value)
		updatedKeys = append(updatedKeys, key)
	}
	return updatedKeys, nil
}

func (r *Repository) ToggleMaintenanceMode(ctx context.Context, enabled bool) error {
	if r.db == nil { return nil }
	modeValue := "false"
	if enabled { modeValue = "true" }
	_, err := r.db.Exec(ctx, `
		INSERT INTO platform_settings (key, value, description) VALUES ('maintenance_mode', $1, 'Platform maintenance mode')
		ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = now()`, modeValue)
	return err
}

func (r *Repository) UpdateSettingByKey(ctx context.Context, key, value, desc string) error {
	if r.db == nil { return nil }
	result, err := r.db.Exec(ctx, `
		UPDATE platform_settings SET value = $1, updated_at = now()
		WHERE key = $2`, value, key)
	if err != nil { return err }
	if result.RowsAffected() == 0 {
		_, _ = r.db.Exec(ctx, `
			INSERT INTO platform_settings (key, value, description) VALUES ($1, $2, $3)
			ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = now()`,
			key, value, desc)
	}
	return nil
}
