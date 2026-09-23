-- ════════════════════════════════════════════════════════════
-- Notification Domain SQL Queries
-- ════════════════════════════════════════════════════════════

-- name: CreateNotificationLog :one
INSERT INTO notification_logs (
  user_id, channel, recipient, subject, content, status
) VALUES (
  $1, $2, $3, $4, $5, 'SENT'
)
RETURNING id, channel, recipient, status, created_at;

-- name: GetUserNotifications :many
SELECT id, channel, subject, content, is_read, created_at
FROM notification_logs
WHERE user_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;
