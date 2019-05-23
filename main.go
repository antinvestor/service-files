package main

import (
	"log"
	"os"

	"bitbucket.org/antinvestor/service-file/utils"
)

func main() {

	serviceName := "file"

	logger, err := utils.ConfigureLogging(serviceName)
	if err != nil {
		log.Fatal("Failed to configure logging: " + err.Error())
	}

	closer, err := utils.ConfigureJuegler(serviceName)
	if err != nil {
		logger.Fatal("Failed to configure Juegler: " + err.Error())
	}

	defer closer.Close()

	database, err := utils.ConfigureDatabase(logger)
	if err != nil {
		logger.Fatalf("Failed to configure Database: %v", err)
	}

	stdArgs := os.Args[1:]
	if len(stdArgs) > 0 && stdArgs[0] == "migrate" {
		logger.Info("Initiating migrations")

		utils.PerformMigration(logger, database)

	} else {
		logger.Info("Initiating the file service")
	}

}
