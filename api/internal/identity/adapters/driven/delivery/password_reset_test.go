package delivery

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/huynhanx03/judgify/internal/ports"
)

func TestResendPasswordResetDeliveryBuildsOneTimeHTTPSStyleLink(t *testing.T) {
	type providerRequest struct {
		From string   `json:"from"`
		To   []string `json:"to"`
		Text string   `json:"text"`
	}
	requestReceived := make(chan providerRequest, 1)
	client := &http.Client{Timeout: time.Second, Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		require.Equal(t, "Bearer re_test_key", request.Header.Get("Authorization"))
		require.Equal(t, "judgify-security-email/01911a8f-test", request.Header.Get("Idempotency-Key"))
		body, err := io.ReadAll(request.Body)
		require.NoError(t, err)
		var payload providerRequest
		require.NoError(t, json.Unmarshal(body, &payload))
		requestReceived <- payload
		return &http.Response{
			StatusCode: http.StatusAccepted,
			Body:       io.NopCloser(strings.NewReader("")),
			Header:     make(http.Header),
		}, nil
	})}
	delivery, err := NewResendPasswordResetDelivery(ResendPasswordResetConfig{
		APIKey: "re_test_key", FromEmail: "security@example.com", FromName: "Judgify",
		ResetURL: "http://localhost:3000/reset-password", Endpoint: "http://localhost:8089/emails",
		VerificationURL: "http://localhost:3000/verify-recovery-contact",
		InvitationURL:   "http://localhost:3000/accept-invitation",
		Client:          client, AllowInsecureDevelopment: true,
	})
	require.NoError(t, err)
	token := "jpr1." + strings.Repeat("A", 43)
	require.NoError(t, delivery.Deliver(context.Background(), ports.PasswordResetDeliveryMessage{
		Recipient: "user@example.com", Token: token, ExpiresAt: time.Now().Add(time.Minute),
		ProviderIdempotencyKey: "judgify-security-email/01911a8f-test",
	}))

	payload := <-requestReceived
	require.Equal(t, `"Judgify" <security@example.com>`, payload.From)
	require.Equal(t, []string{"user@example.com"}, payload.To)
	require.Contains(t, payload.Text, "http://localhost:3000/reset-password#token="+token)
	require.NotContains(t, payload.Text, "reset-password?token=")
}

func TestResendPasswordResetDeliveryNeverReturnsProviderBody(t *testing.T) {
	client := &http.Client{Timeout: time.Second, Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusBadRequest,
			Body: io.NopCloser(strings.NewReader(
				"user@example.com jpr1.secret-provider-echo",
			)),
			Header: make(http.Header),
		}, nil
	})}
	delivery, err := NewResendPasswordResetDelivery(ResendPasswordResetConfig{
		APIKey: "re_test_key", FromEmail: "security@example.com",
		ResetURL: "http://localhost:3000/reset-password", Endpoint: "http://localhost:8089/emails",
		VerificationURL: "http://localhost:3000/verify-recovery-contact",
		InvitationURL:   "http://localhost:3000/accept-invitation",
		Client:          client, AllowInsecureDevelopment: true,
	})
	require.NoError(t, err)
	err = delivery.Deliver(context.Background(), ports.PasswordResetDeliveryMessage{
		Recipient: "user@example.com", Token: "jpr1." + strings.Repeat("A", 43),
		ExpiresAt: time.Now().Add(time.Minute), ProviderIdempotencyKey: "judgify-security-email/test",
	})
	require.Error(t, err)
	require.NotContains(t, err.Error(), "user@example.com")
	require.NotContains(t, err.Error(), "secret-provider-echo")
}

