package chinasourcing

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

type BatchItem struct {
	BatchID       string  `json:"batch_id"`
	BatchCode     string  `json:"batch_code"`
	Status        string  `json:"status"`
	CNYCost       float64 `json:"cny_cost"`
	FreightCost   float64 `json:"freight_cost"`
	LandedCostBDT float64 `json:"landed_cost_bdt"`
	CreatedAt     string  `json:"created_at"`
	UpdatedAt     string  `json:"updated_at"`
}

func (r *Repository) ListBatches(ctx context.Context, userID string) ([]BatchItem, error) {
	if r.db == nil {
		return []BatchItem{}, nil
	}
	rows, err := r.db.Query(ctx, `
		SELECT id::text, batch_code, status,
		       COALESCE(cny_cost,0), COALESCE(freight_cost,0),
		       COALESCE(landed_cost_bdt,0), created_at, updated_at
		FROM china_sourcing_batches
		WHERE user_id::text = $1
		ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []BatchItem
	for rows.Next() {
		var item BatchItem
		var dt1, dt2 time.Time
		if err := rows.Scan(&item.BatchID, &item.BatchCode, &item.Status, &item.CNYCost, &item.FreightCost, &item.LandedCostBDT, &dt1, &dt2); err == nil {
			item.CreatedAt = dt1.Format(time.RFC3339)
			item.UpdatedAt = dt2.Format(time.RFC3339)
			list = append(list, item)
		}
	}
	if list == nil {
		list = []BatchItem{}
	}
	return list, nil
}

type BatchDetail struct {
	BatchID        string                   `json:"batch_id"`
	BatchCode      string                   `json:"batch_code"`
	Status         string                   `json:"status"`
	CNYCost        float64                  `json:"cny_cost"`
	FreightCost    float64                  `json:"freight_cost"`
	CustomsDutyPct float64                  `json:"customs_duty_pct"`
	LandedCostBDT  float64                  `json:"landed_cost_bdt"`
	CustomsInfo    interface{}              `json:"customs_info"`
	Items          []map[string]interface{} `json:"items"`
	CreatedAt      string                   `json:"created_at"`
	UpdatedAt      string                   `json:"updated_at"`
}

func (r *Repository) GetBatchDetail(ctx context.Context, batchID string) (*BatchDetail, error) {
	if r.db == nil {
		return &BatchDetail{BatchID: batchID, Status: "SOURCING"}, nil
	}
	var b BatchDetail
	b.BatchID = batchID
	var dt1, dt2 time.Time
	err := r.db.QueryRow(ctx, `
		SELECT id::text, batch_code, status,
		       COALESCE(cny_cost,0), COALESCE(freight_cost,0),
		       COALESCE(customs_duty_pct,0), COALESCE(landed_cost_bdt,0),
		       customs_info, created_at, updated_at
		FROM china_sourcing_batches WHERE id::text = $1`, batchID).
		Scan(&b.BatchID, &b.BatchCode, &b.Status, &b.CNYCost, &b.FreightCost, &b.CustomsDutyPct, &b.LandedCostBDT, &b.CustomsInfo, &dt1, &dt2)
	if err != nil {
		return nil, err
	}
	b.CreatedAt = dt1.Format(time.RFC3339)
	b.UpdatedAt = dt2.Format(time.RFC3339)

	itemRows, _ := r.db.Query(ctx, `
		SELECT id::text, COALESCE(product_name,''), COALESCE(sku_code,''), quantity, COALESCE(unit_cost_cny,0)
		FROM china_sourcing_batch_items WHERE batch_id::text = $1`, batchID)
	items := []map[string]interface{}{}
	if itemRows != nil {
		defer itemRows.Close()
		for itemRows.Next() {
			var iID, name, sku string
			var qty int
			var unitCost float64
			if err := itemRows.Scan(&iID, &name, &sku, &qty, &unitCost); err == nil {
				items = append(items, map[string]interface{}{
					"item_id": iID, "product_name": name, "sku_code": sku,
					"quantity": qty, "unit_cost_cny": unitCost, "subtotal_cny": float64(qty) * unitCost,
				})
			}
		}
	}
	b.Items = items
	return &b, nil
}

type CustomsDeclarationItem struct {
	BatchID     string      `json:"batch_id"`
	BatchCode   string      `json:"batch_code"`
	Status      string      `json:"status"`
	CustomsInfo interface{} `json:"customs_info"`
	UpdatedAt   string      `json:"updated_at"`
}

func (r *Repository) ListCustomsDeclarations(ctx context.Context) ([]CustomsDeclarationItem, error) {
	if r.db == nil {
		return []CustomsDeclarationItem{}, nil
	}
	rows, err := r.db.Query(ctx, `
		SELECT id::text, batch_code, status, customs_info, updated_at
		FROM china_sourcing_batches
		WHERE customs_info != '{}'::jsonb
		ORDER BY updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []CustomsDeclarationItem
	for rows.Next() {
		var item CustomsDeclarationItem
		var dt time.Time
		if err := rows.Scan(&item.BatchID, &item.BatchCode, &item.Status, &item.CustomsInfo, &dt); err == nil {
			item.UpdatedAt = dt.Format(time.RFC3339)
			list = append(list, item)
		}
	}
	if list == nil {
		list = []CustomsDeclarationItem{}
	}
	return list, nil
}

func (r *Repository) CreateBatch(ctx context.Context, batchCode, userID string) (string, error) {
	if r.db == nil {
		return "mock-batch-id", nil
	}
	var batchID string
	err := r.db.QueryRow(ctx, `
		INSERT INTO china_sourcing_batches (batch_code, user_id, status)
		VALUES ($1, $2::uuid, 'SOURCING') RETURNING id::text`,
		batchCode, userID,
	).Scan(&batchID)
	if err != nil {
		uniqueCode := fmt.Sprintf("%s-%d", batchCode, time.Now().UnixNano()%10000)
		err = r.db.QueryRow(ctx, `
			INSERT INTO china_sourcing_batches (batch_code, user_id, status)
			VALUES ($1, $2::uuid, 'SOURCING') RETURNING id::text`,
			uniqueCode, userID,
		).Scan(&batchID)
	}
	if err != nil {
		return "", err
	}
	return batchID, nil
}

func (r *Repository) AddBatchItem(ctx context.Context, batchID, productName, skuCode string, quantity int, unitCostCNY float64) (string, error) {
	if r.db == nil {
		return "mock-item-id", nil
	}
	var itemID string
	err := r.db.QueryRow(ctx, `
		INSERT INTO china_sourcing_batch_items (batch_id, product_name, sku_code, quantity, unit_cost_cny)
		VALUES ($1::uuid, $2, $3, $4, $5) RETURNING id::text`,
		batchID, productName, skuCode, quantity, unitCostCNY,
	).Scan(&itemID)
	if err != nil {
		return "", err
	}
	return itemID, nil
}

func (r *Repository) UpdateBatchStatus(ctx context.Context, batchID, status string) error {
	if r.db == nil {
		return nil
	}
	res, err := r.db.Exec(ctx, "UPDATE china_sourcing_batches SET status = $1, updated_at = now() WHERE id::text = $2", status, batchID)
	if err != nil {
		return err
	}
	if res.RowsAffected() == 0 {
		return fmt.Errorf("batch not found")
	}
	return nil
}

func (r *Repository) UpdateBatchLandedCost(ctx context.Context, batchID string, cny, freight, duty, landed float64) error {
	if r.db == nil {
		return nil
	}
	res, err := r.db.Exec(ctx, `
		UPDATE china_sourcing_batches
		SET cny_cost = $1, freight_cost = $2, customs_duty_pct = $3, landed_cost_bdt = $4, updated_at = now()
		WHERE id::text = $5`, cny, freight, duty, landed, batchID)
	if err != nil {
		return err
	}
	if res.RowsAffected() == 0 {
		return fmt.Errorf("batch not found")
	}
	return nil
}

func (r *Repository) UpdateBatchCustomsInfo(ctx context.Context, batchID string, customsData map[string]interface{}) error {
	if r.db == nil {
		return nil
	}
	res, err := r.db.Exec(ctx, `
		UPDATE china_sourcing_batches SET customs_info = $1::jsonb, updated_at = now() WHERE id::text = $2`,
		customsData, batchID)
	if err != nil {
		return err
	}
	if res.RowsAffected() == 0 {
		return fmt.Errorf("batch not found")
	}
	return nil
}
