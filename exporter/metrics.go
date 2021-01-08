package main

import (
	"errors"
	"log"
	"strings"
	"time"

	cloudflare "github.com/cloudflare/cloudflare-go"
	prometheus "github.com/prometheus/client_golang/prometheus"
)

type CloudflareMetrics struct {
	api           *cloudflare.API
	zones         []string
	accounts      []string
	since         string
	includeAccess bool

	counters   map[string]*prometheus.CounterVec
	gauges     map[string]*prometheus.GaugeVec
	histograms map[string]*prometheus.HistogramVec
	summaries  map[string]*prometheus.SummaryVec
}

var (
	errNoCloudflareAuth     = errors.New("no Cloudflare authentication method provided")
	errNoCloudflareZones    = errors.New("no Cloudflare zones provided, please set CLOUDFLARE_ZONES or pass in --cloudflare.zones")
	errNoCloudflareAccounts = errors.New("no Cloudflare accounts provided, needed in order to display access-related metrics, please set CLOUDFLARE_ACCOUNTS or pass in --cloudflare.accounts")
)

func New(config ExporterConfig) (*CloudflareMetrics, error) {
	if config.cloudflareZones == "" {
		return nil, errNoCloudflareZones
	}

	if config.cloudflareIncludeAccess == true && config.cloudflareAccounts == "" {
		return nil, errNoCloudflareAccounts
	}

	if config.cloudflareEmail != "" && config.cloudflareKey != "" {
		return newWithEmailAuth(config)
	}

	if config.cloudflareToken != "" {
		return newWithTokenAuth(config)
	}

	if config.cloudflareUserServiceKey != "" {
		return newWithUserServiceKeyAuth(config)
	}

	return nil, errNoCloudflareAuth
}

func newWithEmailAuth(config ExporterConfig) (*CloudflareMetrics, error) {
	cloudflareApi, err := cloudflare.New(config.cloudflareKey, config.cloudflareEmail)
	if err != nil {
		return nil, err
	}

	return newWithAPI(cloudflareApi, config), nil
}

func newWithTokenAuth(config ExporterConfig) (*CloudflareMetrics, error) {
	cloudflareApi, err := cloudflare.NewWithAPIToken(config.cloudflareToken)
	if err != nil {
		return nil, err
	}

	return newWithAPI(cloudflareApi, config), nil
}

func newWithUserServiceKeyAuth(config ExporterConfig) (*CloudflareMetrics, error) {
	cloudflareApi, err := cloudflare.NewWithUserServiceKey(config.cloudflareUserServiceKey)
	if err != nil {
		return nil, err
	}

	return newWithAPI(cloudflareApi, config), nil
}

func newWithAPI(cloudflareApi *cloudflare.API, config ExporterConfig) *CloudflareMetrics {
	return &CloudflareMetrics{
		api:           cloudflareApi,
		zones:         strings.Split(strings.ReplaceAll(config.cloudflareZones, " ", ""), ","),
		accounts:      strings.Split(strings.ReplaceAll(config.cloudflareAccounts, " ", ""), ","),
		since:         config.cloudflareSince,
		includeAccess: config.cloudflareIncludeAccess,
		counters:      map[string]*prometheus.CounterVec{},
		gauges:        map[string]*prometheus.GaugeVec{},
		histograms:    map[string]*prometheus.HistogramVec{},
		summaries:     map[string]*prometheus.SummaryVec{},
	}
}

func (cm *CloudflareMetrics) update() {
	if cm.includeAccess == true {
		for _, account := range cm.accounts {
			cm.updateAccount(account)
		}
	}
	for _, zone := range cm.zones {
		cm.updateZone(zone)
	}
}

func (cm *CloudflareMetrics) updateAccount(accountId string) {
	serviceTokenExpirationMap := make(map[string]int64)

	accessServiceTokens, _, err := cm.api.AccessServiceTokens(accountId)
	if err != nil {
		log.Printf("cloudflare.API.AccessServiceTokens(%v): %v\n", accountId, err)
		return
	}
	for _, data := range accessServiceTokens {
		serviceTokenExpirationMap[data.Name] = data.ExpiresAt.Unix()
	}

	cm.updateAccountGaugeByLabel(accountId, "access_service_token_expiration", "The current unix timestamp at which a service token expires", "token_name", serviceTokenExpirationMap)
}

