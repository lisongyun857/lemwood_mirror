package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

var DB *sql.DB

func InitDB(storagePath string) error {
	dbPath := filepath.Join(storagePath, "stats.db")

	// 确保目录存在
	if err := os.MkdirAll(storagePath, 0755); err != nil {
		return fmt.Errorf("创建数据库目录失败: %w", err)
	}

	var err error
	DB, err = sql.Open("sqlite", dbPath)
	if err != nil {
		return fmt.Errorf("打开数据库失败: %w", err)
	}

	if err := DB.Ping(); err != nil {
		return fmt.Errorf("连接数据库失败: %w", err)
	}

	return createTables()
}

func createTables() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS visits (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            ip TEXT,
            path TEXT,
            user_agent TEXT,
            referer TEXT,
            country TEXT,
            region TEXT,
            city TEXT,
            created_at DATETIME DEFAULT CURRENT_TIMESTAMP
        )`,
		`CREATE TABLE IF NOT EXISTS downloads (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            file_name TEXT,
            launcher TEXT,
            version TEXT,
            ip TEXT,
            country TEXT,
            created_at DATETIME DEFAULT CURRENT_TIMESTAMP
        )`,
		`CREATE INDEX IF NOT EXISTS idx_visits_created_at ON visits(created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_downloads_created_at ON downloads(created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_downloads_file_name ON downloads(file_name)`,
	}

	for _, query := range queries {
		if _, err := DB.Exec(query); err != nil {
			return fmt.Errorf("创建表失败: %w, query: %s", err, query)
		}
	}
	return nil
}
