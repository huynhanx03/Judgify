package di

import "testing"

func TestWebSocketOptionsPropagateTrustedProxiesWithoutAliasing(t *testing.T) {
	origins := []string{"https://judgify.example.com"}
	proxies := []string{"172.31.0.0/24"}
	options := buildHubOptions(origins, proxies)

	if len(options.AllowedOrigins) != 1 || options.AllowedOrigins[0] != origins[0] {
		t.Fatalf("allowed origins = %v", options.AllowedOrigins)
	}
	if len(options.TrustedProxies) != 1 || options.TrustedProxies[0] != proxies[0] {
		t.Fatalf("trusted proxies = %v", options.TrustedProxies)
	}

	origins[0] = "https://attacker.example"
	proxies[0] = "0.0.0.0/0"
	if options.AllowedOrigins[0] != "https://judgify.example.com" ||
		options.TrustedProxies[0] != "172.31.0.0/24" {
		t.Fatal("websocket options alias mutable configuration")
	}
}
