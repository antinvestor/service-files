package main

import (
	"log"
	"os"

	"bitbucket.org/antinvestor/service-file/service"
	"bitbucket.org/antinvestor/service-file/utils"
	"time"
	"bitbucket.org/antinvestor/service-file/service/storage"
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
		logger.Infof("Initiating the file service at %v", time.Now())

		storageProvider := os.Getenv("STORAGE_PROVIDER")

		env := service.Env{
			Logger:          logger,
			ServerPort: os.Getenv("SERVER_PORT"),
			EncryptionPhrase: os.Getenv("ENCRYPTION_PHRASE"),
			FileAccessServer: os.Getenv("FILE_ACCESS_SERVER_URL"),
			StrorageProvider: storage.GetStorageProvider(storageProvider),
		}
		env.SetDb(database)

		service.RunServer(&env)
	}

}