func (cm *CloudflareMetrics) updateZone(zoneName string) {
	zoneId, err := cm.api.ZoneIDByName(zoneName)
	if err != nil {
		log.Printf("cloudflare.API.ZoneIDByName(%v): %v\n", zoneName, err)
		return
	}

	duration := "-" + cm.since
	since, err := time.ParseDuration(duration)
	if err != nil {
		log.Printf("time.ParseDuration(%v): %v\n", duration, err)
		return
	}

	optionSince := time.Now().Add(since)
	optionContinuous := false
	data, err := cm.api.ZoneAnalyticsDashboard(zoneId, cloudflare.ZoneAnalyticsOptions{Since: &optionSince, Continuous: &optionContinuous})
	if err != nil {
		log.Printf("cloudflare.API.ZoneAnalyticsDashboard(%v): %v\n", zoneId, err)
		return
	}

	cm.updateZoneGauge(zoneId, zoneName, "cloudflare_requests_rate"+cm.since, "Total number of requests over the last 24h", data.Totals.Requests.All)
	cm.updateZoneGauge(zoneId, zoneName, "cloudflare_requests_cached_rate"+cm.since, "Total number of cached requests over the last 24h", data.Totals.Requests.Cached)
	cm.updateZoneGauge(zoneId, zoneName, "cloudflare_requests_uncached_rate"+cm.since, "Total number of uncached requests over the last 24h", data.Totals.Requests.Uncached)
	cm.updateZoneGaugeByLabel(zoneId, zoneName, "cloudflare_requests_content_type_rate"+cm.since, "Total number of requests over the last 24h by response Content-Type header", "content_type", data.Totals.Requests.ContentType)
	cm.updateZoneGaugeByLabel(zoneId, zoneName, "cloudflare_requests_country_rate"+cm.since, "Total number of requests over the last 24h by request country", "country", data.Totals.Requests.Country)
	cm.updateZoneGauge(zoneId, zoneName, "cloudflare_requests_encrypted_rate"+cm.since, "Total number of encrypted requests over the last 24h", data.Totals.Requests.SSL.Encrypted)
	cm.updateZoneGauge(zoneId, zoneName, "cloudflare_requests_unencrypted_rate"+cm.since, "Total number of unencrypted requests over the last 24h", data.Totals.Requests.SSL.Unencrypted)
	cm.updateZoneGaugeByLabel(zoneId, zoneName, "cloudflare_requests_status_rate"+cm.since, "Total number of requests over the last 24h by response code", "status", data.Totals.Requests.HTTPStatus)

	cm.updateZoneGauge(zoneId, zoneName, "cloudflare_bandwidth_bytes_rate"+cm.since, "Total bandwidth over the last 24h", data.Totals.Bandwidth.All)
	cm.updateZoneGauge(zoneId, zoneName, "cloudflare_bandwidth_cached_bytes_rate"+cm.since, "Total cached bandwidth over the last 24h", data.Totals.Bandwidth.Cached)
	cm.updateZoneGauge(zoneId, zoneName, "cloudflare_bandwidth_uncached_bytes_rate"+cm.since, "Total uncached bandwidth over the last 24h", data.Totals.Bandwidth.Uncached)
	cm.updateZoneGaugeByLabel(zoneId, zoneName, "cloudflare_bandwidth_content_type_bytes_rate"+cm.since, "Total bandwidth over the last 24h by response Content-Type header", "content_type", data.Totals.Bandwidth.ContentType)
	cm.updateZoneGaugeByLabel(zoneId, zoneName, "cloudflare_bandwidth_country_bytes_rate"+cm.since, "Total bandwidth over the last 24h by request country", "country", data.Totals.Bandwidth.Country)
	cm.updateZoneGauge(zoneId, zoneName, "cloudflare_bandwidth_encrypted_bytes_rate"+cm.since, "Total encrypted bandwidth over the last 24h", data.Totals.Bandwidth.SSL.Encrypted)
	cm.updateZoneGauge(zoneId, zoneName, "cloudflare_bandwidth_unencrypted_bytes_rate"+cm.since, "Total unencrypted bandwidth over the last 24h", data.Totals.Bandwidth.SSL.Unencrypted)

	cm.updateZoneGauge(zoneId, zoneName, "cloudflare_threats_rate"+cm.since, "Total mitigated threats over the last 24h", data.Totals.Threats.All)
	cm.updateZoneGaugeByLabel(zoneId, zoneName, "cloudflare_threats_country_rate"+cm.since, "Total mitigated threats over the last 24h by request country", "country", data.Totals.Threats.Country)
	cm.updateZoneGaugeByLabel(zoneId, zoneName, "cloudflare_threats_type_rate"+cm.since, "Total mitigated threats over the last 24h by type", "type", data.Totals.Threats.Type)

	cm.updateZoneGauge(zoneId, zoneName, "cloudflare_pageviews_rate"+cm.since, "Total page views over the last 24h", data.Totals.Pageviews.All)
	cm.updateZoneGaugeByLabel(zoneId, zoneName, "cloudflare_pageviews_search_engine_rate"+cm.since, "Total page views over the last 24h by search engine", "search_engine", data.Totals.Pageviews.SearchEngines)

	cm.updateZoneGauge(zoneId, zoneName, "cloudflare_uniques_rate"+cm.since, "Total unique visitors over the last 24h", data.Totals.Uniques.All)
}

func (cm *CloudflareMetrics) updateZoneGauge(zoneId string, zoneName string, name string, help string, value int) {
	labels := prometheus.Labels{"zone_id": zoneId, "zone_name": zoneName}
	cm.createGaugeIfNotExists(name, help, labels)
	cm.gauges[name].With(labels).Set(float64(value))
}

func (cm *CloudflareMetrics) updateZoneGaugeByLabel(zoneId string, zoneName string, name string, help string, byLabel string, values map[string]int) {
	labels := prometheus.Labels{"zone_id": zoneId, "zone_name": zoneName, byLabel: ""}
	cm.createGaugeIfNotExists(name, help, labels)
	for key, value := range values {
		labels[byLabel] = key
		cm.gauges[name].With(labels).Set(float64(value))
	}
}

func (cm *CloudflareMetrics) updateAccountGaugeByLabel(accountId string, name string, help string, byLabel string, values map[string]int64) {
	labels := prometheus.Labels{"account_id": accountId, byLabel: ""}
	cm.createGaugeIfNotExists(name, help, labels)
	for key, value := range values {
		labels[byLabel] = key
		cm.gauges[name].With(labels).Set(float64(value))
	}
}

func (cm *CloudflareMetrics) createGaugeIfNotExists(name string, help string, labels prometheus.Labels) {
	if _, ok := cm.gauges[name]; !ok {
		label_names := make([]string, len(labels))
		i := 0
		for label := range labels {
			label_names[i] = label
			i++
		}

		cm.gauges[name] = prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: name, Help: help}, label_names)
		prometheus.MustRegister(cm.gauges[name])
	}
}
