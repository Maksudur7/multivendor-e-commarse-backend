package eventbus

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// Event represents an in-process domain event.
type Event struct {
	ID        string
	Name      string
	Timestamp time.Time
	Payload   interface{}
}

// EventHandler is a function that handles an event.
type EventHandler func(ctx context.Context, event Event) error

// EventBus manages event subscriptions and publishing.
type EventBus struct {
	mu          sync.RWMutex
	subscribers map[string][]EventHandler
}

var globalBus *EventBus
var once sync.Once

// GetBus returns the singleton EventBus instance.
func GetBus() *EventBus {
	once.Do(func() {
		globalBus = &EventBus{
			subscribers: make(map[string][]EventHandler),
		}
	})
	return globalBus
}

// Subscribe registers a handler for a specific event name.
func (b *EventBus) Subscribe(eventName string, handler EventHandler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.subscribers[eventName] = append(b.subscribers[eventName], handler)
	log.Info().Msgf("📡 EventBus: Subscribed handler for event [%s]", eventName)
}

// Publish dispatches an event asynchronously to all subscribers.
func (b *EventBus) Publish(ctx context.Context, eventName string, payload interface{}) {
	b.mu.RLock()
	handlers, exists := b.subscribers[eventName]
	b.mu.RUnlock()

	event := Event{
		ID:        fmt.Sprintf("evt_%d", time.Now().UnixNano()),
		Name:      eventName,
		Timestamp: time.Now(),
		Payload:   payload,
	}

	if !exists || len(handlers) == 0 {
		log.Debug().Msgf("📡 EventBus: Published event [%s] with no subscribers", eventName)
		return
	}

	log.Info().Msgf("⚡ EventBus: Publishing event [%s] to %d subscribers", eventName, len(handlers))

	for _, handler := range handlers {
		h := handler
		go func() {
			defer func() {
				if r := recover(); r != nil {
					log.Error().Msgf("🔥 EventBus Panic Recovery in subscriber [%s]: %v", eventName, r)
				}
			}()
			if err := h(ctx, event); err != nil {
				log.Error().Err(err).Msgf("❌ EventBus Subscriber error for event [%s]", eventName)
			}
		}()
	}
}

// Common Domain Event Constants
const (
	EventUserRegistered           = "UserRegistered"
	EventUserBlocked              = "UserBlocked"
	EventOrderPlaced              = "OrderPlaced"
	EventPaymentConfirmed         = "PaymentConfirmed"
	EventPaymentFailed            = "PaymentFailed"
	EventOrderStatusChanged       = "OrderStatusChanged"
	EventOrderDelivered           = "OrderDelivered"
	EventOrderCancelled           = "OrderCancelled"
	EventReturnRequested          = "ReturnRequested"
	EventProductApproved          = "ProductApproved"
	EventProductRejected          = "ProductRejected"
	EventStockLow                 = "StockLow"
	EventOutOfStock               = "OutOfStock"
	EventStockRestored            = "StockRestored"
	EventEscrowReleased           = "EscrowReleased"
	EventPayoutRequested          = "PayoutRequested"
	EventPayoutApproved           = "PayoutApproved"
	EventConsignmentBooked        = "ConsignmentBooked"
	EventCourierStatusUpdated     = "CourierStatusUpdated"
	EventAffiliateConversionCreated = "AffiliateConversionCreated"
)
