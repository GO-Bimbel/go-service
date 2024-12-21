package configs

import (
	"fmt"
	"os"
)

type Config struct {
	DBHost          string
	DBPort          string
	DBUser          string
	DBPass          string
	DBName          string
	SSLMode         string
	DBHostKBM       string
	DBPortKBM       string
	DBUserKBM       string
	DBPassKBM       string
	DBNameKBM       string
	SSLModeKBM      string
	DBHostTOBK      string
	DBPortTOBK      string
	DBUserTOBK      string
	DBPassTOBK      string
	DBNameTOBK      string
	SSLModeTOBK     string
	ServerPort      string
	SetMaxIdleConns string
	SetMaxOpenConns string
	RangeDay        string
}

func LoadConfig() *Config {
	config := &Config{}
	envVars := map[string]*string{
		"DB_HOST": &config.DBHost,
		"DB_PORT": &config.DBPort,
		"DB_USER": &config.DBUser,
		"DB_PASS": &config.DBPass,
		"DB_NAME": &config.DBName,
		"SSLMODE": &config.SSLMode,

		"DB_HOST_KBM": &config.DBHostKBM,
		"DB_PORT_KBM": &config.DBPortKBM,
		"DB_USER_KBM": &config.DBUserKBM,
		"DB_PASS_KBM": &config.DBPassKBM,
		"DB_NAME_KBM": &config.DBNameKBM,
		"SSLMODE_KBM": &config.SSLModeKBM,

		"DB_HOST_TOBK_EKS": &config.DBHostTOBK,
		"DB_PORT_TOBK_EKS": &config.DBPortTOBK,
		"DB_USER_TOBK_EKS": &config.DBUserTOBK,
		"DB_PASS_TOBK_EKS": &config.DBPassTOBK,
		"DB_NAME_TOBK_EKS": &config.DBNameTOBK,
		"SSLMODE_TOBK_EKS": &config.SSLModeTOBK,

		"RANGE_DAY": &config.RangeDay,
	}

	for key, ptr := range envVars {
		value := os.Getenv(key)
		if value == "" {
			fmt.Println("Missing environment variable: " + key)
		}
		*ptr = value
	}

	if config.RangeDay == "" {
		config.RangeDay = "1"
	}

	return config
}
