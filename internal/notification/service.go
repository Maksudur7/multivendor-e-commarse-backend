package notification

import (
	"context"
	"errors"
	"strings"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) GetNotifications(ctx context.Context, userID string) ([]NotificationItem, error) {
	if strings.TrimSpace(userID) == "" {
		return nil, errors.New("user ID is required")
	}
	return s.repo.GetUserNotifications(ctx, userID)
}

func (s *Service) SendSMS(phone, message string) (string, error) {
	if strings.TrimSpace(phone) == "" || strings.TrimSpace(message) == "" {
		return "", errors.New("phone_number and message are required")
	}
	return "DELIVERED", nil
}
