package authz

import "testing"

func TestResolveRequiresExactMethodAndPrefersStaticRoute(t *testing.T) {
	policies := []RoutePolicy{
		{Method: "GET", Pattern: "/problems/:id", Class: Protected},
		{Method: "GET", Pattern: "/problems/find", Class: Public},
	}

	policy, ok := Resolve(policies, "GET", "/problems/find")
	if !ok || policy.Class != Public {
		t.Fatalf("policy = %#v, ok=%t; want public static route", policy, ok)
	}
	if _, ok := Resolve(policies, "POST", "/problems/find"); ok {
		t.Fatal("unlisted method was resolved")
	}
}

func TestResolveRejectsAmbiguousOrUnsafePaths(t *testing.T) {
	policies := []RoutePolicy{
		{Method: "GET", Pattern: "/problems/:id", Class: Protected},
		{Method: "GET", Pattern: "/problems/:slug", Class: Protected},
	}
	if _, ok := Resolve(policies, "GET", "/problems/a%2Fb"); ok {
		t.Fatal("encoded slash must not reach policy resolution")
	}
	if _, ok := Resolve(policies, "GET", "/problems/123"); ok {
		t.Fatal("ambiguous policy match must fail closed")
	}
}

func TestResolveSelectsSpecificPublicRouteBeforeAuthenticatedFallback(t *testing.T) {
	policy, ok := Resolve([]RoutePolicy{
		{Method: "*", Pattern: "/**", Class: Authenticated},
		{Method: "GET", Pattern: "/problems/**", Class: Public},
	}, "GET", "/problems/123")
	if !ok || policy.Class != Public {
		t.Fatalf("policy=%+v ok=%t, want public", policy, ok)
	}
}
