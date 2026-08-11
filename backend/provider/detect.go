package provider

import (
	"os"
	"path/filepath"
	"strings"
)

type DBType string

const (
	DBTypeMySQL    DBType = "mysql"
	DBTypeMongo    DBType = "mongodb"
	DBTypePostgres DBType = "postgres"
	DBTypeNone     DBType = "none"
)

func DetectDatabaseRequirements(repoPath string) (DBType, error) {
	// Node.js
	packageJson := filepath.Join(repoPath, "package.json")
	if content, err := os.ReadFile(packageJson); err == nil {
		contentStr := string(content)
		if strings.Contains(contentStr, "\"mysql\"") || strings.Contains(contentStr, "\"mysql2\"") {
			return DBTypeMySQL, nil
		}
		if strings.Contains(contentStr, "\"mongodb\"") || strings.Contains(contentStr, "\"mongoose\"") {
			return DBTypeMongo, nil
		}
		if strings.Contains(contentStr, "\"pg\"") || strings.Contains(contentStr, "\"pg-promise\"") {
			return DBTypePostgres, nil
		}
	}

	// Python
	requirementsTxt := filepath.Join(repoPath, "requirements.txt")
	if content, err := os.ReadFile(requirementsTxt); err == nil {
		contentStr := string(content)
		if strings.Contains(contentStr, "mysql-connector") || strings.Contains(contentStr, "PyMySQL") {
			return DBTypeMySQL, nil
		}
		if strings.Contains(contentStr, "pymongo") || strings.Contains(contentStr, "motor") {
			return DBTypeMongo, nil
		}
		if strings.Contains(contentStr, "psycopg2") || strings.Contains(contentStr, "asyncpg") {
			return DBTypePostgres, nil
		}
	}

	// Go
	gomod := filepath.Join(repoPath, "go.mod")
	if content, err := os.ReadFile(gomod); err == nil {
		contentStr := string(content)
		if strings.Contains(contentStr, "github.com/go-sql-driver/mysql") {
			return DBTypeMySQL, nil
		}
		if strings.Contains(contentStr, "github.com/lib/pq") || strings.Contains(contentStr, "github.com/jackc/pgx") {
			return DBTypePostgres, nil
		}
		if strings.Contains(contentStr, "go.mongodb.org/mongo-driver") {
			return DBTypeMongo, nil
		}
	}

	return DBTypeNone, nil
}
