package infrastructure

import (
	"testing"
	"time"

	"github.com/huynhanx03/judgify/pkg/settings"
)

func TestHTTPRateLimitPolicyPropagatesTrustedProxies(t *testing.T) {
	server := settings.Server{
		TrustedProxies: []string{"172.31.0.0/24"},
		RateLimit: settings.RateLimitConfig{
			Limit: 100, Burst: 50, Window: 60,
		},
	}

	policy, err := buildRateLimitPolicy(server, 10)
	if err != nil {
		t.Fatalf("build rate-limit policy: %v", err)
	}
	if policy.Limit != 10 || policy.Burst != 5 || policy.Window != time.Minute {
		t.Fatalf("unexpected bounded auth policy: %#v", policy)
	}
	if len(policy.TrustedProxies) != 1 || policy.TrustedProxies[0] != "172.31.0.0/24" {
		t.Fatalf("trusted proxies = %v", policy.TrustedProxies)
	}

	server.TrustedProxies[0] = "203.0.113.1"
	if policy.TrustedProxies[0] != "172.31.0.0/24" {
		t.Fatal("rate-limit policy aliases mutable server configuration")
	}
}

func TestHTTPRateLimitPolicyRejectsInvalidTrustedProxyAtBootstrap(t *testing.T) {
	server := settings.Server{
		TrustedProxies: []string{"not-an-address"},
		RateLimit: settings.RateLimitConfig{
			Limit: 100, Burst: 50, Window: 60,
		},
	}
	if _, err := buildRateLimitPolicy(server, 1); err == nil {
		t.Fatal("invalid trusted proxy was accepted")
	}
}

func TestHTTPRateLimitPolicyKeepsSmallPositiveQuotasUsable(t *testing.T) {
	server := settings.Server{
		RateLimit: settings.RateLimitConfig{
			Limit:  1,
			Burst:  9,
			Window: 60,
		},
	}
	policy, err := buildRateLimitPolicy(server, 10)
	if err != nil {
		t.Fatalf("buildRateLimitPolicy: %v", err)
	}
	if policy.Limit != 1 || policy.Burst != 1 {
		t.Fatalf("small quota divided to limit=%d burst=%d, want 1/1", policy.Limit, policy.Burst)
	}
}