func TestResendPasswordResetDeliveryClassifiesRetryPolicyWithoutProviderBody(t *testing.T) {
	for _, test := range []struct {
		name      string
		status    int
		body      string
		retryable bool
		class     string
	}{
		{name: "throttled", status: http.StatusTooManyRequests, retryable: true, class: "provider_throttled"},
		{name: "server unavailable", status: http.StatusServiceUnavailable, retryable: true, class: "provider_unavailable"},
		{name: "request rejected", status: http.StatusBadRequest, retryable: false, class: "provider_rejected"},
		{
			name: "concurrent idempotent request", status: http.StatusConflict,
			body:      `{"name":"concurrent_idempotent_requests","message":"possibly secret"}`,
			retryable: true, class: "provider_idempotency_busy",
		},
		{
			name: "idempotency payload conflict", status: http.StatusConflict,
			body:      `{"name":"invalid_idempotent_request","message":"possibly secret"}`,
			retryable: false, class: "provider_idempotency_conflict",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			client := &http.Client{Timeout: time.Second, Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: test.status,
					Body: io.NopCloser(strings.NewReader(func() string {
						if test.body != "" {
							return test.body
						}
						return "user@example.com jpr1.secret-provider-echo"
					}())),
					Header: make(http.Header),
				}, nil
			})}
			delivery, err := NewResendPasswordResetDelivery(ResendPasswordResetConfig{
				APIKey: "re_test_key", FromEmail: "security@example.com",
				ResetURL: "http://localhost:3000/reset-password", Endpoint: "http://localhost:8089/emails",
				VerificationURL: "http://localhost:3000/verify-recovery-contact",
				InvitationURL:   "http://localhost:3000/accept-invitation",
				Client:          client, AllowInsecureDevelopment: true,
			})
			require.NoError(t, err)
			err = delivery.Deliver(context.Background(), ports.PasswordResetDeliveryMessage{
				Recipient: "user@example.com", Token: "jpr1." + strings.Repeat("A", 43),
				ExpiresAt: time.Now().Add(time.Minute), ProviderIdempotencyKey: "judgify-security-email/test",
			})
			var failure ports.PasswordResetDeliveryFailure
			require.True(t, errors.As(err, &failure))
			require.Equal(t, test.retryable, failure.Retryable())
			require.Equal(t, test.class, failure.ErrorClass())
			require.NotContains(t, err.Error(), "user@example.com")
			require.NotContains(t, err.Error(), "secret-provider-echo")
		})
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (roundTrip roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return roundTrip(request)
}

func TestResendPasswordResetDeliveryFailsClosedForInsecureOrPlaceholderConfig(t *testing.T) {
	base := ResendPasswordResetConfig{
		APIKey: "re_test_key", FromEmail: "security@example.com",
		ResetURL:        "https://app.example.com/reset-password",
		VerificationURL: "https://app.example.com/verify-recovery-contact",
		InvitationURL:   "https://app.example.com/accept-invitation",
		Client:          &http.Client{Timeout: time.Second},
	}

	_, err := NewResendPasswordResetDelivery(base)
	require.NoError(t, err, "base fixture must itself be valid so each sub-case exercises only its own defect")

	insecure := base
	insecure.ResetURL = "http://localhost:3000/reset-password"
	_, err = NewResendPasswordResetDelivery(insecure)
	require.EqualError(t, err, "password reset delivery: invalid reset URL")

	placeholderEmail := base
	placeholderEmail.FromEmail = "${RESEND_FROM_EMAIL}"
	_, err = NewResendPasswordResetDelivery(placeholderEmail)
	require.EqualError(t, err, "password reset delivery: invalid Resend configuration")

	placeholderKey := base
	placeholderKey.APIKey = "${RESEND_API_KEY}"
	_, err = NewResendPasswordResetDelivery(placeholderKey)
	require.EqualError(t, err, "password reset delivery: invalid Resend configuration")

	placeholderName := base
	placeholderName.FromName = "${RESEND_FROM_NAME}"
	_, err = NewResendPasswordResetDelivery(placeholderName)
	require.EqualError(t, err, "password reset delivery: invalid Resend configuration")

	resetWithQuery := base
	resetWithQuery.ResetURL = "https://app.example.com/reset-password?token=legacy"
	_, err = NewResendPasswordResetDelivery(resetWithQuery)
	require.EqualError(t, err, "password reset delivery: invalid reset URL")
}

func TestResendPasswordResetDeliveryDoesNotReplaySecretsAcrossRedirects(t *testing.T) {
	requests := 0
	callerRedirectPolicyUsed := false
	client := &http.Client{
		Timeout: time.Second,
		Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			requests++
			if requests > 1 {
				t.Fatalf("secret-bearing request followed redirect to %s", request.URL.Redacted())
			}
			return &http.Response{
				StatusCode: http.StatusTemporaryRedirect,
				Body:       io.NopCloser(strings.NewReader("redirect")),
				Header: http.Header{
					"Location": []string{"https://attacker.example/collect"},
				},
			}, nil
		}),
		CheckRedirect: func(*http.Request, []*http.Request) error {
			callerRedirectPolicyUsed = true
			return nil
		},
	}
	delivery, err := NewResendPasswordResetDelivery(ResendPasswordResetConfig{
		APIKey: "re_test_key", FromEmail: "security@example.com",
		ResetURL:        "https://app.example.com/reset-password",
		VerificationURL: "https://app.example.com/verify-recovery-contact",
		InvitationURL:   "https://app.example.com/accept-invitation",
		Endpoint:        "https://api.example.com/emails",
		Client:          client,
	})
	require.NoError(t, err)

	err = delivery.Deliver(context.Background(), ports.PasswordResetDeliveryMessage{
		Recipient: "user@example.com", Token: "jpr1." + strings.Repeat("A", 43),
		ExpiresAt: time.Now().Add(time.Minute), ProviderIdempotencyKey: "judgify-security-email/test",
	})
	var failure ports.PasswordResetDeliveryFailure
	require.ErrorAs(t, err, &failure)
	require.False(t, failure.Retryable())
	require.Equal(t, "provider_redirect_rejected", failure.ErrorClass())
	require.Equal(t, 1, requests)
	require.False(t, callerRedirectPolicyUsed)

	// The adapter clones the supplied client instead of mutating a client that
	// may be shared by other infrastructure components.
	require.NotNil(t, client.CheckRedirect)
	require.NoError(t, client.CheckRedirect(&http.Request{URL: &url.URL{}}, nil))
	require.True(t, callerRedirectPolicyUsed)
}

