// Package email provides an SMTP & HTTP REST API email client for transactional emails.
// Supports Resend API, SendGrid API, Brevo API (over HTTPS Port 443) and SMTP (Ports 587/465).
package email

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net"
	"net/http"
	"net/smtp"
	"strings"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/yourusername/ecom-backend/config"
)

// Client is an email client supporting HTTP REST API & SMTP.
type Client struct {
	host     string
	port     int
	username string
	password string
	from     string
	apiKey   string
	provider string
	http     *http.Client
}

// NewClient constructs an email client from config.
func NewClient(cfg config.EmailConfig) *Client {
	cleanPass := strings.TrimSpace(strings.ReplaceAll(cfg.Password, " ", ""))
	return &Client{
		host:     strings.TrimSpace(cfg.SMTPHost),
		port:     cfg.SMTPPort,
		username: strings.TrimSpace(cfg.Username),
		password: cleanPass,
		from:     strings.TrimSpace(cfg.From),
		apiKey:   strings.TrimSpace(cfg.APIKey),
		provider: strings.ToLower(strings.TrimSpace(cfg.Provider)),
		http:     &http.Client{Timeout: 15 * time.Second},
	}
}

// IsConfigured returns true if email credentials (API Key or SMTP) are set.
func (c *Client) IsConfigured() bool {
	return c.apiKey != "" || (c.host != "" && c.username != "" && c.password != "")
}

// send is the core email dispatcher with HTTP API priority & SMTP fallback.
func (c *Client) send(ctx context.Context, to, subject, htmlBody string) error {
	if !c.IsConfigured() {
		log.Warn().Str("to", to).Msg("email: Email not configured — missing EMAIL_API_KEY, RESEND_API_KEY, or SMTP credentials")
		return fmt.Errorf("email: credentials missing (EMAIL_API_KEY or SMTP credentials empty)")
	}

	// 1. Try HTTP REST API first if API key is provided (Port 443 - NEVER blocked by cloud firewalls)
	if c.apiKey != "" {
		if err := c.sendViaHTTPAPI(ctx, to, subject, htmlBody); err == nil {
			log.Info().Str("to", to).Str("subject", subject).Msg("email: dispatched successfully via HTTP REST API (Port 443)")
			return nil
		} else {
			log.Warn().Err(err).Msg("email: HTTP REST API dispatch failed, attempting SMTP fallback...")
		}
	}

	// 2. Try SMTP with port fallback (587 STARTTLS / 465)
	targetPort := c.port
	if targetPort == 0 {
		targetPort = 587
	}

	err := c.dispatchSingleSMTP(targetPort, to, subject, htmlBody)
	if err != nil && targetPort != 587 {
		log.Warn().Err(err).Int("failed_port", targetPort).Msg("email: Primary SMTP port failed, retrying via Port 587 STARTTLS...")
		err = c.dispatchSingleSMTP(587, to, subject, htmlBody)
	}

	if err != nil {
		log.Error().Err(err).Str("to", to).Msg("email: All email dispatch attempts failed")
		return err
	}

	log.Info().Str("to", to).Str("subject", subject).Msg("email: dispatched successfully via SMTP")
	return nil
}

// ── HTTP REST API Dispatchers (Port 443 HTTPS) ─────────────────────────────

func (c *Client) sendViaHTTPAPI(ctx context.Context, to, subject, htmlBody string) error {
	fromAddr := c.from
	if fromAddr == "" {
		fromAddr = c.username
	}

	provider := c.provider
	if provider == "" {
		if strings.HasPrefix(c.apiKey, "re_") {
			provider = "resend"
		} else if strings.HasPrefix(c.apiKey, "SG.") {
			provider = "sendgrid"
		} else if strings.HasPrefix(c.apiKey, "xkeysib") {
			provider = "brevo"
		} else {
			provider = "resend"
		}
	}

	switch provider {
	case "resend":
		return c.sendResend(ctx, fromAddr, to, subject, htmlBody)
	case "sendgrid":
		return c.sendSendGrid(ctx, fromAddr, to, subject, htmlBody)
	case "brevo":
		return c.sendBrevo(ctx, fromAddr, to, subject, htmlBody)
	default:
		return c.sendResend(ctx, fromAddr, to, subject, htmlBody)
	}
}

