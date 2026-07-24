package main

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/coalaura/schgo"
	_ "github.com/mattn/go-sqlite3"
)

const DatabasePath = "whiskr.db"

type Database struct {
	*sql.DB
}

type StatisticRecord struct {
	Model           string
	Provider        string
	InputTokens     int
	OutputTokens    int
	ReasoningTokens int
	Cost            float64
	TTFTMs          int64
	DurationMs      int64
	Success         bool
}

func ConnectToDatabase() (*Database, error) {
	dsn := fmt.Sprintf("%s?_journal_mode=WAL&_busy_timeout=5000&_sync=NORMAL", DatabasePath)

	conn, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, err
	}

	conn.SetMaxOpenConns(16)
	conn.SetMaxIdleConns(16)
	conn.SetConnMaxLifetime(time.Hour)

	schema, err := schgo.NewSchema(conn)
	if err != nil {
		conn.Close()

		return nil, err
	}

	table := schema.Table("statistics")

	table.Primary("id", "INTEGER").AutoIncrement()
	table.Column("created_at", "INTEGER").NotNull()
	table.Column("model", "TEXT").NotNull()
	table.Column("provider", "TEXT").NotNull().Default("")
	table.Column("input_tokens", "INTEGER").NotNull().Default("0")
	table.Column("output_tokens", "INTEGER").NotNull().Default("0")
	table.Column("reasoning_tokens", "INTEGER").NotNull().Default("0")
	table.Column("cost", "REAL").NotNull().Default("0")
	table.Column("ttft_ms", "INTEGER")
	table.Column("duration_ms", "INTEGER").NotNull().Default("0")
	table.Column("success", "INTEGER").NotNull().Default("0")

	table.Index("idx_statistics_model", "model")
	table.Index("idx_statistics_created_at", "created_at")

	err = schema.Apply()
	if err != nil {
		conn.Close()

		return nil, err
	}

	return &Database{conn}, nil
}

func (d *Database) AddStatistics(rec StatisticRecord) error {
	var ttft sql.NullInt64

	if rec.TTFTMs > 0 {
		ttft.Valid = true
		ttft.Int64 = rec.TTFTMs
	}

	var success int

	if rec.Success {
		success = 1
	}

	_, err := d.Exec(
		"INSERT INTO statistics (created_at, model, provider, input_tokens, output_tokens, reasoning_tokens, cost, ttft_ms, duration_ms, success) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		time.Now().Unix(),
		rec.Model,
		rec.Provider,
		rec.InputTokens,
		rec.OutputTokens,
		rec.ReasoningTokens,
		rec.Cost,
		ttft,
		rec.DurationMs,
		success,
	)

	return err
}
