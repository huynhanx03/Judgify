package delivery

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/mail"
	"net/url"
	"strings"
	"time"

	"github.com/huynhanx03/go-common/pkg/encoding/json"

	"github.com/huynhanx03/judgify/internal/ports"
)

const defaultResendEndpoint = "https://api.resend.com/emails"

const maximumProviderErrorBodyBytes = 8 << 10

const (
	passwordResetDeliveryTokenPrefix = "jpr1."
	verificationDeliveryTokenPrefix  = "jcv1."
	invitationDeliveryTokenPrefix    = "jai1."
	passwordResetDeliveryEntropySize = 32
	passwordResetSubject             = "Đặt lại mật khẩu Judgify"
	passwordResetBody                = "Dùng liên kết một lần này để đặt lại mật khẩu trước khi hết hạn:\n\n"
	verificationSubject              = "Xác minh email khôi phục Judgify"
	verificationBody                 = "Dùng liên kết một lần này để xác minh email khôi phục trước khi hết hạn:\n\n"
	invitationSubject                = "Lời mời tham gia Judgify"
	invitationBody                   = "Dùng liên kết một lần này để xác nhận lời mời và tự đặt mật khẩu trước khi hết hạn:\n\n"
)

type ResendPasswordResetConfig struct {
	APIKey          string
	FromEmail       string
	FromName        string
	ResetURL        string
	VerificationURL string
	InvitationURL   string
	Endpoint        string
	// Client is cloned during construction. Redirects and the cookie jar are
	// disabled on the clone because each request carries one-time credentials.
	Client                   *http.Client
	AllowInsecureDevelopment bool
}

type ResendPasswordResetDelivery struct {
	apiKey string
	from   string
	url    string
	reset  *url.URL
	verify *url.URL
	invite *url.URL
	client *http.Client
}

type passwordResetProviderError struct {
	class     string
	retryable bool
	cause     error
}

func (failure *passwordResetProviderError) Error() string {
	if failure == nil || failure.class == "" {
		return "password reset delivery: provider failure"
	}
	return "password reset delivery: " + failure.class
}

func (failure *passwordResetProviderError) Unwrap() error {
	if failure == nil {
		return nil
	}
	return failure.cause
}

func (failure *passwordResetProviderError) Retryable() bool {
	return failure != nil && failure.retryable
}

func (failure *passwordResetProviderError) ErrorClass() string {
	if failure == nil || failure.class == "" {
		return "provider_failure"
	}
	return failure.class
}

func NewResendPasswordResetDelivery(
	config ResendPasswordResetConfig,
) (*ResendPasswordResetDelivery, error) {
	config.APIKey = strings.TrimSpace(config.APIKey)
	config.FromEmail = strings.TrimSpace(config.FromEmail)
	config.FromName = strings.TrimSpace(config.FromName)
	config.ResetURL = strings.TrimSpace(config.ResetURL)
	config.VerificationURL = strings.TrimSpace(config.VerificationURL)
	config.InvitationURL = strings.TrimSpace(config.InvitationURL)
	if config.Endpoint == "" {
		config.Endpoint = defaultResendEndpoint
	}
	if config.Client == nil {
		config.Client = &http.Client{Timeout: 10 * time.Second}
	}
	if !validResendSecret(config.APIKey) || !validMailbox(config.FromEmail) ||
		!validMailboxDisplayName(config.FromName) ||
		config.ResetURL == "" || config.VerificationURL == "" || config.InvitationURL == "" ||
		config.Endpoint == "" ||
		config.Client.Timeout <= 0 || config.Client.Timeout > 30*time.Second {
		return nil, fmt.Errorf("password reset delivery: invalid Resend configuration")
	}
	endpoint, err := validatedDeliveryURL(config.Endpoint, config.AllowInsecureDevelopment)
	if err != nil || endpoint.RawQuery != "" || endpoint.Fragment != "" || endpoint.User != nil {
		return nil, fmt.Errorf("password reset delivery: invalid Resend endpoint")
	}
	resetURL, err := validatedDeliveryURL(config.ResetURL, config.AllowInsecureDevelopment)
	if err != nil || resetURL.RawQuery != "" || resetURL.Fragment != "" || resetURL.User != nil {
		return nil, fmt.Errorf("password reset delivery: invalid reset URL")
	}
	verificationURL, err := validatedDeliveryURL(config.VerificationURL, config.AllowInsecureDevelopment)
	if err != nil || verificationURL.RawQuery != "" || verificationURL.Fragment != "" ||
		verificationURL.User != nil {
		return nil, fmt.Errorf("password reset delivery: invalid verification URL")
	}
	invitationURL, err := validatedDeliveryURL(config.InvitationURL, config.AllowInsecureDevelopment)
	if err != nil || invitationURL.RawQuery != "" || invitationURL.Fragment != "" ||
		invitationURL.User != nil {
		return nil, fmt.Errorf("password reset delivery: invalid invitation URL")
	}
	// Provider credentials, the recipient, and the one-time credential must
	// never be replayed to a redirect target. Clone the caller's client so the
	// adapter owns this invariant without mutating shared process state.
	client := *config.Client
	client.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}
	client.Jar = nil
	from := config.FromEmail
	if config.FromName != "" {
		from = (&mail.Address{Name: config.FromName, Address: config.FromEmail}).String()
	}
	return &ResendPasswordResetDelivery{
		apiKey: config.APIKey,
		from:   from,
		url:    endpoint.String(),
		reset:  resetURL,
		verify: verificationURL,
		invite: invitationURL,
		client: &client,
	}, nil
}

