package config_test

import (
	"strings"
	"testing"
	"time"

	"reschedule/internal/config"
)

type Inner struct {
	Name string `env:"TEST_CFG_NAME" default:"inner-default"`
}

type testConfig struct {
	Port    int           `env:"TEST_CFG_PORT" default:"8080"`
	Timeout time.Duration `env:"TEST_CFG_TIMEOUT" default:"5s"`
	Debug   bool          `env:"TEST_CFG_DEBUG" default:"false"`
	*Inner
}

func clearEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{"TEST_CFG_PORT", "TEST_CFG_TIMEOUT", "TEST_CFG_DEBUG", "TEST_CFG_NAME"} {
		t.Setenv(k, "")
	}
}

func TestParseConfig(t *testing.T) {
	t.Run("default applied", func(t *testing.T) {
		clearEnv(t)

		var cfg testConfig
		if errs := config.ParseConfig(&cfg); len(errs) != 0 {
			t.Fatalf("ParseConfig() errors = %v, want none", errs)
		}
		if cfg.Port != 8080 {
			t.Errorf("Port = %d, want 8080", cfg.Port)
		}
		if cfg.Timeout != 5*time.Second {
			t.Errorf("Timeout = %v, want 5s", cfg.Timeout)
		}
		if cfg.Debug {
			t.Errorf("Debug = true, want false")
		}
	})

	t.Run("env overrides default", func(t *testing.T) {
		clearEnv(t)
		t.Setenv("TEST_CFG_PORT", "9090")
		t.Setenv("TEST_CFG_TIMEOUT", "250ms")
		t.Setenv("TEST_CFG_DEBUG", "true")

		var cfg testConfig
		if errs := config.ParseConfig(&cfg); len(errs) != 0 {
			t.Fatalf("ParseConfig() errors = %v, want none", errs)
		}
		if cfg.Port != 9090 {
			t.Errorf("Port = %d, want 9090", cfg.Port)
		}
		if cfg.Timeout != 250*time.Millisecond {
			t.Errorf("Timeout = %v, want 250ms", cfg.Timeout)
		}
		if !cfg.Debug {
			t.Errorf("Debug = false, want true")
		}
	})

	t.Run("invalid value returns error", func(t *testing.T) {
		clearEnv(t)
		t.Setenv("TEST_CFG_PORT", "not-a-number")

		var cfg testConfig
		errs := config.ParseConfig(&cfg)
		if len(errs) != 1 {
			t.Fatalf("ParseConfig() errors = %v, want exactly one", errs)
		}
		if !strings.Contains(errs[0].Error(), "TEST_CFG_PORT") {
			t.Errorf("error = %q, want it to mention TEST_CFG_PORT", errs[0])
		}
		if cfg.Timeout != 5*time.Second {
			t.Errorf("Timeout = %v, want 5s (other fields must still be parsed)", cfg.Timeout)
		}
	})

	t.Run("nested pointer is populated", func(t *testing.T) {
		clearEnv(t)

		var cfg testConfig
		if errs := config.ParseConfig(&cfg); len(errs) != 0 {
			t.Fatalf("ParseConfig() errors = %v, want none", errs)
		}
		if cfg.Inner == nil {
			t.Fatal("Inner = nil, want allocated")
		}
		if cfg.Name != "inner-default" {
			t.Errorf("Name = %q, want %q", cfg.Name, "inner-default")
		}

		t.Setenv("TEST_CFG_NAME", "from-env")
		cfg = testConfig{}
		if errs := config.ParseConfig(&cfg); len(errs) != 0 {
			t.Fatalf("ParseConfig() errors = %v, want none", errs)
		}
		if cfg.Name != "from-env" {
			t.Errorf("Name = %q, want %q", cfg.Name, "from-env")
		}
	})
}

type requiredConfig struct {
	Host    string        `env:"TEST_CFG_HOST"`
	Token   string        `env:"TEST_CFG_TOKEN" optional:"true"`
	Timeout time.Duration `env:"TEST_CFG_TIMEOUT" optional:"true"`
}

func TestParseConfigRequired(t *testing.T) {
	t.Run("missing required returns error", func(t *testing.T) {
		t.Setenv("TEST_CFG_HOST", "")
		t.Setenv("TEST_CFG_TOKEN", "")
		t.Setenv("TEST_CFG_TIMEOUT", "")

		var cfg requiredConfig
		errs := config.ParseConfig(&cfg)
		if len(errs) != 1 {
			t.Fatalf("ParseConfig() errors = %v, want exactly one", errs)
		}
		if !strings.Contains(errs[0].Error(), "TEST_CFG_HOST") {
			t.Errorf("error = %q, want it to mention TEST_CFG_HOST", errs[0])
		}
		if cfg.Token != "" || cfg.Timeout != 0 {
			t.Errorf("optional fields = %q, %v, want zero values", cfg.Token, cfg.Timeout)
		}
	})

	t.Run("invalid duration returns error", func(t *testing.T) {
		t.Setenv("TEST_CFG_HOST", "localhost")
		t.Setenv("TEST_CFG_TOKEN", "")
		t.Setenv("TEST_CFG_TIMEOUT", "5")

		var cfg requiredConfig
		errs := config.ParseConfig(&cfg)
		if len(errs) != 1 {
			t.Fatalf("ParseConfig() errors = %v, want exactly one", errs)
		}
		if !strings.Contains(errs[0].Error(), "TEST_CFG_TIMEOUT") {
			t.Errorf("error = %q, want it to mention TEST_CFG_TIMEOUT", errs[0])
		}
	})
}

func TestDBConfigDSN(t *testing.T) {
	d := config.DBConfig{Host: "db", Port: 5433, Name: "app", User: "u", Password: "p"}

	want := "host=db user=u password=p dbname=app port=5433 sslmode=disable"
	if got := d.DSN(); got != want {
		t.Errorf("DSN() = %q, want %q", got, want)
	}
}
