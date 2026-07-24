package main

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/coalaura/schgo"
	_ "github.com/mattn/go-sqlite3"
	"github.com/revrost/go-openrouter"
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
	CachedTokens    int
	Cost            float64
	TTFTMs          int64
	TTFOMs          int64
	ReasoningMs     int64
	DurationMs      int64
	InputImages     int
	OutputImages    int
	InputFiles      int
	ToolCall        bool
	ToolName        string
	HasReasoning    bool
	FinishReason    string
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
	table.Column("cached_tokens", "INTEGER").NotNull().Default("0")
	table.Column("cost", "REAL").NotNull().Default("0")
	table.Column("ttft_ms", "INTEGER")
	table.Column("ttfo_ms", "INTEGER")
	table.Column("reasoning_ms", "INTEGER")
	table.Column("duration_ms", "INTEGER").NotNull().Default("0")
	table.Column("input_images", "INTEGER").NotNull().Default("0")
	table.Column("output_images", "INTEGER").NotNull().Default("0")
	table.Column("input_files", "INTEGER").NotNull().Default("0")
	table.Column("tool_call", "INTEGER").NotNull().Default("0")
	table.Column("tool_name", "TEXT").NotNull().Default("")
	table.Column("has_reasoning", "INTEGER").NotNull().Default("0")
	table.Column("finish_reason", "TEXT").NotNull().Default("")
	table.Column("success", "INTEGER").NotNull().Default("0")

	table.Index("idx_statistics_model", "model")
	table.Index("idx_statistics_provider", "provider")
	table.Index("idx_statistics_created_at", "created_at")

	err = schema.Apply()
	if err != nil {
		conn.Close()

		return nil, err
	}

	return &Database{conn}, nil
}

func (d *Database) AddStatistics(rec StatisticRecord) error {
	_, err := d.Exec(
		`INSERT INTO statistics (
			created_at, model, provider,
			input_tokens, output_tokens, reasoning_tokens, cached_tokens, cost,
			ttft_ms, ttfo_ms, reasoning_ms, duration_ms,
			input_images, output_images, input_files,
			tool_call, tool_name, has_reasoning, finish_reason, success
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		time.Now().Unix(),
		rec.Model,
		rec.Provider,
		rec.InputTokens,
		rec.OutputTokens,
		rec.ReasoningTokens,
		rec.CachedTokens,
		rec.Cost,
		nullableInt64(rec.TTFTMs),
		nullableInt64(rec.TTFOMs),
		nullableInt64(rec.ReasoningMs),
		rec.DurationMs,
		rec.InputImages,
		rec.OutputImages,
		rec.InputFiles,
		bool2int(rec.ToolCall),
		rec.ToolName,
		bool2int(rec.HasReasoning),
		rec.FinishReason,
		bool2int(rec.Success),
	)

	return err
}

func bool2int(val bool) int {
	if val {
		return 1
	}

	return 0
}

func nullableInt64(val int64) sql.NullInt64 {
	if val <= 0 {
		return sql.NullInt64{}
	}

	return sql.NullInt64{Int64: val, Valid: true}
}

func countMediaInRequest(request *openrouter.ChatCompletionRequest) (int, int) {
	var (
		images int
		files  int
	)

	for _, message := range request.Messages {
		images += len(message.Images)

		for _, part := range message.Content.Multi {
			switch part.Type {
			case openrouter.ChatMessagePartTypeImageURL:
				images++
			case openrouter.ChatMessagePartTypeFile:
				files++
			case openrouter.ChatMessagePartTypeText:
				files += strings.Count(part.Text, "<file name=")
			}
		}

		if message.Content.Text != "" {
			files += strings.Count(message.Content.Text, "<file name=")
		}
	}

	return images, files
}
