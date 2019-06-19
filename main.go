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

	database, err := utils.ConfigureDatabase(logger, false)
	if err != nil {
		logger.Warnf("Configuring write database has error: %v", err)
	}
	defer database.Close()

	replicaDatabase, err := utils.ConfigureDatabase(logger, true)
	if err != nil {
		logger.Warnf("Configuring read only database has error: %v", err)
	}
	defer replicaDatabase.Close()

	stdArgs := os.Args[1:]
	if len(stdArgs) > 0 && stdArgs[0] == "migrate" {
		logger.Info("Initiating migrations")

		service.PerformMigration(logger, database)

	} else {
		logger.Infof("Initiating the file service at %v", time.Now())

		storageProvider := utils.GetEnv("STORAGE_PROVIDER", "LOCAL")

		env := service.Env{
			Logger:          logger,
			ServerPort: utils.GetEnv("SERVER_PORT", "7513"),
			EncryptionPhrase: utils.GetEnv("ENCRYPTION_PHRASE", "AES256Key-XihgT047PgfrbYZJB4Rf2K"),
			FileAccessServer: utils.GetEnv("FILE_ACCESS_SERVER_URL", ""),
			StrorageProvider: storage.GetStorageProvider(storageProvider),
		}
		env.SetWriteDb(database)
		env.SetReadDb(replicaDatabase)

		service.RunServer(&env)
	}

}
