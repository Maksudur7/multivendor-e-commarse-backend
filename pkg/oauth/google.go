// Package oauth provides Google OAuth 2.0 authentication.
//
// Flow:
//  1. Frontend redirects user to Google URL returned by AuthURL().
//  2. Google redirects back to /auth/google/callback with ?code=<code>&state=<state>.
//  3. Backend calls ExchangeCode() to get access token, then GetUserInfo().
//
// Required .env variables:
//
//	GOOGLE_CLIENT_ID=<from Google Cloud Console>
//	GOOGLE_CLIENT_SECRET=<from Google Cloud Console>
//	GOOGLE_REDIRECT_URL=https://yourdomain.com/api/v1/auth/google/callback
package oauth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// GoogleUserInfo is the user profile returned by Google's userinfo endpoint.
type GoogleUserInfo struct {
	ID            string `json:"id"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Name          string `json:"name"`
	GivenName     string `json:"given_name"`
	FamilyName    string `json:"family_name"`
	Picture       string `json:"picture"`
	Locale        string `json:"locale"`
}

// GoogleClient wraps the OAuth2 config for Google.
type GoogleClient struct {
	config *oauth2.Config
}

// NewGoogleClient constructs the Google OAuth client.
// clientID, clientSecret come from Google Cloud Console.
// redirectURL is your backend's callback URL (must be registered in Google Console).
func NewGoogleClient(clientID, clientSecret, redirectURL string) *GoogleClient {
	cfg := &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
		Scopes: []string{
			"https://www.googleapis.com/auth/userinfo.email",
			"https://www.googleapis.com/auth/userinfo.profile",
			"openid",
		},
		Endpoint: google.Endpoint,
	}
	return &GoogleClient{config: cfg}
}

// IsConfigured returns true if Google credentials are set.
func (g *GoogleClient) IsConfigured() bool {
	return g.config.ClientID != "" && g.config.ClientSecret != ""
}

// GenerateState creates a cryptographically random state token
// to prevent CSRF attacks in the OAuth flow.
func GenerateState() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("oauth: failed to generate state: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// AuthURL returns the Google OAuth consent page URL.
// state must be a random value stored in session/cookie before redirect.
func (g *GoogleClient) AuthURL(state string) string {
	return g.config.AuthCodeURL(state, oauth2.AccessTypeOnline)
}

// ExchangeCode exchanges the authorization code for a token pair.
func (g *GoogleClient) ExchangeCode(ctx context.Context, code string) (*oauth2.Token, error) {
	token, err := g.config.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("oauth: code exchange failed: %w", err)
	}
	return token, nil
}

// GetUserInfo fetches the authenticated user's profile from Google.
func (g *GoogleClient) GetUserInfo(ctx context.Context, token *oauth2.Token) (*GoogleUserInfo, error) {
	httpClient := g.config.Client(ctx, token)
	httpClient.Timeout = 10 * time.Second

	resp, err := httpClient.Get("https://www.googleapis.com/oauth2/v2/userinfo")
	if err != nil {
		return nil, fmt.Errorf("oauth: userinfo request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("oauth: userinfo returned %d: %s", resp.StatusCode, body)
	}

	var info GoogleUserInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, fmt.Errorf("oauth: failed to decode userinfo: %w", err)
	}
	if info.Email == "" {
		return nil, fmt.Errorf("oauth: userinfo has no email — check scopes")
	}
	return &info, nil
}
