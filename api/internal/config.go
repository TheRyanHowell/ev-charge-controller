package internal

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"

	"ev-charge-controller/api/models"
)

// Config holds all application configuration loaded from environment variables.
type Config struct {
	// Database
	DBPath string

	// Server
	Port string

	// Push notifications (optional)
	VapidPublicKey  string
	VapidPrivateKey string

	// JWT auth
	JWTSecret string

	// MQTT client (optional - omit to disable MQTT)
	MQTTBrokerURL string
	MQTTUsername  string
	MQTTPassword  string

	// MQTT external endpoint shown to Tasmota devices on the LAN.
	// Separate from MQTTBrokerURL which is the internal Docker address.
	MQTTExternalIP   string
	MQTTExternalPort string

	// Environment
	AppEnv string

	// CORS
	CORSOrigin string

	// CarbonIntensityDisabled turns off the live carbon-intensity forecast client.
	// Used in CI/E2E, where hitting the real external API would make carbon-aware
	// schedule estimates and activation timing non-deterministic across runs.
	CarbonIntensityDisabled bool
}

// LoadConfig reads configuration from environment variables with defaults.
func LoadConfig() *Config {
	return &Config{
		DBPath:          envOrDefault("DB_PATH", models.DefaultDBPath),
		Port:            envOrDefault("PORT", "8080"),
		VapidPublicKey:      os.Getenv("VAPID_PUBLIC_KEY"),
		VapidPrivateKey:     os.Getenv("VAPID_PRIVATE_KEY"),
		AppEnv:                envOrDefault("APP_ENV", "prod"),
		CORSOrigin:            os.Getenv("CORS_ORIGIN"),
		JWTSecret:             os.Getenv("JWT_SECRET"),
		MQTTBrokerURL:         os.Getenv("MQTT_BROKER_URL"),
		MQTTUsername:          os.Getenv("MQTT_USERNAME"),
		MQTTPassword:          os.Getenv("MQTT_PASSWORD"),
		MQTTExternalIP:        os.Getenv("MQTT_IP"),
		MQTTExternalPort:   envOrDefault("MQTT_EXTERNAL_PORT", "1883"),
		CarbonIntensityDisabled: os.Getenv("CARBON_INTENSITY_DISABLED") == "true",
	}
}

// PushEnabled returns true if VAPID keys are configured.
func (c *Config) PushEnabled() bool {
	return c.VapidPublicKey != "" && c.VapidPrivateKey != ""
}

// IsDev returns true if running in development mode.
func (c *Config) IsDev() bool {
	return c.AppEnv == "dev"
}

// minJWTSecretBytes is the minimum acceptable length for JWT_SECRET.
// 32 bytes (256 bits) provides adequate HMAC-SHA256 security margin.
const minJWTSecretBytes = 32

// Validate checks all configuration values and reports every violation at once
// (via errors.Join) so a misconfiguration fails fast at startup rather than at
// the first request that happens to touch a bad field.
func (c *Config) Validate() error {
	var errs []error

	if c.DBPath == "" {
		errs = append(errs, errors.New("DB_PATH is required"))
	}
	if err := validatePort(c.Port); err != nil {
		errs = append(errs, err)
	}
	// CORS_ORIGIN is optional, but when set it must be a valid origin URL.
	if err := validateHTTPURL("CORS_ORIGIN", c.CORSOrigin, false); err != nil {
		errs = append(errs, err)
	}
	if err := validateVapidPair(c.VapidPublicKey, c.VapidPrivateKey); err != nil {
		errs = append(errs, err)
	}
	if len(c.JWTSecret) < minJWTSecretBytes {
		errs = append(errs, fmt.Errorf("JWT_SECRET must be at least %d bytes; generate one with: openssl rand -hex 32", minJWTSecretBytes))
	}

	return errors.Join(errs...)
}

// validatePort checks the listen port is present and a valid TCP port (1–65535).
func validatePort(port string) error {
	if port == "" {
		return errors.New("PORT is required")
	}
	n, err := strconv.Atoi(port)
	if err != nil || n < 1 || n > 65535 {
		return fmt.Errorf("PORT must be a number between 1 and 65535, got %q", port)
	}
	return nil
}

// validateHTTPURL checks that raw is an absolute http(s) URL with a host. When
// required is false an empty value is accepted (optional field).
func validateHTTPURL(name, raw string, required bool) error {
	if raw == "" {
		if required {
			return fmt.Errorf("%s is required", name)
		}
		return nil
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return fmt.Errorf("%s must be an absolute http(s) URL, got %q", name, raw)
	}
	return nil
}

// validateVapidPair enforces that VAPID keys are configured as a pair: either
// both present (push enabled) or both absent (push disabled), never one alone.
func validateVapidPair(public, private string) error {
	if (public == "") != (private == "") {
		return errors.New("VAPID_PUBLIC_KEY and VAPID_PRIVATE_KEY must be set together")
	}
	return nil
}

func envOrDefault(key, fallback string) string {
	val := os.Getenv(key)
	if val == "" {
		return fallback
	}
	return val
}
