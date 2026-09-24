package payment

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) GetPaymentStatus(ctx context.Context, txID string) (*PaymentStatusItem, error) {
	return s.repo.GetPaymentStatus(ctx, txID)
}

func (s *Service) InitiatePayment(ctx context.Context, masterOrderID, gateway string, amount float64) (string, map[string]interface{}, error) {
	if gateway == "" {
		gateway = "COD"
	}
	txRef := fmt.Sprintf("TXN-%d", time.Now().UnixNano()/1e6)

	_ = s.repo.RecordPayment(ctx, masterOrderID, txRef, gateway, amount)

	var res map[string]interface{}
	switch gateway {
	case "BKASH":
		res = map[string]interface{}{
			"payment_gateway":       "BKASH",
			"transaction_reference": txRef,
			"bkash_payment_url":     fmt.Sprintf("https://checkout.sandbox.bka.sh/v1.2.0-beta/pay/checkout?paymentID=BK-%s", uuid.New().String()[:8]),
		}
	case "SSLCOMMERZ":
		res = map[string]interface{}{
			"payment_gateway":       "SSLCOMMERZ",
			"transaction_reference": txRef,
			"ssl_redirect_url":      fmt.Sprintf("https://sandbox.sslcommerz.com/gwprocess/v4/gw.php?Q=PAY&sessionkey=%s", uuid.New().String()),
		}
	case "NAGAD":
		res = map[string]interface{}{
			"payment_gateway":       "NAGAD",
			"transaction_reference": txRef,
			"nagad_redirect_url":    fmt.Sprintf("https://api.mynagad.com/pay/%s", txRef),
		}
	default:
		res = map[string]interface{}{
			"payment_gateway":       "COD",
			"transaction_reference": txRef,
			"payment_status":        "PENDING_COD_COLLECTION",
		}
	}
	return txRef, res, nil
}

func (s *Service) RequestRefund(ctx context.Context, customerID string, amount float64, reason string) error {
	return s.repo.RequestRefund(ctx, customerID, amount, reason)
}
