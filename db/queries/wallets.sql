-- ════════════════════════════════════════════════════════════
-- Wallet Domain SQL Queries
-- ════════════════════════════════════════════════════════════

-- name: GetWalletByOwner :one
SELECT id, owner_type, owner_id, available_balance, pending_escrow_balance, locked_balance, total_withdrawn, currency, updated_at
FROM wallets
WHERE owner_type = $1 AND owner_id = $2 LIMIT 1;

-- name: CreateWalletLedgerEntry :one
INSERT INTO wallet_ledger (
  wallet_id, transaction_type, amount, reference_type, reference_id, description, balance_after
) VALUES (
  $1, $2, $3, $4, $5, $6, $7
)
RETURNING id, wallet_id, transaction_type, amount, balance_after, created_at;

-- name: CreateWithdrawalRequest :one
INSERT INTO withdrawal_requests (
  wallet_id, owner_type, owner_id, amount, payment_method, account_details, status
) VALUES (
  $1, $2, $3, $4, $5, $6, 'PENDING'
)
RETURNING id, amount, payment_method, status, created_at;

-- name: ProcessWithdrawalAdmin :one
UPDATE withdrawal_requests
SET status = $2, transaction_proof_ref = $3, processed_at = NOW(), updated_at = NOW()
WHERE id = $1
RETURNING id, amount, status, transaction_proof_ref, processed_at;
