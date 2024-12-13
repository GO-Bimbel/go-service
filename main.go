package main

import (
	"scheduler/configs"
	"scheduler/database"
	"scheduler/handler"
	"scheduler/utils"

	log "github.com/sirupsen/logrus"

	"github.com/joho/godotenv"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Errorf("Error loading .env file")
	}

	// Load configuration
	config := configs.LoadConfig()

	// Connect to database
	database.ConnectDatabase(config)

	// Connect to KBM database
	database.ConnectKBMDatabase(config)

	rd, err := utils.ConvertStringToInt(config.RangeDay)
	if err != nil {
		log.Fatal(err)
	}

	// Rencana kerja
	handler.RencanaKerja(rd)

}
