// Package config loads application configuration from .env and environment variables.
//
// Struct tags:
//   - env:"NAME"       — environment variable name (required for non-pointer fields).
//   - optional:"true"  — allows the variable to be empty or missing.
//
// Supported field types: string, int, bool, and pointers to nested config structs
// (parsed recursively, no tags needed).
package config

import (
	"fmt"
	"log"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	ParserConfig *ParserConfig
}

type ParserConfig struct {
	URL string `env:"PARSE_URL"`
}

func LoadConfig() *Config {
	if err := godotenv.Load(".env"); err != nil {
		log.Fatalf("godotenv error: %s", err.Error())
	}

	conf := &Config{}
	errors := ParseConfig(conf)

	if len(errors) > 0 {
		printErrors(errors)
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
