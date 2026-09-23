-- ════════════════════════════════════════════════════════════
-- Dispute & Ticket Domain SQL Queries
-- ════════════════════════════════════════════════════════════

-- name: CreateDisputeTicket :one
INSERT INTO dispute_tickets (
  sub_order_id, customer_id, vendor_id, ticket_number, reason, description, evidence_urls, status
) VALUES (
  $1, $2, $3, $4, $5, $6, $7, 'OPEN'
)
RETURNING id, ticket_number, sub_order_id, status, created_at;

-- name: AddTicketMessage :one
INSERT INTO ticket_messages (
  ticket_id, sender_id, sender_type, message, attachments
) VALUES (
  $1, $2, $3, $4, $5
)
RETURNING id, ticket_id, sender_id, message, created_at;

-- name: ResolveDispute :one
UPDATE dispute_tickets
SET status = $2, admin_resolution = $3, resolved_at = NOW(), updated_at = NOW()
WHERE id = $1
RETURNING id, ticket_number, status, admin_resolution, resolved_at;
