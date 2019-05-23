package utils

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/jinzhu/gorm"
)

/*
This is just documentation on creating a postgresql db and user

postgres=# create database ant_service;
postgres=# create user ant with encrypted password 'ant-secret';
postgres=# grant all privileges on database ant_service to ant;

*/

// AntMigration Our simple table holding all the migration data
type AntMigration struct {
	ID         uint   `gorm:"primary_key"`
	Name       string `gorm:"type:varchar(50);unique_index"`
	Patch      string `gorm:"type:text"`
	AppliedAt  *time.Time
	CreatedAt  time.Time
	ModifiedAt time.Time
	Version    uint32 `gorm:"DEFAULT 0"`
}

// BeforeCreate Ensures we update a migrations time stamps
func (model *AntMigration) BeforeCreate(scope *gorm.Scope) error {
	scope.SetColumn("CreatedAt", time.Now())
	return scope.SetColumn("ModifiedAt", time.Now())
}

// BeforeUpdate Updates time stamp every time we update status of a migration
func (model *AntMigration) BeforeUpdate(scope *gorm.Scope) error {
	scope.SetColumn("Version", model.Version+1)
	return scope.SetColumn("ModifiedAt", time.Now())
}

// PerformMigration finds missing migrations and records them in the database,
// We use the fragmenta_metadata table to do this
func PerformMigration(logger *logrus.Entry, db *gorm.DB) {

	migrationsDirPath := "./migrations/0001"

	// Migrate the schema
	db.AutoMigrate(&AntMigration{})

	if err := scanForNewMigrations(logger, db, migrationsDirPath); err != nil {
		logger.Warnf("Error scanning for new migrations : %v ", err)
		return
	}

	if err := applyNewMigrations(logger, db); err != nil {
		logger.Warnf("There was an error applying migrations : %v ", err)
	}
}

func scanForNewMigrations(logger *logrus.Entry, db *gorm.DB, migrationsDirPath string) error {

	// Get a list of migration files
	files, err := filepath.Glob(migrationsDirPath + "/*.sql")
	if err != nil {
		logger.Printf("Error running restore %s", err)
		return err
	}

	for _, file := range files {

		var migration AntMigration

		filename := filepath.Base(file)
		filename = strings.Replace(filename, ".sql", "", 1)

		migration.Name = filename
		migrationPatch, err := ioutil.ReadFile(file)

		if db.Where("name = ?", filename).Find(&migration).RecordNotFound() {

			if err != nil {
				logger.Warn("Problem reading migration file content : %v", err)
				continue
			}
			migration.Patch = string(migrationPatch)
			migration.Version = 0
			db.Create(&migration)
		} else {

			if migration.AppliedAt == nil {

				if migration.Patch != string(migrationPatch) {
					db.Model(&migration).Update("patch", string(migrationPatch))
				}
			}

		}
	}
	return nil
}

func applyNewMigrations(logger *logrus.Entry, db *gorm.DB) error {

	unAppliedMigrations := []AntMigration{}
	if err := db.Where("applied_at IS NULL").Find(&unAppliedMigrations).Error; err != nil {
		return err
	}

	for _, migration := range unAppliedMigrations {

		if err := db.Exec(migration.Patch).Error; err != nil {
			return err
		}

		db.Model(&migration).UpdateColumn("applied_at", time.Now())
		logger.Infof("Successfully applied the file : %v", fmt.Sprintf("%s.sql", migration.Name))
	}

	return nil
}
