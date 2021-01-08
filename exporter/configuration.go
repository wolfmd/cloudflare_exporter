package main

import (
	"os"
)

func EnvString(key string, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

type ExporterConfig struct {
	cloudflareEmail          string
	cloudflareKey            string
	cloudflareToken          string
	cloudflareUserServiceKey string
	cloudflareZones          string
	cloudflareAccounts       string
	cloudflareSince          string
	cloudflareIncludeAccess  bool
}
