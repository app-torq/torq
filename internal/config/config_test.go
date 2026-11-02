package config

import "testing"

func env(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestLoadDefaults(t *testing.T) {
	cfg, err := Load(env(nil))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DataDir != "/data" || cfg.ListenAddr != ":8080" || cfg.MaxUploadMB != 25 ||
		cfg.LogFormat != "json" || cfg.LogLevel != "info" || cfg.CookieSecure != CookieSecureAuto {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
	if cfg.SecureCookies() {
		t.Fatal("auto without https base URL must not be secure")
	}
}

func TestLoadSecureAuto(t *testing.T) {
	cfg, err := Load(env(map[string]string{"TORQ_BASE_URL": "https://torq.example/"}))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.BaseURL != "https://torq.example" || !cfg.SecureCookies() {
		t.Fatalf("got %+v", cfg)
	}
}

func TestLoadErrors(t *testing.T) {
	cases := map[string]map[string]string{
		"invalid cidr":    {"TORQ_TRUSTED_PROXIES": "10.0.0.0/8, nope"},
		"relative base":   {"TORQ_BASE_URL": "/torq"},
		"ftp base":        {"TORQ_BASE_URL": "ftp://x"},
		"upload zero":     {"TORQ_MAX_UPLOAD_MB": "0"},
		"upload negative": {"TORQ_MAX_UPLOAD_MB": "-3"},
		"upload non-int":  {"TORQ_MAX_UPLOAD_MB": "lots"},
		"bad cookie":      {"TORQ_COOKIE_SECURE": "maybe"},
		"bad log format":  {"TORQ_LOG_FORMAT": "xml"},
		"bad log level":   {"TORQ_LOG_LEVEL": "loud"},
		"bad demo mode":   {"TORQ_DEMO_MODE": "yes"},
	}
	for name, m := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := Load(env(m)); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestTrustedProxies(t *testing.T) {
	cfg, err := Load(env(map[string]string{"TORQ_TRUSTED_PROXIES": "10.0.0.1/8,192.168.1.0/24"}))
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.TrustedProxies) != 2 || cfg.TrustedProxies[0].String() != "10.0.0.0/8" {
		t.Fatalf("got %v", cfg.TrustedProxies)
	}
}
