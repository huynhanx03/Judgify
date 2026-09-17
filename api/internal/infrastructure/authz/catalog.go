package authz

// DefaultPolicies defines the outer admission boundary. Every non-public API
// route requires a verified session here; endpoint-specific Casbin decisions
// remain in the existing HTTP handlers until their route manifests are moved
// one by one to this catalog. The wildcard is intentionally last because
// Resolve selects the most-specific match.
func DefaultPolicies() []RoutePolicy {
	return []RoutePolicy{
		{Method: "GET", Pattern: "/ping", Class: Public},
		{Method: "GET", Pattern: "/live", Class: Public},
		{Method: "GET", Pattern: "/ready", Class: Public},
		{Method: "POST", Pattern: "/pre-session", Class: Public},
		{Method: "POST", Pattern: "/register", Class: Public},
		{Method: "POST", Pattern: "/login", Class: Public},
		{Method: "POST", Pattern: "/auth/refresh", Class: Public},
		{Method: "POST", Pattern: "/auth/forgot-password", Class: Public},
		{Method: "POST", Pattern: "/auth/reset-password", Class: Public},
		{Method: "POST", Pattern: "/auth/oauth/**", Class: Public},
		{Method: "GET", Pattern: "/auth/oauth/**", Class: Public},
		{Method: "POST", Pattern: "/onboarding/trait-offers", Class: Public},
		{Method: "POST", Pattern: "/invitations/activate", Class: Public},
		{Method: "POST", Pattern: "/recovery-contact/verify", Class: Public},
		{Method: "GET", Pattern: "/problems/**", Class: Public},
		{Method: "POST", Pattern: "/problems/find", Class: Public},
		{Method: "GET", Pattern: "/contests/**", Class: Public},
		{Method: "POST", Pattern: "/contests/find", Class: Public},
		{Method: "GET", Pattern: "/materials/**", Class: Public},
		{Method: "POST", Pattern: "/materials/search", Class: Public},
		{Method: "GET", Pattern: "/rankings/**", Class: Public},
		{Method: "GET", Pattern: "/categories/**", Class: Public},
		{Method: "GET", Pattern: "/submissions/**", Class: Public},
		{Method: "*", Pattern: "/**", Class: Authenticated},
	}
}
