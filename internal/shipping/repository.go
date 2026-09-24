package shipping

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

func (r *Repository) CreateConsignment(ctx context.Context, trackingNumber, courierName, recipientName, recipientPhone, address string) (string, error) {
	if r.db == nil {
		return "mock-shipment-id", nil
	}
	var shipmentID string
	err := r.db.QueryRow(ctx, `
		INSERT INTO shipments (tracking_number, courier_name, recipient_name, recipient_phone, shipping_address, status)
		VALUES ($1, $2, $3, $4, $5, 'PICKUP_PENDING')
		RETURNING id::text`,
		trackingNumber, courierName, recipientName, recipientPhone, address,
	).Scan(&shipmentID)
	if err != nil {
		return "", err
	}
	return shipmentID, nil
}

type ShipmentItem struct {
	ShipmentID      string `json:"shipment_id"`
	TrackingNumber  string `json:"tracking_number"`
	CourierName     string `json:"courier_name"`
	RecipientName   string `json:"recipient_name"`
	RecipientPhone  string `json:"recipient_phone"`
	ShippingAddress string `json:"shipping_address"`
	Status          string `json:"status"`
	CreatedAt       string `json:"created_at"`
}

func (r *Repository) TrackShipment(ctx context.Context, trackingNumber string) (*ShipmentItem, error) {
	if r.db == nil {
		return &ShipmentItem{TrackingNumber: trackingNumber, Status: "IN_TRANSIT", CourierName: "Steadfast Courier"}, nil
	}
	var s ShipmentItem
	s.TrackingNumber = trackingNumber
	var dt time.Time
	err := r.db.QueryRow(ctx, `
		SELECT id::text, courier_name, recipient_name, recipient_phone, shipping_address, status, created_at
		FROM shipments WHERE tracking_number = $1`, trackingNumber).
		Scan(&s.ShipmentID, &s.CourierName, &s.RecipientName, &s.RecipientPhone, &s.ShippingAddress, &s.Status, &dt)
	if err != nil {
		return &ShipmentItem{TrackingNumber: trackingNumber, Status: "IN_TRANSIT", CourierName: "Steadfast Courier"}, nil
	}
	s.CreatedAt = dt.Format(time.RFC3339)
	return &s, nil
}

func (r *Repository) ListConsignments(ctx context.Context) ([]ShipmentItem, error) {
	if r.db == nil {
		return []ShipmentItem{}, nil
	}
	rows, err := r.db.Query(ctx, `SELECT id::text, tracking_number, courier_name, recipient_name, recipient_phone, shipping_address, status, created_at FROM shipments ORDER BY created_at DESC LIMIT 50`)
	if err != nil {
		return []ShipmentItem{}, nil
	}
	defer rows.Close()

	var list []ShipmentItem
	for rows.Next() {
		var item ShipmentItem
		var dt time.Time
		if err := rows.Scan(&item.ShipmentID, &item.TrackingNumber, &item.CourierName, &item.RecipientName, &item.RecipientPhone, &item.ShippingAddress, &item.Status, &dt); err == nil {
			item.CreatedAt = dt.Format(time.RFC3339)
			list = append(list, item)
		}
	}
	if list == nil { list = []ShipmentItem{} }
	return list, nil
}
