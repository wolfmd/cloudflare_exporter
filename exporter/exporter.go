package main

import (
	"gopkg.in/alecthomas/kingpin.v2"
	"log"
)

var (
	CLOUDFLARE_KEY              = EnvString("CLOUDFLARE_KEY", "")              // (optional) key used for Cloudflare API email authentication
	CLOUDFLARE_TOKEN            = EnvString("CLOUDFLARE_TOKEN", "")            // (optional) token used for Cloudflare API token authentication
	CLOUDFLARE_USER_SERVICE_KEY = EnvString("CLOUDFLARE_USER_SERVICE_KEY", "") // (optional) key used for Cloudflare API user service key authentication

	cloudflareEmail         = kingpin.Flag("cloudflare.email", "email used for Cloudflare API email authentication, env: CLOUDFLARE_EMAIL").Default(EnvString("CLOUDFLARE_EMAIL", "")).String()
	cloudflareZones         = kingpin.Flag("cloudflare.zones", "(required) comma-separated list of zone names to scrape for metrics (e.g. 'example.com,example.org'), env: CLOUDFLARE_ZONES").Default(EnvString("CLOUDFLARE_ZONES", "")).String()
	cloudflareAccounts      = kingpin.Flag("cloudflare.accounts", "comma-separated list of account ids to scrape for metrics (e.g. '123548648,123548644868'), env: CLOUDFLARE_ACCOUNTS").Default(EnvString("CLOUDFLARE_ACCOUNTS", "")).String()
	cloudflareSince         = kingpin.Flag("cloudflare.since", "`since` parameter of calls to the Cloudflare Analytics API ('Free' tenants have a minimum of 24h), env: CLOUDFLARE_SCRAPE_ANALYTICS_SINCE").Default(EnvString("CLOUDFLARE_SCRAPE_ANALYTICS_SINCE", "24h")).String()
	cloudflareIncludeAccess = kingpin.Flag("cloudflare.include-access", "bool to enable access-related metrics").Default("false").Bool()
	exporterListenAddr      = kingpin.Flag("web.listen-addr", "address for the exporter to bind to, env: EXPORTER_LISTEN_ADDR").Default(EnvString("EXPORTER_LISTEN_ADDR", "127.0.0.1:9199")).String()
	cloudflare_metrics      *CloudflareMetrics
)

func main() {
	var err error
	kingpin.HelpFlag.Short('h')
	kingpin.Parse()
	config := ExporterConfig{
		*cloudflareEmail,
		CLOUDFLARE_KEY,
		CLOUDFLARE_TOKEN,
		CLOUDFLARE_USER_SERVICE_KEY,
		*cloudflareZones,
		*cloudflareAccounts,
		*cloudflareSince,
		*cloudflareIncludeAccess,
	}
	cloudflare_metrics, err = New(config)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("serving metrics at http://%v/metrics\n", *exporterListenAddr)
	log.Fatal(ListenAndServe(*exporterListenAddr))
}
