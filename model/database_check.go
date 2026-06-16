package model

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func DetectDatabaseType(dsn string) string {
	if dsn == "" || strings.HasPrefix(dsn, "local") {
		return "sqlite"
	}
	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		return "postgres"
	}
	return "mysql"
}

func TestDatabaseConnection(dsn string, sqlitePath string) (string, error) {
	dbType := DetectDatabaseType(dsn)
	db, err := openDatabaseForConnectionTest(dbType, dsn, sqlitePath)
	if err != nil {
		return dbType, err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return dbType, err
	}
	defer sqlDB.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := sqlDB.PingContext(ctx); err != nil {
		return dbType, err
	}
	return dbType, nil
}

func openDatabaseForConnectionTest(dbType string, dsn string, sqlitePath string) (*gorm.DB, error) {
	switch dbType {
	case "sqlite":
		if sqlitePath == "" {
			sqlitePath = common.SQLitePath
		}
		return gorm.Open(sqlite.Open(sqlitePath), &gorm.Config{})
	case "postgres":
		return gorm.Open(postgres.New(postgres.Config{
			DSN:                  dsn,
			PreferSimpleProtocol: true,
		}), &gorm.Config{})
	case "mysql":
		if !strings.Contains(dsn, "parseTime") {
			if strings.Contains(dsn, "?") {
				dsn += "&parseTime=true"
			} else {
				dsn += "?parseTime=true"
			}
		}
		return gorm.Open(mysql.Open(dsn), &gorm.Config{})
	default:
		return nil, fmt.Errorf("unsupported database type: %s", dbType)
	}
}