// Resend HTTP API (https://resend.com) — 3,000 free emails/month
func (c *Client) sendResend(ctx context.Context, from, to, subject, htmlBody string) error {
	if from == "" || strings.Contains(from, "gmail.com") {
		from = "onboarding@resend.dev" // Default testing sender if domain not verified
	}

	payload := map[string]interface{}{
		"from":    from,
		"to":      []string{to},
		"subject": subject,
		"html":    htmlBody,
	}
	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.resend.com/emails", bytes.NewReader(jsonBytes))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return fmt.Errorf("resend returned HTTP %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

// SendGrid HTTP API (https://sendgrid.com) — 100 free emails/day
func (c *Client) sendSendGrid(ctx context.Context, from, to, subject, htmlBody string) error {
	if from == "" {
		from = "no-reply@ecom.internal"
	}
	payload := map[string]interface{}{
		"personalizations": []map[string]interface{}{
			{"to": []map[string]string{{"email": to}}},
		},
		"from":    map[string]string{"email": from},
		"subject": subject,
		"content": []map[string]string{
			{"type": "text/html", "value": htmlBody},
		},
	}
	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.sendgrid.com/v3/mail/send", bytes.NewReader(jsonBytes))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("sendgrid returned HTTP %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

// Brevo HTTP API (https://brevo.com) — 300 free emails/day
func (c *Client) sendBrevo(ctx context.Context, from, to, subject, htmlBody string) error {
	if from == "" {
		from = "no-reply@ecom.internal"
	}
	payload := map[string]interface{}{
		"sender":      map[string]string{"email": from},
		"to":          []map[string]string{{"email": to}},
		"subject":     subject,
		"htmlContent": htmlBody,
	}
	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.brevo.com/v3/smtp/email", bytes.NewReader(jsonBytes))
	if err != nil {
		return err
	}
	req.Header.Set("api-key", c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("brevo returned HTTP %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

// ── Standard SMTP Dispatcher (Ports 587/465) ────────────────────────────────

func (c *Client) dispatchSingleSMTP(port int, to, subject, htmlBody string) error {
	addr := fmt.Sprintf("%s:%d", c.host, port)
	cleanPass := strings.ReplaceAll(c.password, " ", "")
	auth := smtp.PlainAuth("", c.username, cleanPass, c.host)

	fromAddr := c.from
	if fromAddr == "" {
		fromAddr = c.username
	}

	domain := "gmail.com"
	if parts := strings.Split(c.username, "@"); len(parts) == 2 {
		domain = parts[1]
	}
	msgID := fmt.Sprintf("<%d.%s>", time.Now().UnixNano(), domain)
	dateStr := time.Now().Format(time.RFC1123Z)

	msg := fmt.Sprintf(
		"From: %s\r\n"+
			"To: %s\r\n"+
			"Subject: %s\r\n"+
			"Date: %s\r\n"+
			"Message-ID: %s\r\n"+
			"MIME-Version: 1.0\r\n"+
			"Content-Type: text/html; charset=\"UTF-8\"\r\n"+
			"\r\n%s",
		fromAddr, to, subject, dateStr, msgID, htmlBody,
	)

	var err error

	if port == 465 {
		tlsCfg := &tls.Config{
			InsecureSkipVerify: false,
			ServerName:         c.host,
			MinVersion:         tls.VersionTLS12,
		}
		conn, dialErr := tls.DialWithDialer(
			&net.Dialer{Timeout: 5 * time.Second},
			"tcp", addr, tlsCfg,
		)
		if dialErr != nil {
			return fmt.Errorf("email: TLS dial failed on port 465: %w", dialErr)
		}
		defer conn.Close()

		client, clientErr := smtp.NewClient(conn, c.host)
		if clientErr != nil {
			return fmt.Errorf("email: SMTP client creation failed: %w", clientErr)
		}
		defer client.Quit()

		if err = client.Auth(auth); err != nil {
			return fmt.Errorf("email: SMTP auth failed: %w", err)
		}
		if err = client.Mail(c.username); err != nil {
			return fmt.Errorf("email: MAIL FROM failed: %w", err)
		}
		if err = client.Rcpt(to); err != nil {
			return fmt.Errorf("email: RCPT TO failed: %w", err)
		}
		w, wErr := client.Data()
		if wErr != nil {
			return fmt.Errorf("email: DATA command failed: %w", wErr)
		}
		if _, err = fmt.Fprint(w, msg); err != nil {
			return fmt.Errorf("email: write message failed: %w", err)
		}
		return w.Close()
	}

	// Port 587 / STARTTLS
	conn, dialErr := net.DialTimeout("tcp", addr, 10*time.Second)
	if dialErr != nil {
		return fmt.Errorf("email: STARTTLS TCP dial failed on port %d: %w", port, dialErr)
	}
	defer conn.Close()

	client, clientErr := smtp.NewClient(conn, c.host)
	if clientErr != nil {
		return fmt.Errorf("email: SMTP client creation failed: %w", clientErr)
	}
	defer client.Quit()

	tlsCfg := &tls.Config{
		ServerName: c.host,
		MinVersion: tls.VersionTLS12,
	}
	if err = client.StartTLS(tlsCfg); err != nil {
		return fmt.Errorf("email: STARTTLS upgrade failed: %w", err)
	}

	if err = client.Auth(auth); err != nil {
		return fmt.Errorf("email: SMTP auth failed: %w", err)
	}
	if err = client.Mail(c.username); err != nil {
		return fmt.Errorf("email: MAIL FROM failed: %w", err)
	}
	if err = client.Rcpt(to); err != nil {
		return fmt.Errorf("email: RCPT TO failed: %w", err)
	}
	w, wErr := client.Data()
	if wErr != nil {
		return fmt.Errorf("email: DATA command failed: %w", wErr)
	}
	if _, err = fmt.Fprint(w, msg); err != nil {
		return fmt.Errorf("email: write message failed: %w", err)
	}
	return w.Close()
}

// ── OTP Email ─────────────────────────────────────────────────────────────────

const otpEmailTpl = `
<!DOCTYPE html>
<html lang="bn">
<head><meta charset="UTF-8"><title>OTP Code</title></head>
<body style="font-family:Arial,sans-serif;background:#f4f4f4;margin:0;padding:0;">
  <table width="100%%" cellpadding="0" cellspacing="0" style="background:#f4f4f4;padding:40px 0;">
    <tr><td align="center">
      <table width="520" cellpadding="0" cellspacing="0"
             style="background:#ffffff;border-radius:12px;overflow:hidden;box-shadow:0 4px 20px rgba(0,0,0,.08);">
        <tr><td style="background:linear-gradient(135deg,#6C63FF,#4ECDC4);padding:32px 40px;text-align:center;">
          <h1 style="color:#fff;margin:0;font-size:24px;letter-spacing:1px;">🛍️ E-Commerce</h1>
        </td></tr>
        <tr><td style="padding:40px;">
          <h2 style="color:#333;margin-top:0;">আপনার OTP কোড</h2>
          <p style="color:#666;font-size:15px;">নিচের কোডটি ব্যবহার করুন:</p>
          <div style="background:#f8f7ff;border:2px dashed #6C63FF;border-radius:8px;
                      padding:24px;text-align:center;margin:24px 0;">
            <span style="font-size:42px;font-weight:bold;color:#6C63FF;
                         letter-spacing:12px;">{{.OTP}}</span>
          </div>
          <p style="color:#888;font-size:13px;">
            ⏰ এই কোডটি <strong>{{.ExpiresIn}}</strong> পর মেয়াদোত্তীর্ণ হবে।<br>
            🔒 এই কোড কারো সাথে শেয়ার করবেন না।
          </p>
        </td></tr>
        <tr><td style="background:#f8f8f8;padding:20px 40px;text-align:center;">
          <p style="color:#aaa;font-size:12px;margin:0;">
            যদি আপনি এই অনুরোধ না করে থাকেন, এই ইমেইল উপেক্ষা করুন।
          </p>
        </td></tr>
      </table>
    </td></tr>
  </table>
</body>
</html>`

// SendOTP sends a login/register OTP email.
func (c *Client) SendOTP(ctx context.Context, to, otpCode string) error {
	tpl, err := template.New("otp").Parse(otpEmailTpl)
	if err != nil {
		return fmt.Errorf("email: template parse error: %w", err)
	}
	var buf bytes.Buffer
	if err := tpl.Execute(&buf, map[string]string{
		"OTP":       otpCode,
		"ExpiresIn": "৫ মিনিট",
	}); err != nil {
		return fmt.Errorf("email: template execute error: %w", err)
	}
	return c.send(ctx, to, "Your OTP Code - E-Commerce", buf.String())
}

// SendPasswordResetOTP sends a password-reset OTP email.
func (c *Client) SendPasswordResetOTP(ctx context.Context, to, otpCode string) error {
	tpl, err := template.New("reset").Parse(otpEmailTpl)
	if err != nil {
		return fmt.Errorf("email: template parse error: %w", err)
	}
	var buf bytes.Buffer
	if err := tpl.Execute(&buf, map[string]string{
		"OTP":       otpCode,
		"ExpiresIn": "১৫ মিনিট",
	}); err != nil {
		return fmt.Errorf("email: template execute error: %w", err)
	}
	return c.send(ctx, to, "Password Reset OTP - E-Commerce", buf.String())
}

// ── Email Verification ────────────────────────────────────────────────────────

const verifyEmailTpl = `
<!DOCTYPE html>
<html lang="bn">
<head><meta charset="UTF-8"><title>Email Verify</title></head>
<body style="font-family:Arial,sans-serif;background:#f4f4f4;margin:0;padding:0;">
  <table width="100%%" cellpadding="0" cellspacing="0" style="background:#f4f4f4;padding:40px 0;">
    <tr><td align="center">
      <table width="520" cellpadding="0" cellspacing="0"
             style="background:#ffffff;border-radius:12px;overflow:hidden;box-shadow:0 4px 20px rgba(0,0,0,.08);">
        <tr><td style="background:linear-gradient(135deg,#6C63FF,#4ECDC4);padding:32px 40px;text-align:center;">
          <h1 style="color:#fff;margin:0;font-size:24px;">✉️ ইমেইল যাচাই করুন</h1>
        </td></tr>
        <tr><td style="padding:40px;">
          <h2 style="color:#333;margin-top:0;">স্বাগতম, {{.Name}}!</h2>
          <p style="color:#666;font-size:15px;">
            আপনার ইমেইল ঠিকানা যাচাই করতে নিচের বাটনে ক্লিক করুন:
          </p>
          <div style="text-align:center;margin:32px 0;">
            <a href="{{.Link}}"
               style="background:linear-gradient(135deg,#6C63FF,#4ECDC4);color:#fff;
                      text-decoration:none;padding:16px 40px;border-radius:8px;
                      font-size:16px;font-weight:bold;display:inline-block;">
              ইমেইল যাচাই করুন →
            </a>
          </div>
          <p style="color:#888;font-size:13px;">
            অথবা এই লিংকটি কপি করুন:<br>
            <a href="{{.Link}}" style="color:#6C63FF;word-break:break-all;">{{.Link}}</a>
          </p>
          <p style="color:#888;font-size:13px;">
            ⏰ এই লিংকটি <strong>২৪ ঘন্টা</strong> পর মেয়াদোত্তীর্ণ হবে।
          </p>
        </td></tr>
        <tr><td style="background:#f8f8f8;padding:20px 40px;text-align:center;">
          <p style="color:#aaa;font-size:12px;margin:0;">
            যদি আপনি এই অ্যাকাউন্ট তৈরি না করে থাকেন, এই ইমেইল উপেক্ষা করুন।
          </p>
        </td></tr>
      </table>
    </td></tr>
  </table>
</body>
</html>`

// SendVerificationEmail sends an account email-verification link.
func (c *Client) SendVerificationEmail(ctx context.Context, to, name, verificationLink string) error {
	tpl, err := template.New("verify").Parse(verifyEmailTpl)
	if err != nil {
		return fmt.Errorf("email: template parse error: %w", err)
	}
	var buf bytes.Buffer
	if err := tpl.Execute(&buf, map[string]string{
		"Name": name,
		"Link": verificationLink,
	}); err != nil {
		return fmt.Errorf("email: template execute error: %w", err)
	}
	return c.send(ctx, to, "Verify Your Email Address - E-Commerce", buf.String())
}
