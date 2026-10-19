package platform

import (
	"os"
	"strings"
)

// Config is read from the environment.
type Config struct {
	Addr        string   // HEKATE_ADDR
	DatabaseURL string   // HEKATE_DATABASE_URL (owner role; the pool switches to hekate_app)
	JWKSURL     string   // HEKATE_JWKS_URL (Keycloak certs endpoint)
	Issuer      string   // HEKATE_ISSUER (optional exact match)
	Audience    string   // HEKATE_AUDIENCE (optional)
	DevSecret   string   // HEKATE_DEV_JWT_SECRET: enables HS256 tokens. Never set in production.
	CORSOrigins string   // HEKATE_CORS_ORIGINS, comma separated
	NATSURL     string   // HEKATE_NATS_URL; empty logs events instead
	PortalURL   string   // HEKATE_PORTAL_URL: where the QR code on documents points (the web app)
	Letterhead  []string // HEKATE_LETTERHEAD: lines printed under the company name, separated by |
	PDFCompress bool     // HEKATE_PDF_COMPRESS: on unless set to 0 (tests turn it off to read text)
	PushEnabled bool     // HEKATE_PUSH=1 sends push notifications through the Expo push service
	ExpoToken   string   // HEKATE_EXPO_ACCESS_TOKEN, optional

	PaymentsWebhookSecret string // HEKATE_PAYMENTS_WEBHOOK_SECRET: signs provider callbacks (HMAC-SHA256); callbacks are refused without it
	PaymentsSandbox       bool   // HEKATE_PAYMENTS_SANDBOX=1: a fake provider that never moves money. Development only.
	MoMo                  MoMoConfig
}

// MoMoConfig is the MTN Mobile Money Collection API account (all empty: MoMo is off).
type MoMoConfig struct {
	BaseURL         string // HEKATE_MOMO_BASE_URL, e.g. https://proxy.momoapi.mtn.com
	SubscriptionKey string // HEKATE_MOMO_SUBSCRIPTION_KEY
	APIUser         string // HEKATE_MOMO_API_USER
	APIKey          string // HEKATE_MOMO_API_KEY
	Environment     string // HEKATE_MOMO_ENVIRONMENT: sandbox or the production target, for X-Target-Environment
	Currency        string // HEKATE_MOMO_CURRENCY, default XAF
	CallbackURL     string // HEKATE_MOMO_CALLBACK_URL, optional
}

func LoadConfig() Config {
	get := func(k, d string) string {
		if v := os.Getenv(k); v != "" {
			return v
		}
		return d
	}
	return Config{
		Addr:        get("HEKATE_ADDR", ":8080"),
		DatabaseURL: get("HEKATE_DATABASE_URL", "postgres://hekate:dev-only@localhost:5432/hekate?sslmode=disable"),
		JWKSURL:     os.Getenv("HEKATE_JWKS_URL"),
		Issuer:      os.Getenv("HEKATE_ISSUER"),
		Audience:    os.Getenv("HEKATE_AUDIENCE"),
		DevSecret:   os.Getenv("HEKATE_DEV_JWT_SECRET"),
		CORSOrigins: os.Getenv("HEKATE_CORS_ORIGINS"),
		NATSURL:     os.Getenv("HEKATE_NATS_URL"),
		PortalURL:   get("HEKATE_PORTAL_URL", "http://localhost:3000"),
		Letterhead:  splitLetterhead(os.Getenv("HEKATE_LETTERHEAD")),
		PDFCompress: os.Getenv("HEKATE_PDF_COMPRESS") != "0",
		PushEnabled: os.Getenv("HEKATE_PUSH") == "1",
		ExpoToken:   os.Getenv("HEKATE_EXPO_ACCESS_TOKEN"),

		PaymentsWebhookSecret: os.Getenv("HEKATE_PAYMENTS_WEBHOOK_SECRET"),
		PaymentsSandbox:       os.Getenv("HEKATE_PAYMENTS_SANDBOX") == "1",
		MoMo: MoMoConfig{
			BaseURL: os.Getenv("HEKATE_MOMO_BASE_URL"), SubscriptionKey: os.Getenv("HEKATE_MOMO_SUBSCRIPTION_KEY"),
			APIUser: os.Getenv("HEKATE_MOMO_API_USER"), APIKey: os.Getenv("HEKATE_MOMO_API_KEY"),
			Environment: os.Getenv("HEKATE_MOMO_ENVIRONMENT"), Currency: get("HEKATE_MOMO_CURRENCY", "XAF"),
			CallbackURL: os.Getenv("HEKATE_MOMO_CALLBACK_URL"),
		},
	}
}

func splitLetterhead(v string) []string {
	var out []string
	for _, l := range strings.Split(v, "|") {
		if l = strings.TrimSpace(l); l != "" {
			out = append(out, l)
		}
	}
	return out
}
