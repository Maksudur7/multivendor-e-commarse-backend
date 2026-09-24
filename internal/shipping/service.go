package shipping

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

type ConsignmentResult struct {
	ShipmentID     string `json:"shipment_id"`
	SubOrderID     string `json:"sub_order_id"`
	CourierName    string `json:"courier_name"`
	ConsignmentID  string `json:"consignment_id"`
	TrackingNumber string `json:"tracking_number"`
	Status         string `json:"status"`
	WaybillURL     string `json:"waybill_url"`
}

func (s *Service) CreateConsignment(ctx context.Context, subOrderID, courier, recipientName, phone, address string) (*ConsignmentResult, error) {
	if courier == "" {
		courier = "STEADFAST"
	}
	if phone == "" {
		phone = "01700000000"
	}
	courierPrefix := courier
	if len(courierPrefix) > 3 {
		courierPrefix = courierPrefix[:3]
	}
	consignmentID := fmt.Sprintf("CSG-%s-%d", courierPrefix, time.Now().Unix()%100000)
	trackingNumber := fmt.Sprintf("TRK-%s-%d", courierPrefix, time.Now().UnixNano()/1e6)

	shipmentID, err := s.repo.CreateConsignment(ctx, trackingNumber, courier, recipientName, phone, address)
	if err != nil {
		return nil, err
	}

	return &ConsignmentResult{
		ShipmentID:     shipmentID,
		SubOrderID:     subOrderID,
		CourierName:    courier,
		ConsignmentID:  consignmentID,
		TrackingNumber: trackingNumber,
		Status:         "PICKUP_PENDING",
		WaybillURL:     fmt.Sprintf("https://shipping.platform.com/waybill/%s.pdf", trackingNumber),
	}, nil
}

func (s *Service) TrackShipment(ctx context.Context, trackingNumber string) (*ShipmentItem, error) {
	if strings.TrimSpace(trackingNumber) == "" {
		return nil, errors.New("tracking_number is required")
	}
	return s.repo.TrackShipment(ctx, trackingNumber)
}

func (s *Service) ListConsignments(ctx context.Context) ([]ShipmentItem, error) {
	return s.repo.ListConsignments(ctx)
}

type RateCalculationResult struct {
	DeliveryType string  `json:"delivery_type"`
	WeightKg     float64 `json:"weight_kg"`
	ShippingFee  float64 `json:"shipping_fee"`
}

func (s *Service) CalculateShippingRate(deliveryType string, weightKg float64) *RateCalculationResult {
	fee := 60.0
	if deliveryType == "OUTSIDE_DHAKA" {
		fee = 120.0
	}
	if weightKg > 1.0 {
		fee += (weightKg - 1.0) * 20.0
	}
	return &RateCalculationResult{
		DeliveryType: deliveryType,
		WeightKg:     weightKg,
		ShippingFee:  fee,
	}
}
