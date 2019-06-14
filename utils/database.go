package utils

import (
	"github.com/Sirupsen/logrus"
	"github.com/jinzhu/gorm"

	// Gorm relies on this dialect for initialization
	_ "github.com/jinzhu/gorm/dialects/postgres"
	"github.com/smacker/opentracing-gorm"
	"fmt"
	"time"
)

// ConfigureDatabase Database Access for environment is configured here
func ConfigureDatabase(log *logrus.Entry) (*gorm.DB, error) {

	dbDriver := GetEnv("DATABASE_DRIVER","postgres")

	dbDatasource := GetEnv("DATABASE_URL", "")
	if(dbDatasource == ""){

		dbHost := GetEnv("DATABASE_HOST", "127.0.0.1")
		dbName := GetEnv("DATABASE_NAME", "service-file")
		dbUserName := GetEnv("DATABASE_USER_NAME", "file")
		dbSecret := GetEnv("DATABASE_SECRET", "files")
		dbPort := GetEnv("DATABASE_PORT", "5432")

		dbDatasource = fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s  sslmode=require", dbHost, dbPort, dbUserName, dbSecret, dbName)
	}

	log.Debugf("Connecting using driver %v and source %v ", dbDriver, dbDatasource)

	db, err := gorm.Open(dbDriver, dbDatasource)
	if err != nil {
		log.Warning(err)
		log.Debugf("Connection details include : %s", dbDatasource)
		log.Info("Retrying to reconnect in 5 seconds")

		time.Sleep(5 * time.Second)

		return ConfigureDatabase(log)
	}

	otgorm.AddGormCallbacks(db)

	return db, err
}
