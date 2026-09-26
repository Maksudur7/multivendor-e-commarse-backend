// Package sms provides an SMS gateway client for Greenweb (Bangladesh).
// Docs: https://greenweb.com.bd/api-doc.php
//
// Usage:
//
//	client := sms.NewClient(cfg.SMS.APIKey, cfg.SMS.SenderID)
//	err := client.Send(ctx, "+8801XXXXXXXXX", "Your OTP is 482910")
package sms

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

// Client is a Greenweb SMS gateway client.
type Client struct {
	apiKey   string
	senderID string
	baseURL  string
	http     *http.Client
}

// NewClient constructs a Greenweb SMS client.
// apiKey is your Greenweb API token; senderID is the sender name/number (e.g. "8809XXXXXXXX").
func NewClient(apiKey, senderID string) *Client {
	return &Client{
		apiKey:   apiKey,
		senderID: senderID,
		baseURL:  "https://api.greenweb.com.bd/api.php",
		http:     &http.Client{Timeout: 10 * time.Second},
	}
}

// Send delivers an SMS to the given phone number.
// phone must be a Bangladeshi number (01XXXXXXXXX or +8801XXXXXXXXX).
func (c *Client) Send(ctx context.Context, phone, message string) error {
	if c.apiKey == "" || strings.Contains(c.apiKey, "sample") {
		log.Info().Str("to", phone).Msgf("📱 [SMS DEV MODE] Target: %s | Message: %s", phone, message)
		return nil
	}

	// Normalise phone: strip leading + and spaces.
	phone = strings.TrimSpace(strings.TrimPrefix(phone, "+"))

	params := url.Values{
		"token":   {c.apiKey},
		"to":      {phone},
		"message": {message},
	}
	if c.senderID != "" {
		params.Set("from", c.senderID)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		c.baseURL+"?"+params.Encode(), nil)
	if err != nil {
		return fmt.Errorf("sms: failed to build request: %w", err)
	}
	req.Header.Set("User-Agent", "ecom-backend/1.0")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("sms: request to Greenweb failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	bodyStr := strings.TrimSpace(string(body))

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("sms: Greenweb returned HTTP %d: %s", resp.StatusCode, bodyStr)
	}

	// Greenweb returns "success" or an error code in the body.
	if !strings.Contains(strings.ToLower(bodyStr), "success") &&
		!strings.HasPrefix(bodyStr, "1") {
		return fmt.Errorf("sms: Greenweb gateway error: %s", bodyStr)
	}

	log.Info().
		Str("to", phone).
		Str("status", bodyStr).
		Msg("sms: SMS dispatched successfully")

	return nil
}

// SendOTP is a convenience wrapper that formats and sends an OTP SMS.
func (c *Client) SendOTP(ctx context.Context, phone, otpCode string) error {
	msg := fmt.Sprintf(
		"আপনার OTP কোড: %s\nএটি ৫ মিনিটের জন্য বৈধ।\nএই কোড কারো সাথে শেয়ার করবেন না।",
		otpCode,
	)
	return c.Send(ctx, phone, msg)
}

// SendPasswordResetOTP sends a password-reset OTP SMS.
func (c *Client) SendPasswordResetOTP(ctx context.Context, phone, otpCode string) error {
	msg := fmt.Sprintf(
		"পাসওয়ার্ড রিসেট OTP: %s\nএটি ১৫ মিনিটের জন্য বৈধ।\nঅনুরোধ না করলে এই কোড উপেক্ষা করুন।",
		otpCode,
	)
	return c.Send(ctx, phone, msg)
}

// IsConfigured returns true if a live SMS API key is configured.
func (c *Client) IsConfigured() bool {
	return c.apiKey != "" && !strings.Contains(c.apiKey, "sample")
}

// IsPhone returns true if the target looks like a phone number (not an email).
func IsPhone(target string) bool {
	target = strings.TrimPrefix(strings.TrimSpace(target), "+")
	return len(target) >= 10 && !strings.Contains(target, "@")
}

// NormalizePhone normalizes phone numbers to standard E.164 format (e.g. +8801XXXXXXXXX).
func NormalizePhone(phone string) string {
	phone = strings.TrimSpace(phone)
	phone = strings.ReplaceAll(phone, " ", "")
	phone = strings.ReplaceAll(phone, "-", "")
	if strings.HasPrefix(phone, "01") && len(phone) == 11 {
		return "+88" + phone
	}
	if strings.HasPrefix(phone, "8801") && len(phone) == 13 {
		return "+" + phone
	}
	if !strings.HasPrefix(phone, "+") && len(phone) >= 10 && !strings.Contains(phone, "@") {
		return "+" + phone
	}
	return phone
}