func (delivery *ResendPasswordResetDelivery) Deliver(
	ctx context.Context,
	message ports.PasswordResetDeliveryMessage,
) error {
	if delivery == nil || !validSecurityEmailDeliveryMessage(
		ctx, message.Recipient, message.Token, passwordResetDeliveryTokenPrefix,
		message.ExpiresAt, message.ProviderIdempotencyKey,
	) {
		return fmt.Errorf("password reset delivery: invalid message")
	}
	return delivery.deliverProvider(
		ctx,
		message.Recipient,
		oneTimeDeliveryLink(delivery.reset, message.Token),
		passwordResetSubject,
		passwordResetBody,
		message.ProviderIdempotencyKey,
	)
}

func (delivery *ResendPasswordResetDelivery) DeliverVerification(
	ctx context.Context,
	message ports.RecoveryContactVerificationDeliveryMessage,
) error {
	if delivery == nil || !validSecurityEmailDeliveryMessage(
		ctx, message.Recipient, message.Token, verificationDeliveryTokenPrefix,
		message.ExpiresAt, message.ProviderIdempotencyKey,
	) {
		return fmt.Errorf("recovery contact verification delivery: invalid message")
	}
	return delivery.deliverProvider(
		ctx,
		message.Recipient,
		oneTimeDeliveryLink(delivery.verify, message.Token),
		verificationSubject,
		verificationBody,
		message.ProviderIdempotencyKey,
	)
}

func (delivery *ResendPasswordResetDelivery) DeliverInvitation(
	ctx context.Context,
	message ports.AccountInvitationDeliveryMessage,
) error {
	if delivery == nil || !validSecurityEmailDeliveryMessage(
		ctx, message.Recipient, message.Token, invitationDeliveryTokenPrefix,
		message.ExpiresAt, message.ProviderIdempotencyKey,
	) {
		return fmt.Errorf("account invitation delivery: invalid message")
	}
	return delivery.deliverProvider(
		ctx,
		message.Recipient,
		oneTimeDeliveryLink(delivery.invite, message.Token),
		invitationSubject,
		invitationBody,
		message.ProviderIdempotencyKey,
	)
}

func oneTimeDeliveryLink(base *url.URL, token string) string {
	link := *base
	// Fragments are absent from HTTP requests and referrer headers. The client
	// consumes the credential into memory and immediately erases the address.
	link.Fragment = "token=" + token
	return link.String()
}

