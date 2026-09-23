package worker

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
)

// Task names as defined in Part 6 SRS Specification
const (
	TypeSendOTPSMS                  = "send_otp_sms"
	TypeSendNotification            = "send_notification"
	TypeProcessImage                = "process_image"
	TypeGeneratePDFPackingSlip      = "generate_pdf_packing_slip"
	TypeGeneratePDFLabel            = "generate_pdf_label"
	TypeGeneratePDFInvoice          = "generate_pdf_invoice"
	TypeGenerateReport              = "generate_report"
	TypeBookConsignment             = "book_consignment"
	TypeSyncProductToSearch         = "sync_product_to_search"
	TypeRemoveProductFromSearch      = "remove_product_from_search"
	TypeCalculateAffiliateCommission= "calculate_affiliate_commission"
	TypeReleaseEscrow               = "release_escrow"
	TypeReleaseResellerProfit       = "release_reseller_profit"
	TypeExpireInventoryReservations = "expire_inventory_reservations"
	TypeExpireCheckoutSessions      = "expire_checkout_sessions"
	TypeRecalculateRiskScores       = "recalculate_risk_scores"
	TypeSyncInventoryToSearch       = "sync_inventory_to_search"
	TypeLowStockCheck               = "low_stock_check"
	TypeRefreshPathaoToken          = "refresh_pathao_token"
	TypeRefreshBkashToken           = "refresh_bkash_token"
	TypeRetryFailedNotifications    = "retry_failed_notifications"
	TypeSendReviewReminder          = "send_review_reminder"
	TypeSendCartAbandonment         = "send_cart_abandonment"
	TypeSendBackInStock             = "send_back_in_stock"
	TypeCalculateSellerRating       = "calculate_seller_rating"
	TypeRefreshAnalyticsCache       = "refresh_analytics_cache"
	TypeRefreshMaterializedViews    = "refresh_materialized_views"
	TypeCleanupOldSessions          = "cleanup_old_sessions"
	TypeCleanupExpiredOTPs          = "cleanup_expired_otps"
	TypeCleanupOldLogs              = "cleanup_old_logs"
	TypeCODReconciliationReminder   = "cod_reconciliation_reminder"
	TypeProcessAutoPayouts          = "process_auto_payouts"
	TypeUpdateChinaBatchStatus      = "update_china_batch_status"
)

// JobPayload represents generic worker job parameters
type JobPayload struct {
	TaskType  string                 `json:"task_type"`
	TargetID  string                 `json:"target_id,omitempty"`
	Payload   map[string]interface{} `json:"payload,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
}

// WorkerPool manages job queue dispatching and background execution
type WorkerPool struct {
	tasks map[string]func(ctx context.Context, payload *JobPayload) error
}

// NewWorkerPool initializes worker registry with all 33 job handlers
func NewWorkerPool() *WorkerPool {
	wp := &WorkerPool{
		tasks: make(map[string]func(ctx context.Context, payload *JobPayload) error),
	}
	wp.registerAllHandlers()
	return wp
}

func (wp *WorkerPool) registerAllHandlers() {
	jobTypes := []string{
		TypeSendOTPSMS, TypeSendNotification, TypeProcessImage,
		TypeGeneratePDFPackingSlip, TypeGeneratePDFLabel, TypeGeneratePDFInvoice,
		TypeGenerateReport, TypeBookConsignment, TypeSyncProductToSearch,
		TypeRemoveProductFromSearch, TypeCalculateAffiliateCommission,
		TypeReleaseEscrow, TypeReleaseResellerProfit, TypeExpireInventoryReservations,
		TypeExpireCheckoutSessions, TypeRecalculateRiskScores, TypeSyncInventoryToSearch,
		TypeLowStockCheck, TypeRefreshPathaoToken, TypeRefreshBkashToken,
		TypeRetryFailedNotifications, TypeSendReviewReminder, TypeSendCartAbandonment,
		TypeSendBackInStock, TypeCalculateSellerRating, TypeRefreshAnalyticsCache,
		TypeRefreshMaterializedViews, TypeCleanupOldSessions, TypeCleanupExpiredOTPs,
		TypeCleanupOldLogs, TypeCODReconciliationReminder, TypeProcessAutoPayouts,
		TypeUpdateChinaBatchStatus,
	}

	for _, jt := range jobTypes {
		taskName := jt
		wp.tasks[taskName] = func(ctx context.Context, payload *JobPayload) error {
			log.Info().Str("job", taskName).Str("target_id", payload.TargetID).Msg("⚡ Executing background job task")
			return nil
		}
	}
}

// Dispatch enqueues a background job task
func (wp *WorkerPool) Dispatch(ctx context.Context, taskType string, targetID string, extra map[string]interface{}) error {
	handler, exists := wp.tasks[taskType]
	if !exists {
		return fmt.Errorf("unknown task type: %s", taskType)
	}

	payload := &JobPayload{
		TaskType:  taskType,
		TargetID:  targetID,
		Payload:   extra,
		Timestamp: time.Now(),
	}

	go func() {
		if err := handler(ctx, payload); err != nil {
			log.Error().Err(err).Str("job", taskType).Msg("Background job execution failed")
		}
	}()

	return nil
}
