package utils

import (
	"os"

	"github.com/Sirupsen/logrus"
	"github.com/jinzhu/gorm"

	// Gorm relies on this dialect for initialization
	_ "github.com/jinzhu/gorm/dialects/postgres"
	otgorm "github.com/smacker/opentracing-gorm"
	"fmt"
)

// ConfigureDatabase Database Access for environment is configured here
func ConfigureDatabase(log *logrus.Entry) (*gorm.DB, error) {

	dbDriver := os.Getenv("DATABASE_DRIVER")
	if dbDriver == "" {
		dbDriver = "postgres"
	}

	dbDatasource := os.Getenv("DATABASE_URL")
	if(dbDatasource == ""){

		dbHost := os.Getenv("DATABASE_HOST")
		dbName := os.Getenv("DATABASE_NAME")
		dbUserName := os.Getenv("DATABASE_USER_NAME")
		dbSecret := os.Getenv("DATABASE_SECRET")
		dbPort := os.Getenv("DATABASE_PORT")

		dbDatasource = fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s", dbHost, dbPort, dbUserName, dbSecret, dbName)

	}

	log.Debugf("Connecting using driver %v and source %v ", dbDriver, dbDatasource)

	db, err := gorm.Open(dbDriver, dbDatasource)
	if err != nil {
		log.Warningf("Problem experienced while obtaining the database link :  %v ", err)
	}

	otgorm.AddGormCallbacks(db)

	return db, err
}
