// Package config loads application configuration from .env and environment variables.
// A missing .env file is not an error.
//
// Struct tags:
//   - env:"NAME"       — environment variable name (required for non-pointer fields).
//   - default:"value"  — used when the variable is empty or unset.
//   - optional:"true"  — leaves the field at its zero value instead of reporting an error.
//
// Supported field types: string, int, bool, time.Duration (e.g. "5s"), and pointers
// to nested config structs (parsed recursively, no tags needed; may be embedded).
package config

import (
	"errors"
	"fmt"
	"io/fs"
	"log"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	*DBConfig
	*ParserConfig
}

type ParserConfig struct {
	URL string `env:"PARSE_URL"`
}

type DBConfig struct {
	Host     string `env:"DB_HOST" default:"localhost"`
	Port     int    `env:"DB_PORT" default:"5432"`
	Name     string `env:"DB_NAME" default:"rescheduler"`
	Password string `env:"DB_PASSWORD" default:"postgres"`
	User     string `env:"DB_USER" default:"postgres"`
}

func (d *DBConfig) DSN() string {
	return fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%d sslmode=disable", d.Host, d.User, d.Password, d.Name, d.Port)
}

func LoadConfig() *Config {
	if err := godotenv.Load(".env"); err != nil && !errors.Is(err, fs.ErrNotExist) {
		log.Fatalf("load config: %v", err)
	}

	conf := &Config{}
	errs := ParseConfig(conf)

	if len(errs) > 0 {
		printErrors(errs)
	}

	return conf
}

func printErrors(errs []error) {
	var msg strings.Builder
	msg.WriteString("config errors:\n")
	for _, err := range errs {
		fmt.Fprintf(&msg, "  - %s\n", err.Error())
	}
	log.Fatal(msg.String())
}
