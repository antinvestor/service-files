package utils

import (
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/postgres"
	// Gorm relies on this dialect for initialization
	"github.com/sirupsen/logrus"
	otgorm "github.com/smacker/opentracing-gorm"
)

// ConfigureDatabase Database Access for environment is configured here
func ConfigureDatabase(log *logrus.Entry, replica bool) (*gorm.DB, error) {

	dbDriver := GetEnv("DATABASE_DRIVER","postgres")

	dbDataSource := GetEnv("DATABASE_URL", "")
	if replica {
		dbDataSource = GetEnv("REPLICA_DATABASE_URL", dbDataSource)
	}

	log.Debugf("Connecting using driver %v and source %v ", dbDriver, dbDataSource)

	db, err := gorm.Open(dbDriver, dbDataSource)

	if db != nil {
		otgorm.AddGormCallbacks(db)
	}
	return db, err
}
