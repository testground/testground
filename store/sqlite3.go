package store

import (
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/mattn/go-sqlite3"

	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/sqlite"

	"github.com/pkg/errors"
)

type SqliteStore struct {
	db *gorm.DB
}

func NewSqliteStore() (*SqliteStore, error) {
	db, err := gorm.Open("sqlite3", "file:test-pipeline.db")
	if err != nil {
		return nil, errors.Wrap(err, "failed while opening sqlite3 database via gorm")
	}

	db.LogMode(true)

	driver, err := sqlite3.WithInstance(db.DB(), &sqlite3.Config{})
	if err != nil {
		return nil, errors.Wrap(err, "failed while instantiating sqlite3 migration driver")
	}

	mig, err := migrate.NewWithDatabaseInstance("file://../sql", "main", driver)
	if err != nil {
		return nil, errors.Wrap(err, "failed while instantiating sqlite3 migrator")
	}

	err = mig.Up()
	if err != nil && err != migrate.ErrNoChange {
		return nil, errors.Wrap(err, "failed while migrating database all the way up")
	}

	entities := []interface{}{
		&TestPlan{},
		&TestCase{},
		&TestRun{},
		&TestIteration{},
		&MetricDefinition{},
		&Result{},
		&Commit{},
	}

	if err := db.AutoMigrate(entities...).Error; err != nil {
		return nil, errors.Wrap(err, "failed while automigrating sqlite3 db")
	}

	return &SqliteStore{db}, nil
}
