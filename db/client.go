// db/client.go
package db

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "github.com/marcboeker/go-duckdb" // duckdb driver registration
)

// Client represents a DuckDB client that stores its database and parquet files in a given directory.
type Client struct {
	DB  *sql.DB
	dir string
}

// NewClient creates a new DuckDB client using the specified directory.
// It uses the directory to store a persistent DuckDB database file ("duckdb.db")
// as well as to write/read Parquet files.
func NewClient(dir string) (*Client, error) {
	// Check if the directory exists; if not, create it.
	info, err := os.Stat(dir)
	if os.IsNotExist(err) {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create directory: %w", err)
		}
		// Re-stat the directory after creation.
		info, err = os.Stat(dir)
		if err != nil {
			return nil, fmt.Errorf("failed to stat directory after creation: %w", err)
		}
	} else if err != nil {
		return nil, fmt.Errorf("failed to stat directory: %w", err)
	}

	if !info.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", dir)
	}

	// Create a path for the DuckDB database file inside the given directory.
	dbPath := filepath.Join(dir, "duck.db?threads=4")
	fmt.Println(dbPath)
	// connStr := fmt.Sprintf("file:%s", dbPath)
	db, err := sql.Open("duckdb", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open duckdb: %w", err)
	}

	return &Client{
		DB:  db,
		dir: dir,
	}, nil
}

// Start ensures that the database connection is available by pinging it.
func (c *Client) Start(ctx context.Context) error {
	if err := c.DB.PingContext(ctx); err != nil {
		return fmt.Errorf("failed to ping duckdb: %w", err)
	}
	return nil
}

// Stop closes the DuckDB connection.
func (c *Client) Stop() error {
	return c.DB.Close()
}

// Conn returns the underlying database connection for running queries directly.
func (c *Client) Conn() *sql.DB {
	return c.DB
}

// WriteParquet executes the provided query and writes the results to a Parquet file.
// The file is saved in the client's directory with the given filename.
func (c *Client) WriteParquet(ctx context.Context, query, filename string) error {
	outPath := filepath.Join(c.dir, filename)
	sqlQuery := fmt.Sprintf("COPY (%s) TO '%s' (FORMAT 'parquet')", query, outPath)
	_, err := c.DB.ExecContext(ctx, sqlQuery)
	if err != nil {
		return fmt.Errorf("failed to write parquet file: %w", err)
	}
	return nil
}

// ReadParquet reads the Parquet file with the given filename from the client's directory.
// It returns the resulting rows from the query that reads the file.
func (c *Client) ReadParquet(ctx context.Context, filename string) (*sql.Rows, error) {
	filePath := filepath.Join(c.dir, filename)
	query := fmt.Sprintf("SELECT * FROM read_parquet('%s')", filePath)
	return c.DB.QueryContext(ctx, query)
}
