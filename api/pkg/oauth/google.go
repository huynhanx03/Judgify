package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

const (
	googleTokenURL    = "https://oauth2.googleapis.com/token"
	googleUserInfoURL = "https://www.googleapis.com/oauth2/v3/userinfo"
	httpTimeout       = 10 * time.Second
)

// googleUserInfoResp represents the raw response from Google's userinfo endpoint.
type googleUserInfoResp struct {
	Sub           string `json:"sub"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Name          string `json:"name"`
	Picture       string `json:"picture"`
}

// googleTokenResp represents the token response from Google.
type googleTokenResp struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
	IDToken     string `json:"id_token"`
}

// GoogleProvider implements the Provider interface for Google OAuth.
type GoogleProvider struct {
	clientID     string
	clientSecret string
	redirectURL  string
}

// NewGoogleProvider creates a new Google OAuth provider.
func NewGoogleProvider(clientID, clientSecret, redirectURL string) *GoogleProvider {
	return &GoogleProvider{
		clientID:     clientID,
		clientSecret: clientSecret,
		redirectURL:  redirectURL,
	}
}

// Name returns the provider identifier.
func (g *GoogleProvider) Name() string {
	return "google"
}

// ExchangeCode exchanges a Google authorization code for standardized user info.
func (g *GoogleProvider) ExchangeCode(ctx context.Context, code string) (*OAuthUserInfo, error) {
	token, err := g.exchangeCodeForToken(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("exchange code: %w", err)
	}

	raw, err := g.fetchUserInfo(ctx, token.AccessToken)
	if err != nil {
		return nil, fmt.Errorf("fetch user info: %w", err)
	}

	return &OAuthUserInfo{
		ExternalID: raw.Sub,
		Metadata: map[string]any{
			"email":    raw.Email,
			"verified": raw.EmailVerified,
			"name":     raw.Name,
			"picture":  raw.Picture,
		},
	}, nil
}

// exchangeCodeForToken exchanges the authorization code for an access token.
func (g *GoogleProvider) exchangeCodeForToken(ctx context.Context, code string) (*googleTokenResp, error) {
	data := url.Values{
		"code":          {code},
		"client_id":     {g.clientID},
		"client_secret": {g.clientSecret},
		"redirect_uri":  {g.redirectURL},
		"grant_type":    {"authorization_code"},
	}

	client := &http.Client{Timeout: httpTimeout}
	resp, err := client.PostForm(googleTokenURL, data)
	if err != nil {
		return nil, fmt.Errorf("post token request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("google token endpoint returned %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp googleTokenResp
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, fmt.Errorf("decode token response: %w", err)
	}

	return &tokenResp, nil
}

// fetchUserInfo retrieves user info from Google using the access token.
func (g *GoogleProvider) fetchUserInfo(ctx context.Context, accessToken string) (*googleUserInfoResp, error) {
	client := &http.Client{Timeout: httpTimeout}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, googleUserInfoURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("get user info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("google userinfo endpoint returned %d: %s", resp.StatusCode, string(body))
	}

	var userInfo googleUserInfoResp
	if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
		return nil, fmt.Errorf("decode user info: %w", err)
	}

	return &userInfo, nil
}