func TestResendPasswordResetDeliveryDrainsAndClosesBoundedSuccessBody(t *testing.T) {
	body := &trackingReadCloser{Reader: strings.NewReader("accepted")}
	client := &http.Client{Timeout: time.Second, Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusAccepted,
			Body:       body,
			Header:     make(http.Header),
		}, nil
	})}
	delivery, err := NewResendPasswordResetDelivery(ResendPasswordResetConfig{
		APIKey: "re_test_key", FromEmail: "security@example.com",
		ResetURL: "http://localhost:3000/reset-password", Endpoint: "http://localhost:8089/emails",
		VerificationURL: "http://localhost:3000/verify-recovery-contact",
		InvitationURL:   "http://localhost:3000/accept-invitation",
		Client:          client, AllowInsecureDevelopment: true,
	})
	require.NoError(t, err)

	require.NoError(t, delivery.Deliver(context.Background(), ports.PasswordResetDeliveryMessage{
		Recipient: "user@example.com", Token: "jpr1." + strings.Repeat("A", 43),
		ExpiresAt: time.Now().Add(time.Minute), ProviderIdempotencyKey: "judgify-security-email/test",
	}))
	require.True(t, body.readToEOF)
	require.True(t, body.closed)
}

func TestSecurityEmailDeliveriesRejectMalformedBoundaryValues(t *testing.T) {
	requests := 0
	client := &http.Client{Timeout: time.Second, Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		requests++
		return &http.Response{
			StatusCode: http.StatusAccepted,
			Body:       io.NopCloser(strings.NewReader("")),
			Header:     make(http.Header),
		}, nil
	})}
	delivery, err := NewResendPasswordResetDelivery(ResendPasswordResetConfig{
		APIKey: "re_test_key", FromEmail: "security@example.com",
		ResetURL: "http://localhost:3000/reset-password", Endpoint: "http://localhost:8089/emails",
		VerificationURL: "http://localhost:3000/verify-recovery-contact",
		InvitationURL:   "http://localhost:3000/accept-invitation",
		Client:          client, AllowInsecureDevelopment: true,
	})
	require.NoError(t, err)

	err = delivery.Deliver(context.Background(), ports.PasswordResetDeliveryMessage{
		Recipient: "not-a-mailbox", Token: "jpr1." + strings.Repeat("A", 43),
		ExpiresAt: time.Now().Add(time.Minute), ProviderIdempotencyKey: "judgify-security-email/test",
	})
	require.Error(t, err)
	require.Zero(t, requests)

	err = (DiscardPasswordResetDelivery{}).Deliver(context.Background(), ports.PasswordResetDeliveryMessage{
		Recipient: "user@example.com", Token: "jpr1.truncated",
		ExpiresAt: time.Now().Add(time.Minute), ProviderIdempotencyKey: "judgify-security-email/test",
	})
	require.Error(t, err)
}

func TestResendPasswordResetDeliveryFormatsDisplayNameAsMailbox(t *testing.T) {
	delivery, err := NewResendPasswordResetDelivery(ResendPasswordResetConfig{
		APIKey: "re_test_key", FromEmail: "security@example.com", FromName: "Judgify, Inc.",
		ResetURL:        "https://app.example.com/reset-password",
		VerificationURL: "https://app.example.com/verify-recovery-contact",
		InvitationURL:   "https://app.example.com/accept-invitation",
		Client:          &http.Client{Timeout: time.Second},
	})
	require.NoError(t, err)
	require.Equal(t, `"Judgify, Inc." <security@example.com>`, delivery.from)
}

type trackingReadCloser struct {
	*strings.Reader
	readToEOF bool
	closed    bool
}

func (body *trackingReadCloser) Read(buffer []byte) (int, error) {
	read, err := body.Reader.Read(buffer)
	if errors.Is(err, io.EOF) {
		body.readToEOF = true
	}
	return read, err
}

func (body *trackingReadCloser) Close() error {
	body.closed = true
	return nil
}
