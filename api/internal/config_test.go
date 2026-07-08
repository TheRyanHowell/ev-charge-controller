package internal

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfig_Defaults(t *testing.T) {
	keys := []string{"DB_PATH", "PORT", "VAPID_PUBLIC_KEY", "VAPID_PRIVATE_KEY", "LOG_LEVEL", "APP_ENV", "CORS_ORIGIN"}
	originals := make(map[string]string)
	for _, k := range keys {
		originals[k] = os.Getenv(k)
		os.Unsetenv(k)
	}
	t.Cleanup(func() {
		for k, v := range originals {
			if v == "" {
				os.Unsetenv(k)
			} else {
				os.Setenv(k, v)
			}
		}
	})

	cfg := LoadConfig()

	assert.Equal(t, "./ev-charge.db", cfg.DBPath)
	assert.Equal(t, "8080", cfg.Port)
	assert.Empty(t, cfg.VapidPublicKey)
	assert.Empty(t, cfg.VapidPrivateKey)
	assert.Equal(t, "prod", cfg.AppEnv)
	assert.Empty(t, cfg.CORSOrigin)
	assert.False(t, cfg.PushEnabled())
	assert.False(t, cfg.IsDev())
	assert.False(t, cfg.CarbonIntensityDisabled)
}

func TestLoadConfig_CarbonIntensityDisabled(t *testing.T) {
	t.Setenv("CARBON_INTENSITY_DISABLED", "true")
	assert.True(t, LoadConfig().CarbonIntensityDisabled)
}

func TestLoadConfig_FromEnv(t *testing.T) {
	keys := []string{"DB_PATH", "PORT", "VAPID_PUBLIC_KEY", "VAPID_PRIVATE_KEY", "LOG_LEVEL", "APP_ENV", "CORS_ORIGIN"}
	originals := make(map[string]string)
	for _, k := range keys {
		originals[k] = os.Getenv(k)
	}
	os.Setenv("DB_PATH", "/tmp/test.db")
	os.Setenv("PORT", "9090")
	os.Setenv("VAPID_PUBLIC_KEY", "pub")
	os.Setenv("VAPID_PRIVATE_KEY", "priv")
	os.Setenv("APP_ENV", "dev")
	os.Setenv("CORS_ORIGIN", "http://localhost:3000")
	t.Cleanup(func() {
		for k, v := range originals {
			if v == "" {
				os.Unsetenv(k)
			} else {
				os.Setenv(k, v)
			}
		}
	})

	cfg := LoadConfig()

	assert.Equal(t, "/tmp/test.db", cfg.DBPath)
	assert.Equal(t, "9090", cfg.Port)
	assert.Equal(t, "pub", cfg.VapidPublicKey)
	assert.Equal(t, "priv", cfg.VapidPrivateKey)
	assert.Equal(t, "dev", cfg.AppEnv)
	assert.Equal(t, "http://localhost:3000", cfg.CORSOrigin)
	assert.True(t, cfg.PushEnabled())
	assert.True(t, cfg.IsDev())
}

// validJWTSecret is a 32-byte (256-bit) secret that satisfies minJWTSecretBytes.
const validJWTSecret = "12345678901234567890123456789012"

func validConfig() *Config {
	return &Config{
		DBPath:    "/tmp/test.db",
		Port:      "8080",
		JWTSecret: validJWTSecret,
	}
}

func TestConfig_Validate_Valid(t *testing.T) {
	assert.NoError(t, validConfig().Validate())
}

func TestConfig_Validate_ReportsAllViolationsAtOnce(t *testing.T) {
	// Empty config: every required field is invalid; errors.Join must surface all.
	err := (&Config{}).Validate()
	require.Error(t, err)
	for _, want := range []string{"DB_PATH", "PORT", "JWT_SECRET"} {
		assert.Contains(t, err.Error(), want)
	}
}

func TestConfig_Validate_FieldRules(t *testing.T) {
	tests := map[string]struct {
		mutate func(*Config)
		want   string
	}{
		"non-numeric port":      {func(c *Config) { c.Port = "abc" }, "PORT"},
		"port out of range":     {func(c *Config) { c.Port = "70000" }, "PORT"},
		"cors invalid when set": {func(c *Config) { c.CORSOrigin = "not a url" }, "CORS_ORIGIN"},
		"vapid public only":     {func(c *Config) { c.VapidPublicKey = "pub" }, "VAPID"},
		"vapid private only":    {func(c *Config) { c.VapidPrivateKey = "priv" }, "VAPID"},
		"empty jwt secret":      {func(c *Config) { c.JWTSecret = "" }, "JWT_SECRET"},
		"short jwt secret":      {func(c *Config) { c.JWTSecret = "tooshort" }, "JWT_SECRET"},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			cfg := validConfig()
			tc.mutate(cfg)
			err := cfg.Validate()
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
		})
	}
}

func TestConfig_Validate_JWTSecret_ExactMinLength(t *testing.T) {
	cfg := validConfig()
	cfg.JWTSecret = string(make([]byte, minJWTSecretBytes))
	assert.NoError(t, cfg.Validate())
}

func TestConfig_Validate_JWTSecret_OneBelowMin(t *testing.T) {
	cfg := validConfig()
	cfg.JWTSecret = string(make([]byte, minJWTSecretBytes-1))
	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "JWT_SECRET")
}

func TestConfig_Validate_VapidPairOK(t *testing.T) {
	cfg := validConfig()
	cfg.VapidPublicKey = "pub"
	cfg.VapidPrivateKey = "priv"
	assert.NoError(t, cfg.Validate())
}
