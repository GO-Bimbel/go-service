package main

import (
	"scheduler/configs"
	"scheduler/database"
	"scheduler/handler"

	log "github.com/sirupsen/logrus"

	"github.com/joho/godotenv"
)

func main2() {
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

	database.ConnectTobkDatabase(config)

	if err != nil {
		log.Fatal(err)
	}

	handler.FetchDetilJawabanH(handler.QueryParams{}, database.DB)

}
