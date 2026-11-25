package httpapi

import "testing"

func TestLogPathRedactsShareTokens(t *testing.T) {
	for in, want := range map[string]string{
		"/s/AbC_-123":                    "/s/[token]",
		"/s/AbC_-123/extra":              "/s/[token]/extra",
		"/api/v1/shared/AbC_-123":        "/api/v1/shared/[token]",
		"/api/v1/vehicles/1/share":       "/api/v1/vehicles/1/share",
		"/api/v1/vehicles/1/share/renew": "/api/v1/vehicles/1/share/renew",
		"/settings/instance":             "/settings/instance",
		"/s":                             "/s",
	} {
		if got := logPath(in); got != want {
			t.Errorf("logPath(%q) = %q, want %q", in, got, want)
		}
	}
}
