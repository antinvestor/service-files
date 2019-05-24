package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/gorilla/handlers"

	"bitbucket.org/antinvestor/service-file/openapi"
	"bitbucket.org/antinvestor/service-file/service"
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

		service.PerformMigration(logger, database)

	} else {
		logger.Info("Initiating the file service")

		router := openapi.NewRouterV1(database, logger)

		port := fmt.Sprintf(":%s", os.Getenv("PORT"))

		logger.Fatal(http.ListenAndServe(port, handlers.RecoveryHandler()(router)))
	}

}
