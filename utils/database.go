package utils

import (
	"fmt"
	// Gorm relies on this dialect for initialization
	"github.com/Sirupsen/logrus"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/postgres"
	otgorm "github.com/smacker/opentracing-gorm"
)

// ConfigureDatabase Database Access for environment is configured here
func ConfigureDatabase(log *logrus.Entry, replica bool) (*gorm.DB, error) {

	dbDriver := GetEnv("DATABASE_DRIVER","postgres")

	dbDatasource := GetEnv("DATABASE_URL", "")
	if dbDatasource == "" {

		dbHost := GetEnv("DATABASE_HOST", "127.0.0.1")
		if replica{
			dbHost = GetEnv("REPLICA_DATABASE_HOST", dbHost)
		}


		dbName := GetEnv("DATABASE_NAME", "service-file")
		dbUserName := GetEnv("DATABASE_USER_NAME", "file")
		dbSecret := GetEnv("DATABASE_SECRET", "files")
		dbPort := GetEnv("DATABASE_PORT", "5432")

		dbDatasource = fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s  sslmode=require", dbHost, dbPort, dbUserName, dbSecret, dbName)
	}

	log.Debugf("Connecting using driver %v and source %v ", dbDriver, dbDatasource)

	db, err := gorm.Open(dbDriver, dbDatasource)

	if db != nil {
		otgorm.AddGormCallbacks(db)
	}
	return db, err
}
