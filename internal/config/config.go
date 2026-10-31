// Package config loads torq's runtime configuration from environment variables.
package config

import (
	"fmt"
	"net/netip"
	"net/url"
	"os"
	"strconv"
	"strings"
)

// CookieSecure controls the Secure flag on the session cookie.
type CookieSecure string

const (
	CookieSecureAuto  CookieSecure = "auto"
	CookieSecureTrue  CookieSecure = "true"
	CookieSecureFalse CookieSecure = "false"
)

// Config is the typed runtime configuration (research R20).
type Config struct {
	DataDir        string
	ListenAddr     string
	BaseURL        string // empty or absolute http(s) URL without trailing slash
	CookieSecure   CookieSecure
	TrustedProxies []netip.Prefix
	MaxUploadMB    int
	LogLevel       string
	LogFormat      string // "json" or "text"
	// CatalogOnlineUpdates lets admins refresh the vehicle catalog from its
	// upstream sources (Constitution VII); lookups always stay offline.
	CatalogOnlineUpdates bool
}

// MaxUploadBytes returns the per-file upload limit in bytes.
func (c Config) MaxUploadBytes() int64 { return int64(c.MaxUploadMB) << 20 }

// SecureCookies reports whether the session cookie must carry the Secure flag.
func (c Config) SecureCookies() bool {
	switch c.CookieSecure {
	case CookieSecureTrue:
		return true
	case CookieSecureFalse:
		return false
	default:
		return strings.HasPrefix(c.BaseURL, "https://")
	}
}

// Load reads the configuration using getenv (os.Getenv when nil).
func Load(getenv func(string) string) (Config, error) {
	if getenv == nil {
		getenv = os.Getenv
	}
	get := func(key, def string) string {
		if v := strings.TrimSpace(getenv(key)); v != "" {
			return v
		}
		return def
	}

	cfg := Config{
		DataDir:    get("TORQ_DATA_DIR", "/data"),
		ListenAddr: get("TORQ_LISTEN_ADDR", ":8080"),
		LogLevel:   strings.ToLower(get("TORQ_LOG_LEVEL", "info")),
		LogFormat:  strings.ToLower(get("TORQ_LOG_FORMAT", "json")),
	}

	if raw := get("TORQ_BASE_URL", ""); raw != "" {
		u, err := url.Parse(raw)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return Config{}, fmt.Errorf("TORQ_BASE_URL must be an absolute http(s) URL, got %q", raw)
		}
		cfg.BaseURL = strings.TrimRight(raw, "/")
	}

	switch cs := CookieSecure(strings.ToLower(get("TORQ_COOKIE_SECURE", "auto"))); cs {
	case CookieSecureAuto, CookieSecureTrue, CookieSecureFalse:
		cfg.CookieSecure = cs
	default:
		return Config{}, fmt.Errorf("TORQ_COOKIE_SECURE must be auto, true or false, got %q", cs)
	}

	if raw := get("TORQ_TRUSTED_PROXIES", ""); raw != "" {
		for _, part := range strings.Split(raw, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			p, err := netip.ParsePrefix(part)
			if err != nil {
				return Config{}, fmt.Errorf("TORQ_TRUSTED_PROXIES: invalid CIDR %q: %w", part, err)
			}
			cfg.TrustedProxies = append(cfg.TrustedProxies, p.Masked())
		}
	}

	mb, err := strconv.Atoi(get("TORQ_MAX_UPLOAD_MB", "25"))
	if err != nil || mb <= 0 {
		return Config{}, fmt.Errorf("TORQ_MAX_UPLOAD_MB must be a positive integer")
	}
	cfg.MaxUploadMB = mb

	switch v := strings.ToLower(get("TORQ_CATALOG_ONLINE_UPDATES", "true")); v {
	case "true", "false":
		cfg.CatalogOnlineUpdates = v == "true"
	default:
		return Config{}, fmt.Errorf("TORQ_CATALOG_ONLINE_UPDATES must be true or false, got %q", v)
	}

	switch cfg.LogLevel {
	case "debug", "info", "warn", "error":
	default:
		return Config{}, fmt.Errorf("TORQ_LOG_LEVEL must be debug, info, warn or error, got %q", cfg.LogLevel)
	}
	switch cfg.LogFormat {
	case "json", "text":
	default:
		return Config{}, fmt.Errorf("TORQ_LOG_FORMAT must be json or text, got %q", cfg.LogFormat)
	}
	return cfg, nil
}
