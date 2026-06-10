package model

import (
	"fmt"

	"github.com/QuantumNous/new-api/common"
)

func migrateMCPToolOpenAPIUrlColumn() error {
	return migrateOpenAPIUrlColumn("mcp_tools")
}

func migrateMCPOpenAPIToolOpenAPIUrlColumn() error {
	return migrateOpenAPIUrlColumn("mcp_openapi_tools")
}

func migrateOpenAPIUrlColumn(tableName string) error {
	oldColumnName := "open_api_url"
	newColumnName := "openapi_url"
	if !DB.Migrator().HasTable(tableName) {
		return nil
	}

	if common.UsingSQLite {
		return migrateMCPToolOpenAPIUrlColumnSQLite(tableName, oldColumnName, newColumnName)
	}

	if common.UsingPostgreSQL {
		return migrateMCPToolOpenAPIUrlColumnPostgreSQL(tableName, oldColumnName, newColumnName)
	}

	if common.UsingMySQL {
		return migrateMCPToolOpenAPIUrlColumnMySQL(tableName, oldColumnName, newColumnName)
	}

	return nil
}

func migrateMCPToolOpenAPIUrlColumnSQLite(tableName string, oldColumnName string, newColumnName string) error {
	hasOld, err := sqliteColumnExists(tableName, oldColumnName)
	if err != nil || !hasOld {
		return err
	}
	hasNew, err := sqliteColumnExists(tableName, newColumnName)
	if err != nil {
		return err
	}
	if hasNew {
		return DB.Exec(fmt.Sprintf("UPDATE `%s` SET `%s` = COALESCE(NULLIF(`%s`, ''), `%s`)", tableName, newColumnName, newColumnName, oldColumnName)).Error
	}
	return DB.Exec(fmt.Sprintf("ALTER TABLE `%s` RENAME COLUMN `%s` TO `%s`", tableName, oldColumnName, newColumnName)).Error
}

func migrateMCPToolOpenAPIUrlColumnPostgreSQL(tableName string, oldColumnName string, newColumnName string) error {
	hasOld, err := postgreSQLColumnExists(tableName, oldColumnName)
	if err != nil || !hasOld {
		return err
	}
	hasNew, err := postgreSQLColumnExists(tableName, newColumnName)
	if err != nil {
		return err
	}
	if hasNew {
		if err := DB.Exec(fmt.Sprintf(`UPDATE "%s" SET "%s" = COALESCE(NULLIF("%s", ''), "%s")`, tableName, newColumnName, newColumnName, oldColumnName)).Error; err != nil {
			return err
		}
		return DB.Exec(fmt.Sprintf(`ALTER TABLE "%s" DROP COLUMN "%s"`, tableName, oldColumnName)).Error
	}
	return DB.Exec(fmt.Sprintf(`ALTER TABLE "%s" RENAME COLUMN "%s" TO "%s"`, tableName, oldColumnName, newColumnName)).Error
}

func migrateMCPToolOpenAPIUrlColumnMySQL(tableName string, oldColumnName string, newColumnName string) error {
	hasOld, err := mySQLColumnExists(tableName, oldColumnName)
	if err != nil || !hasOld {
		return err
	}
	hasNew, err := mySQLColumnExists(tableName, newColumnName)
	if err != nil {
		return err
	}
	if hasNew {
		if err := DB.Exec(fmt.Sprintf("UPDATE `%s` SET `%s` = COALESCE(NULLIF(`%s`, ''), `%s`)", tableName, newColumnName, newColumnName, oldColumnName)).Error; err != nil {
			return err
		}
		return DB.Exec(fmt.Sprintf("ALTER TABLE `%s` DROP COLUMN `%s`", tableName, oldColumnName)).Error
	}
	return DB.Exec(fmt.Sprintf("ALTER TABLE `%s` RENAME COLUMN `%s` TO `%s`", tableName, oldColumnName, newColumnName)).Error
}

func sqliteColumnExists(tableName string, columnName string) (bool, error) {
	var count int64
	err := DB.Raw("SELECT COUNT(*) FROM pragma_table_info(?) WHERE name = ?", tableName, columnName).Scan(&count).Error
	return count > 0, err
}

func postgreSQLColumnExists(tableName string, columnName string) (bool, error) {
	var count int64
	err := DB.Raw(`SELECT COUNT(*) FROM information_schema.columns
		WHERE table_schema = current_schema() AND table_name = ? AND column_name = ?`,
		tableName, columnName).Scan(&count).Error
	return count > 0, err
}

func mySQLColumnExists(tableName string, columnName string) (bool, error) {
	var count int64
	err := DB.Raw(`SELECT COUNT(*) FROM information_schema.columns
		WHERE table_schema = DATABASE() AND table_name = ? AND column_name = ?`,
		tableName, columnName).Scan(&count).Error
	return count > 0, err
}
