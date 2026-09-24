package dispute

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

type DisputeItem struct {
	TicketID     string `json:"ticket_id"`
	TicketNumber string `json:"ticket_number"`
	CustomerID   string `json:"customer_id,omitempty"`
	Reason       string `json:"reason"`
	Description  string `json:"description"`
	Status       string `json:"status"`
	CreatedAt    string `json:"created_at"`
}

func (r *Repository) OpenDispute(ctx context.Context, customerID, reason, description string) (string, string, error) {
	ticketNo := fmt.Sprintf("TK-%d", time.Now().UnixNano()/1e6)
	if r.db == nil {
		return "", ticketNo, nil
	}
	var ticketID string
	err := r.db.QueryRow(ctx, `
		INSERT INTO disputes (ticket_number, customer_id, reason, description, status)
		VALUES ($1, $2::uuid, $3, $4, 'OPEN')
		RETURNING id::text`,
		ticketNo, customerID, reason, description,
	).Scan(&ticketID)
	if err != nil {
		return "", ticketNo, err
	}
	return ticketID, ticketNo, nil
}

func (r *Repository) GetTicketsByCustomer(ctx context.Context, customerID string) ([]DisputeItem, error) {
	if r.db == nil {
		return []DisputeItem{}, nil
	}
	rows, err := r.db.Query(ctx, `
		SELECT id::text, ticket_number, reason, description, status, created_at 
		FROM disputes 
		WHERE customer_id::text = $1 
		ORDER BY created_at DESC`, customerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tickets := []DisputeItem{}
	for rows.Next() {
		var id, tNo, reason, desc, status string
		var dt time.Time
		rows.Scan(&id, &tNo, &reason, &desc, &status, &dt)
		tickets = append(tickets, DisputeItem{
			TicketID:     id,
			TicketNumber: tNo,
			Reason:       reason,
			Description:  desc,
			Status:       status,
			CreatedAt:    dt.Format(time.RFC3339),
		})
	}
	return tickets, nil
}

func (r *Repository) GetAllDisputes(ctx context.Context) ([]DisputeItem, error) {
	if r.db == nil {
		return []DisputeItem{}, nil
	}
	rows, err := r.db.Query(ctx, `
		SELECT id::text, ticket_number, customer_id::text, reason, description, status, created_at 
		FROM disputes 
		ORDER BY created_at DESC LIMIT 50`)
	if err != nil {
		return []DisputeItem{}, nil
	}
	defer rows.Close()

	tickets := []DisputeItem{}
	for rows.Next() {
		var id, tNo, custID, reason, desc, status string
		var dt time.Time
		rows.Scan(&id, &tNo, &custID, &reason, &desc, &status, &dt)
		tickets = append(tickets, DisputeItem{
			TicketID:     id,
			TicketNumber: tNo,
			CustomerID:   custID,
			Reason:       reason,
			Description:  desc,
			Status:       status,
			CreatedAt:    dt.Format(time.RFC3339),
		})
	}
	return tickets, nil
}

func (r *Repository) GetDisputeByID(ctx context.Context, id string) (*DisputeItem, error) {
	if r.db == nil {
		return &DisputeItem{TicketID: id, Status: "OPEN"}, nil
	}
	var tNo, custID, reason, desc, status string
	var dt time.Time
	err := r.db.QueryRow(ctx, `
		SELECT ticket_number, customer_id::text, reason, description, status, created_at 
		FROM disputes 
		WHERE id::text = $1 OR ticket_number = $1`, id).
		Scan(&tNo, &custID, &reason, &desc, &status, &dt)
	if err != nil {
		return &DisputeItem{TicketID: id, Status: "OPEN"}, nil
	}
	return &DisputeItem{
		TicketID:     id,
		TicketNumber: tNo,
		CustomerID:   custID,
		Reason:       reason,
		Description:  desc,
		Status:       status,
		CreatedAt:    dt.Format(time.RFC3339),
	}, nil
}

func (r *Repository) ResolveDispute(ctx context.Context, ticketID, status, resolution string) error {
	if r.db == nil {
		return nil
	}
	_, err := r.db.Exec(ctx, "UPDATE disputes SET status = $1, admin_resolution = $2 WHERE id::text = $3 OR ticket_number = $3", status, resolution, ticketID)
	return err
}
