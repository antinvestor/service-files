package main

import (
	"os"

	"github.com/antinvestor/files/service"
	"github.com/antinvestor/files/service/storage"
	"github.com/antinvestor/files/utils"
	"time"
)

func main() {

	serviceName := "files"

	logger, err := utils.ConfigureLogging(serviceName)
	if err != nil {
		println("Failed to configure logging: " + err.Error())
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
