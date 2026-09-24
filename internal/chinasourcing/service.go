package chinasourcing

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) ListBatches(ctx context.Context, userID string) ([]BatchItem, error) {
	if strings.TrimSpace(userID) == "" {
		return nil, errors.New("user ID is required")
	}
	return s.repo.ListBatches(ctx, userID)
}

func (s *Service) GetBatchDetail(ctx context.Context, batchID string) (*BatchDetail, error) {
	if strings.TrimSpace(batchID) == "" {
		return nil, errors.New("batch_id is required")
	}
	return s.repo.GetBatchDetail(ctx, batchID)
}

type LandedCostCalculation struct {
	Inputs  map[string]interface{} `json:"inputs"`
	Rates   map[string]interface{} `json:"rates"`
	PerUnit map[string]interface{} `json:"per_unit"`
	Total   map[string]interface{} `json:"total"`
}

func (s *Service) CalculateLandedCost(cnyCost float64, qty int, freightType string, customsDutyPct float64) (*LandedCostCalculation, error) {
	if cnyCost <= 0 {
		return nil, errors.New("cny_cost is required and must be > 0")
	}
	if qty <= 0 {
		qty = 1
	}
	if freightType == "" {
		freightType = "SEA"
	}
	if customsDutyPct <= 0 {
		customsDutyPct = 25.0
	}

	cnyToBdtRate := 16.8
	var freightPerUnit float64
	if strings.ToUpper(freightType) == "AIR" {
		freightPerUnit = 150.0
	} else {
		freightPerUnit = 85.0
	}

	bdtCostPerUnit := cnyCost * cnyToBdtRate
	customsDutyPerUnit := bdtCostPerUnit * (customsDutyPct / 100)
	landedCostPerUnit := bdtCostPerUnit + freightPerUnit + customsDutyPerUnit
	totalLandedCost := landedCostPerUnit * float64(qty)

	return &LandedCostCalculation{
		Inputs: map[string]interface{}{
			"cny_cost_per_unit": cnyCost, "quantity": qty,
			"freight_type": freightType, "customs_duty_pct": customsDutyPct,
		},
		Rates: map[string]interface{}{
			"cny_to_bdt_rate": cnyToBdtRate, "freight_per_unit_bdt": freightPerUnit,
		},
		PerUnit: map[string]interface{}{
			"bdt_cost":        bdtCostPerUnit,
			"customs_duty":    customsDutyPerUnit,
			"freight":         freightPerUnit,
			"landed_cost_bdt": landedCostPerUnit,
		},
		Total: map[string]interface{}{
			"quantity":            qty,
			"total_landed_bdt":    totalLandedCost,
			"avg_landed_per_unit": landedCostPerUnit,
		},
	}, nil
}

func (s *Service) ListCustomsDeclarations(ctx context.Context) ([]CustomsDeclarationItem, error) {
	return s.repo.ListCustomsDeclarations(ctx)
}

func (s *Service) CreateBatch(ctx context.Context, userID, freightType string) (string, string, error) {
	if strings.TrimSpace(userID) == "" {
		return "", "", errors.New("user ID is required")
	}
	if freightType == "" {
		freightType = "SEA"
	}
	batchCode := fmt.Sprintf("CN-%d-%s-%04d",
		time.Now().Year(),
		strings.ToUpper(freightType[:3]),
		time.Now().UnixNano()%10000,
	)
	batchID, err := s.repo.CreateBatch(ctx, batchCode, userID)
	return batchID, batchCode, err
}

func (s *Service) AddBatchItem(ctx context.Context, batchID, productName, skuCode string, quantity int, unitCostCNY float64) (string, error) {
	if strings.TrimSpace(batchID) == "" {
		return "", errors.New("batch_id is required")
	}
	if strings.TrimSpace(productName) == "" || quantity <= 0 {
		return "", errors.New("product_name and quantity > 0 are required")
	}
	return s.repo.AddBatchItem(ctx, batchID, productName, skuCode, quantity, unitCostCNY)
}

func (s *Service) UpdateBatchStatus(ctx context.Context, batchID, status string) error {
	if strings.TrimSpace(batchID) == "" {
		return errors.New("batch_id is required")
	}
	validStatuses := map[string]bool{
		"SOURCING": true, "ORDERED": true, "SHIPPED": true,
		"IN_TRANSIT_AIR": true, "IN_TRANSIT_SEA": true,
		"CUSTOMS": true, "WAREHOUSE": true, "DELIVERED": true,
		"COMPLETED": true, "CANCELLED": true,
	}
	if !validStatuses[status] {
		return errors.New("invalid status")
	}
	return s.repo.UpdateBatchStatus(ctx, batchID, status)
}

func (s *Service) UpdateBatchLandedCost(ctx context.Context, batchID string, cny, freight, duty, landed float64) error {
	if strings.TrimSpace(batchID) == "" {
		return errors.New("batch_id is required")
	}
	return s.repo.UpdateBatchLandedCost(ctx, batchID, cny, freight, duty, landed)
}

func (s *Service) UpdateBatchCustomsInfo(ctx context.Context, batchID string, customsData map[string]interface{}) error {
	if strings.TrimSpace(batchID) == "" {
		return errors.New("batch_id is required")
	}
	return s.repo.UpdateBatchCustomsInfo(ctx, batchID, customsData)
}
