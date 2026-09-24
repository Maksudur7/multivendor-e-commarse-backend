package user

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// --- Pure helpers ---

func (s *Service) HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

func (s *Service) CheckPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

func (s *Service) IsValidKYCDocumentType(docType string) bool {
	validTypes := map[string]bool{
		"NID": true, "PASSPORT": true, "DRIVING_LICENSE": true, "TRADE_LICENSE": true,
	}
	return validTypes[docType]
}

func (s *Service) NormalizeAddressInput(name, fullName, phone, phoneAlt, addr1, addr, div, city, dist string) (string, string, string, string, string) {
	if name == "" {
		name = fullName
	}
	if phone == "" {
		phone = phoneAlt
	}
	if addr1 == "" {
		addr1 = addr
	}
	if div == "" {
		div = city
	}
	if dist == "" {
		dist = div
	}
	return name, phone, addr1, div, dist
}

// --- Profile operations ---

func (s *Service) GetProfile(ctx context.Context, userID string) (*UserProfile, error) {
	return s.repo.GetProfileByID(ctx, userID)
}

func (s *Service) UpdateProfile(ctx context.Context, userID, fullName, avatarURL, phone string) error {
	return s.repo.UpdateProfile(ctx, userID, fullName, avatarURL, phone)
}

// --- Password operations ---

func (s *Service) ChangePassword(ctx context.Context, userID, currentPassword, newPassword string) error {
	storedHash, err := s.repo.GetUserPasswordHash(ctx, userID)
	if err != nil || storedHash == "" {
		return fmt.Errorf("user not found")
	}
	if !s.CheckPassword(currentPassword, storedHash) {
		return fmt.Errorf("current password is incorrect")
	}
	newHash, err := s.HashPassword(newPassword)
	if err != nil {
		return fmt.Errorf("failed to secure new password: %w", err)
	}
	return s.repo.UpdatePasswordAndRevokeSessions(ctx, userID, newHash)
}

// --- Address operations ---

func (s *Service) GetAddresses(ctx context.Context, userID string) ([]UserAddress, error) {
	return s.repo.GetAddresses(ctx, userID)
}

type CreateAddressInput struct {
	UserID        string
	RecipientName string
	RecipientPhone string
	AddressLine1  string
	AddressLine2  string
	Division      string
	District      string
	Upazila       string
	PostalCode    string
	Label         string
	IsDefault     bool
}

func (s *Service) CreateAddress(ctx context.Context, input CreateAddressInput) (string, error) {
	return s.repo.CreateAddress(ctx,
		input.UserID, input.RecipientName, input.RecipientPhone,
		input.AddressLine1, input.AddressLine2,
		input.Division, input.District,
		input.Upazila, input.PostalCode,
		input.Label, input.IsDefault,
	)
}

func (s *Service) UpdateAddress(ctx context.Context, addressID, userID, recName, recPhone, line1, label string) error {
	return s.repo.UpdateAddress(ctx, addressID, userID, recName, recPhone, line1, label)
}

func (s *Service) SetDefaultAddress(ctx context.Context, addressID, userID string) (int64, error) {
	return s.repo.SetDefaultAddress(ctx, addressID, userID)
}

func (s *Service) DeleteAddress(ctx context.Context, addressID, userID string) (int64, error) {
	return s.repo.DeleteAddress(ctx, addressID, userID)
}

// --- KYC operations ---

func (s *Service) SubmitKYC(ctx context.Context, userID, documentType, documentNumber, frontImageURL, backImageURL, selfieImageURL string) (string, error) {
	kycID := uuid.New().String()
	err := s.repo.SaveKYC(ctx, kycID, userID, documentType, documentNumber, frontImageURL, backImageURL, selfieImageURL)
	if err != nil {
		return "", fmt.Errorf("failed to submit KYC: %w", err)
	}
	return kycID, nil
}

func (s *Service) ReviewKYC(ctx context.Context, targetUserID, status, rejectionReason string) (int64, error) {
	return s.repo.ReviewKYC(ctx, targetUserID, status, rejectionReason)
}
