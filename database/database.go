package database

import (
	"scheduler/configs"

	"fmt"

	log "github.com/sirupsen/logrus"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var DB *gorm.DB
var DBKBM *gorm.DB

func ConnectDatabase(cfg *configs.Config) *gorm.DB {
	if cfg.SSLMode == "" {
		cfg.SSLMode = "prefer"
	}

	dsn := fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%s sslmode=%s",
		cfg.DBHost, cfg.DBUser, cfg.DBPass, cfg.DBName, cfg.DBPort, cfg.SSLMode,
	)

	var err error
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("Error connecting to database go: %v", err)
	}

	log.Info("Database GO connected")

	DB = db

	return db
}

func ConnectKBMDatabase(cfg *configs.Config) *gorm.DB {
	if cfg.SSLMode == "" {
		cfg.SSLMode = "prefer"
	}

	dsn := fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%s sslmode=%s",
		cfg.DBHostKBM, cfg.DBUserKBM, cfg.DBPassKBM, cfg.DBNameKBM, cfg.DBPortKBM, cfg.SSLModeKBM,
	)

	var err error
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("Error connecting to database kbm: %v", err)
	}

	log.Info("Database KBM connected")

	DBKBM = db

	return db
}
