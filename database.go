package main

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/coalaura/schgo"
	_ "modernc.org/sqlite"
)

const DatabasePath = "chat.db"

type Database struct {
	*sql.DB
}

func ConnectToDatabase() (*Database, error) {
	dsn := fmt.Sprintf("%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)", DatabasePath)

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(16)
	db.SetMaxIdleConns(16)
	db.SetConnMaxLifetime(time.Hour)

	schema, err := schgo.NewSchema(db)
	if err != nil {
		db.Close()

		return nil, err
	}

	// model_statistics

	table := schema.Table("model_statistics")

	table.Primary("slug", "TEXT").NotNull()

	table.Column("requests", "INTEGER").Default("0")

	err = schema.Apply()
	if err != nil {
		db.Close()

		return nil, err
	}

	return &Database{db}, nil
}

func (d *Database) IncrementModelStatistics(slug string) error {
	_, err := d.Exec("INSERT INTO model_statistics (slug, requests) VALUES (?, 1) ON CONFLICT(slug) DO UPDATE SET requests = requests + 1", slug)
	return err
}