func (delivery *ResendPasswordResetDelivery) deliverProvider(
	ctx context.Context,
	recipient string,
	link string,
	subject string,
	bodyPrefix string,
	idempotencyKey string,
) error {
	body, err := json.Marshal(map[string]any{
		"from":    delivery.from,
		"to":      []string{recipient},
		"subject": subject,
		"text":    bodyPrefix + link,
	})
	if err != nil {
		return fmt.Errorf("password reset delivery: encode request: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, delivery.url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("password reset delivery: create request: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+delivery.apiKey)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", idempotencyKey)
	response, err := delivery.client.Do(request)
	if err != nil {
		return &passwordResetProviderError{
			class: "provider_unavailable", retryable: true, cause: sanitizeProviderCause(err),
		}
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		// Never include the provider body: it may echo recipient or content.
		code := providerErrorCode(response.Body)
		_ = response.Body.Close()
		return classifyProviderStatus(response.StatusCode, code)
	}
	// Draining a bounded success body lets net/http reuse the connection while
	// preventing a misbehaving provider from forcing an unbounded read.
	_, _ = io.CopyN(io.Discard, response.Body, maximumProviderErrorBodyBytes+1)
	_ = response.Body.Close()
	return nil
}

func validSecurityEmailDeliveryMessage(
	ctx context.Context,
	recipient string,
	token string,
	tokenPrefix string,
	expiresAt time.Time,
	idempotencyKey string,
) bool {
	return ctx != nil && ctx.Err() == nil && validMailbox(recipient) &&
		validOneTimeDeliveryToken(token, tokenPrefix) && !expiresAt.IsZero() &&
		validProviderIdempotencyKey(idempotencyKey)
}

func validOneTimeDeliveryToken(value string, prefix string) bool {
	if !strings.HasPrefix(value, prefix) {
		return false
	}
	encoded := strings.TrimPrefix(value, prefix)
	if len(encoded) != base64.RawURLEncoding.EncodedLen(passwordResetDeliveryEntropySize) {
		return false
	}
	decoded, err := base64.RawURLEncoding.DecodeString(encoded)
	return err == nil && len(decoded) == passwordResetDeliveryEntropySize
}

func classifyProviderStatus(status int, code string) error {
	switch {
	case status >= http.StatusMultipleChoices && status < http.StatusBadRequest:
		return &passwordResetProviderError{class: "provider_redirect_rejected", retryable: false}
	case status == http.StatusRequestTimeout || status == http.StatusTooManyRequests:
		return &passwordResetProviderError{class: "provider_throttled", retryable: true}
	case status == http.StatusConflict && code == "invalid_idempotent_request":
		return &passwordResetProviderError{class: "provider_idempotency_conflict", retryable: false}
	case status == http.StatusConflict:
		// Resend reports concurrent_idempotent_requests as 409. An unknown 409
		// is retried too: immutable payload + key make conflict impossible in a
		// healthy client, while treating provider concurrency as terminal can
		// lose an email accepted by the first in-flight request.
		return &passwordResetProviderError{class: "provider_idempotency_busy", retryable: true}
	case status >= http.StatusInternalServerError:
		return &passwordResetProviderError{class: "provider_unavailable", retryable: true}
	default:
		return &passwordResetProviderError{class: "provider_rejected", retryable: false}
	}
}

func providerErrorCode(body io.Reader) string {
	if body == nil {
		return ""
	}
	bounded, err := io.ReadAll(io.LimitReader(body, maximumProviderErrorBodyBytes+1))
	if err != nil || len(bounded) == 0 || len(bounded) > maximumProviderErrorBodyBytes {
		return ""
	}
	var response struct {
		Name  string `json:"name"`
		Code  string `json:"code"`
		Error struct {
			Name string `json:"name"`
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(bounded, &response); err != nil {
		return ""
	}
	for _, candidate := range []string{
		response.Name, response.Code, response.Error.Name, response.Error.Code,
	} {
		switch candidate {
		case "concurrent_idempotent_requests", "invalid_idempotent_request":
			return candidate
		}
	}
	return ""
}

func sanitizeProviderCause(err error) error {
	if errors.Is(err, context.DeadlineExceeded) {
		return context.DeadlineExceeded
	}
	if errors.Is(err, context.Canceled) {
		return context.Canceled
	}
	return nil
}

func validResendSecret(value string) bool {
	if value == "" || len(value) > 4096 || strings.Contains(value, "${") ||
		strings.TrimSpace(value) != value {
		return false
	}
	for _, character := range value {
		if character <= 0x20 || character == 0x7f {
			return false
		}
	}
	return true
}

func validMailbox(value string) bool {
	if value == "" || len(value) > 320 || strings.Contains(value, "${") ||
		strings.ContainsAny(value, "\r\n") {
		return false
	}
	parsed, err := mail.ParseAddress(value)
	return err == nil && parsed.Name == "" && parsed.Address == value
}

func validMailboxDisplayName(value string) bool {
	if len(value) > 128 || strings.Contains(value, "${") {
		return false
	}
	for _, character := range value {
		if character < 0x20 || character == 0x7f {
			return false
		}
	}
	return true
}

func validatedDeliveryURL(value string, allowInsecureDevelopment bool) (*url.URL, error) {
	parsed, err := url.ParseRequestURI(value)
	if err != nil || parsed.Host == "" {
		return nil, fmt.Errorf("invalid URL")
	}
	if parsed.Scheme == "https" {
		return parsed, nil
	}
	if parsed.Scheme != "http" || !allowInsecureDevelopment || !loopbackHost(parsed.Hostname()) {
		return nil, fmt.Errorf("insecure URL")
	}
	return parsed, nil
}

func loopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	address := net.ParseIP(host)
	return address != nil && address.IsLoopback()
}

// DiscardPasswordResetDelivery is an explicit development/test sink. It
// validates the secret-bearing message and intentionally stores/logs nothing.
type DiscardPasswordResetDelivery struct{}

func (DiscardPasswordResetDelivery) Deliver(
	ctx context.Context,
	message ports.PasswordResetDeliveryMessage,
) error {
	if !validSecurityEmailDeliveryMessage(
		ctx, message.Recipient, message.Token, passwordResetDeliveryTokenPrefix,
		message.ExpiresAt, message.ProviderIdempotencyKey,
	) {
		return fmt.Errorf("password reset discard sink: invalid message")
	}
	return nil
}

func (DiscardPasswordResetDelivery) DeliverVerification(
	ctx context.Context,
	message ports.RecoveryContactVerificationDeliveryMessage,
) error {
	if !validSecurityEmailDeliveryMessage(
		ctx, message.Recipient, message.Token, verificationDeliveryTokenPrefix,
		message.ExpiresAt, message.ProviderIdempotencyKey,
	) {
		return fmt.Errorf("recovery contact verification discard sink: invalid message")
	}
	return nil
}

func (DiscardPasswordResetDelivery) DeliverInvitation(
	ctx context.Context,
	message ports.AccountInvitationDeliveryMessage,
) error {
	if !validSecurityEmailDeliveryMessage(
		ctx, message.Recipient, message.Token, invitationDeliveryTokenPrefix,
		message.ExpiresAt, message.ProviderIdempotencyKey,
	) {
		return fmt.Errorf("account invitation discard sink: invalid message")
	}
	return nil
}

func validProviderIdempotencyKey(value string) bool {
	if value == "" || len(value) > 128 || strings.TrimSpace(value) != value {
		return false
	}
	for index := 0; index < len(value); index++ {
		if value[index] <= 0x20 || value[index] >= 0x7f {
			return false
		}
	}
	return true
}

var _ ports.PasswordResetDelivery = (*ResendPasswordResetDelivery)(nil)
var _ ports.SecurityEmailDelivery = (*ResendPasswordResetDelivery)(nil)
var _ ports.PasswordResetDelivery = DiscardPasswordResetDelivery{}
var _ ports.SecurityEmailDelivery = DiscardPasswordResetDelivery{}
var _ ports.PasswordResetDeliveryFailure = (*passwordResetProviderError)(nil)
