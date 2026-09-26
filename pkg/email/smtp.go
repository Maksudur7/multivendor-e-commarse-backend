// Package email provides an SMTP email client for transactional emails.
// Supports TLS (port 465) and STARTTLS (port 587) connections.
//
// Usage:
//
//	client := email.NewClient(cfg.Email)
//	err := client.SendOTP(ctx, "user@example.com", "482910")
package email

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"html/template"
	"net"
	"net/smtp"
	"strings"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/yourusername/ecom-backend/config"
)

// Client is an SMTP email client.
type Client struct {
	host     string
	port     int
	username string
	password string
	from     string
}

// NewClient constructs an email client from config.
func NewClient(cfg config.EmailConfig) *Client {
	return &Client{
		host:     cfg.SMTPHost,
		port:     cfg.SMTPPort,
		username: cfg.Username,
		password: cfg.Password,
		from:     cfg.From,
	}
}

// IsConfigured returns true if SMTP credentials are set.
func (c *Client) IsConfigured() bool {
	return c.host != "" && c.username != "" && c.password != ""
}

// send is the core SMTP dispatcher with automatic port fallback.
func (c *Client) send(ctx context.Context, to, subject, htmlBody string) error {
	if !c.IsConfigured() {
		log.Warn().Str("to", to).Msg("email: SMTP not configured — skipping email dispatch")
		return nil
	}

	targetPort := c.port
	if targetPort == 0 {
		targetPort = 587
	}

	err := c.dispatchSingle(targetPort, to, subject, htmlBody)
	if err != nil && targetPort != 587 {
		log.Warn().Err(err).Int("failed_port", targetPort).Msg("email: Primary SMTP port failed, retrying via Port 587 STARTTLS...")
		err = c.dispatchSingle(587, to, subject, htmlBody)
	}

	if err != nil {
		log.Error().Err(err).Str("to", to).Msg("email: All SMTP dispatch attempts failed")
		return err
	}

	log.Info().Str("to", to).Str("subject", subject).Msg("email: dispatched successfully")
	return nil
}

func (c *Client) dispatchSingle(port int, to, subject, htmlBody string) error {
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
