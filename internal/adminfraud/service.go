package adminfraud

import (
	"context"
	"errors"
	"net"
	"strings"
	"time"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) ListRiskProfiles(ctx context.Context, riskLevel string) ([]RiskProfileItem, error) {
	return s.repo.ListRiskProfiles(ctx, riskLevel)
}

func (s *Service) ListBlacklists(ctx context.Context) ([]BlacklistPhoneItem, error) {
	return s.repo.ListBlacklists(ctx)
}

func (s *Service) ListIPBlocks(ctx context.Context) ([]IPBlockItem, error) {
	return s.repo.ListIPBlocks(ctx)
}

func (s *Service) BlacklistPhone(ctx context.Context, phone, reason, note string) (string, error) {
	if strings.TrimSpace(phone) == "" || strings.TrimSpace(reason) == "" {
		return "", errors.New("phone and reason are required")
	}
	validReasons := map[string]bool{"FRAUD": true, "ABUSIVE": true, "FAKE_ORDERS": true, "EXCESSIVE_RETURNS": true, "OTHER": true}
	if !validReasons[reason] {
		return "", errors.New("reason must be FRAUD, ABUSIVE, FAKE_ORDERS, EXCESSIVE_RETURNS, or OTHER")
	}
	return s.repo.BlacklistPhone(ctx, phone, reason, note)
}

func (s *Service) BlockIP(ctx context.Context, ip, reason string, durationHours int) (string, time.Time, error) {
	ip = strings.TrimSpace(ip)
	if ip == "" {
		return "", time.Time{}, errors.New("ip_address is required")
	}
	if net.ParseIP(ip) == nil {
		return "", time.Time{}, errors.New("invalid IP address format")
	}
	if durationHours <= 0 {
		durationHours = 24
	}
	blockedUntil := time.Now().Add(time.Duration(durationHours) * time.Hour)
	blockID, err := s.repo.BlockIP(ctx, ip, reason, blockedUntil)
	return blockID, blockedUntil, err
}

func (s *Service) RecalculateRisk(ctx context.Context, userID string) error {
	if strings.TrimSpace(userID) == "" {
		return errors.New("user_id is required")
	}
	return s.repo.RecalculateRisk(ctx, userID)
}

func (s *Service) UpdateRiskProfile(ctx context.Context, profileID, riskLevel, note string, score float64) error {
	if strings.TrimSpace(profileID) == "" {
		return errors.New("profile_id is required")
	}
	validLevels := map[string]bool{"LOW": true, "MEDIUM": true, "HIGH": true, "VERY_HIGH": true}
	if riskLevel != "" && !validLevels[riskLevel] {
		return errors.New("risk_level must be LOW, MEDIUM, HIGH, or VERY_HIGH")
	}
	return s.repo.UpdateRiskProfile(ctx, profileID, riskLevel, note, score)
}

func (s *Service) UnblockIP(ctx context.Context, blockID string) error {
	if strings.TrimSpace(blockID) == "" {
		return errors.New("block_id is required")
	}
	return s.repo.UnblockIP(ctx, blockID)
}
