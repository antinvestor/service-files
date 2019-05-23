package utils

import (
	"os"

	"github.com/Sirupsen/logrus"
	"github.com/jinzhu/gorm"

	// Gorm relies on this dialect for initialization
	_ "github.com/jinzhu/gorm/dialects/postgres"
	otgorm "github.com/smacker/opentracing-gorm"
)

// ConfigureDatabase Database Access for environment is configured here
func ConfigureDatabase(log *logrus.Entry) (*gorm.DB, error) {

	dbDatasource := os.Getenv("DATABASE_URL")
	dbDriver := os.Getenv("DATABASE_DRIVER")

	log.Debugf("Connecting using driver %v and source %v ", dbDriver, dbDatasource)

	db, err := gorm.Open(dbDriver, dbDatasource)
	if err != nil {
		log.Warningf("Problem experienced while obtaining the database link :  %v ", err)
	}

	otgorm.AddGormCallbacks(db)

	return db, err
}
