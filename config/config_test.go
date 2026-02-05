package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAppConfigFields(t *testing.T) {
	fields := AppConfigFields()
	assert.Len(t, fields, 12, "AppConfig should have 12 fields")

	assert.Equal(t, "LISTEN_ADDR", fields[0].Env)
	assert.Equal(t, "REGISTRY_DIR", fields[1].Env)
	assert.Equal(t, "DEFAULT_MODEL", fields[2].Env)
	assert.Equal(t, "DEFAULT_PROVIDER", fields[3].Env)
	assert.Equal(t, "DEFAULT_BASE_URL", fields[4].Env)
	assert.Equal(t, "DEFAULT_TEMPERATURE", fields[5].Env)
	assert.Equal(t, "DEFAULT_MAX_TOKENS", fields[6].Env)
	assert.Equal(t, "DEFAULT_API_KEY", fields[7].Env)

	assert.Equal(t, "string", fields[0].Type)
	assert.Equal(t, "Address to bind HTTP server to", fields[0].Desc)
	assert.Equal(t, ":8080", fields[0].Default)

	assert.Equal(t, "claude-3-5-sonnet-20240620", fields[2].Default)
	assert.Equal(t, "APP_VARS", fields[8].Env)
	assert.Equal(t, "map", fields[8].Type)
	assert.Equal(t, "App variables from APP_* env vars", fields[8].Desc)
}

func TestAppVariables_Nested(t *testing.T) {
	os.Clearenv()
	os.Setenv("APP_NESTED_FOO", "bar")
	cfg := &AppConfig{}
	assert.NoError(t, LoadAppVariables(cfg))
	assert.Equal(t, "bar", cfg.Variables["NESTED_FOO"])
}

func TestAppVariables_Missing(t *testing.T) {
	os.Clearenv()
	cfg := &AppConfig{}
	assert.NoError(t, LoadAppVariables(cfg))
	assert.Empty(t, cfg.Variables) // No panic on missing
}

func TestLoadAppVariables(t *testing.T) {
	os.Clearenv()
	os.Setenv("APP_VAR1", "val1")
	os.Setenv("APP_FOO", "bar")
	os.Setenv("NOTAPP", "ignore")
	cfg := &AppConfig{}
	assert.NoError(t, LoadAppVariables(cfg))
	assert.Len(t, cfg.Variables, 2)
	assert.Equal(t, "val1", cfg.Variables["VAR1"])
	assert.Equal(t, "bar", cfg.Variables["FOO"])
}

func TestEnvironmentDerivation(t *testing.T) {
	os.Clearenv(); os.Setenv("APP_ENV", "prod")
	cfg := &AppConfig{}; err := LoadAppVariables(cfg); assert.NoError(t, err)
	assert.Equal(t, "prod", cfg.Environment)
	assert.Equal(t, "prod", cfg.Variables["ENV"])
}

func TestCompanyNameDerivation(t *testing.T) {
	os.Clearenv(); os.Setenv("APP_COMPANY_NAME", "Acme")
	cfg := &AppConfig{}; err := LoadAppVariables(cfg); assert.NoError(t, err)
	assert.Equal(t, "Acme", cfg.CompanyName)
	assert.Equal(t, "Acme", cfg.Variables["COMPANY_NAME"])
}

func TestMissingEnvironmentVariable(t *testing.T) {
	os.Clearenv(); cfg := &AppConfig{}; err := LoadAppVariables(cfg); assert.NoError(t, err)
	assert.Equal(t, "development", cfg.Environment)
}
