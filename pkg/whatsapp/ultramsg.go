package whatsapp

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

// Client is a WhatsApp API gateway client supporting UltraMsg.
type Client struct {
	instanceID string
	token      string
	baseURL    string
	http       *http.Client
}

// NewClient constructs a WhatsApp UltraMsg client.
func NewClient(instanceID, token string) *Client {
	return &Client{
		instanceID: strings.TrimSpace(instanceID),
		token:      strings.TrimSpace(token),
		baseURL:    "https://api.ultramsg.com",
		http:       &http.Client{Timeout: 10 * time.Second},
	}
}

// IsConfigured returns true if UltraMsg instance ID and token are set.
func (c *Client) IsConfigured() bool {
	return c.instanceID != "" && c.token != "" && !strings.Contains(c.token, "sample")
}

// Send delivers a WhatsApp text message using UltraMsg API.
// phone should be in international format (e.g., "+88017XXXXXXXX" or "88017XXXXXXXX").
func (c *Client) Send(ctx context.Context, phone, message string) error {
	if !c.IsConfigured() {
		log.Info().Str("to", phone).Msgf("💬 [WHATSAPP DEV MODE] Target: %s | Message: %s", phone, message)
		return nil
	}

	// Normalise phone: strip leading +, spaces, hyphens
	phone = strings.TrimSpace(phone)
	phone = strings.ReplaceAll(phone, " ", "")
	phone = strings.ReplaceAll(phone, "-", "")

	apiURL := fmt.Sprintf("%s/%s/messages/chat", c.baseURL, c.instanceID)

	data := url.Values{
		"token": {c.token},
		"to":    {phone},
		"body":  {message},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, strings.NewReader(data.Encode()))
	if err != nil {
		return fmt.Errorf("whatsapp: failed to build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "ecom-backend/1.0")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("whatsapp: request to UltraMsg failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	bodyStr := strings.TrimSpace(string(body))

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("whatsapp: UltraMsg returned HTTP %d: %s", resp.StatusCode, bodyStr)
	}

	log.Info().
		Str("to", phone).
		Str("status", bodyStr).
		Msg("whatsapp: UltraMsg message dispatched successfully")

	return nil
}

// SendOTP formats and dispatches an OTP code via WhatsApp.
func (c *Client) SendOTP(ctx context.Context, phone, otpCode string) error {
	msg := fmt.Sprintf(
		"🔐 *আপনার WhatsApp OTP কোড: %s*\n\nএটি ৫ মিনিটের জন্য বৈধ।\nএই কোডটি কারো সাথে শেয়ার করবেন না।",
		otpCode,
	)
	return c.Send(ctx, phone, msg)
}

// SendPasswordResetOTP formats and dispatches a password reset OTP code via WhatsApp.
func (c *Client) SendPasswordResetOTP(ctx context.Context, phone, otpCode string) error {
	msg := fmt.Sprintf(
		"🔑 *পাসওয়ার্ড রিসেট OTP: %s*\n\nএটি ১৫ মিনিটের জন্য বৈধ।\nঅনুরোধ না করলে এই মেসেজটি উপেক্ষা করুন।",
		otpCode,
	)
	return c.Send(ctx, phone, msg)
}
