package platform

import "os"

// Config is read from the environment.
type Config struct {
	Addr        string // HEKATE_ADDR
	DatabaseURL string // HEKATE_DATABASE_URL (owner role; the pool switches to hekate_app)
	JWKSURL     string // HEKATE_JWKS_URL (Keycloak certs endpoint)
	Issuer      string // HEKATE_ISSUER (optional exact match)
	Audience    string // HEKATE_AUDIENCE (optional)
	DevSecret   string // HEKATE_DEV_JWT_SECRET: enables HS256 tokens. Never set in production.
	CORSOrigins string // HEKATE_CORS_ORIGINS, comma separated
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
	}
}
