package model

import (
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestChannelKeyEncryptedAtRest(t *testing.T) {
	originalDB := DB
	originalCryptoSecret := common.CryptoSecret
	common.CryptoSecret = "channel-key-encryption-test-secret"
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	DB = db
	require.NoError(t, DB.AutoMigrate(&Channel{}))
	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
		DB = originalDB
		common.CryptoSecret = originalCryptoSecret
	})

	channel := Channel{
		Name:   "encrypted channel",
		Type:   1,
		Key:    "sk-test-secret",
		Models: "gpt-4o",
		Group:  "default",
	}
	require.NoError(t, DB.Create(&channel).Error)

	var storedKey string
	require.NoError(t, DB.Table("channels").
		Select("key").
		Where("id = ?", channel.Id).
		Scan(&storedKey).Error)
	require.NotEqual(t, "sk-test-secret", storedKey)
	require.True(t, strings.HasPrefix(storedKey, "enc:v1:"))

	var fetched Channel
	require.NoError(t, DB.First(&fetched, "id = ?", channel.Id).Error)
	require.Equal(t, "sk-test-secret", fetched.Key)

	fetched.Name = "encrypted channel updated"
	require.NoError(t, DB.Save(&fetched).Error)
	var updatedStoredKey string
	require.NoError(t, DB.Table("channels").
		Select("key").
		Where("id = ?", channel.Id).
		Scan(&updatedStoredKey).Error)
	require.NotEqual(t, "sk-test-secret", updatedStoredKey)
	require.True(t, strings.HasPrefix(updatedStoredKey, "enc:v1:"))

	require.NoError(t, DB.Exec("UPDATE channels SET key = ? WHERE id = ?", "legacy-plain-key", channel.Id).Error)
	var legacyFetched Channel
	require.NoError(t, DB.First(&legacyFetched, "id = ?", channel.Id).Error)
	require.Equal(t, "legacy-plain-key", legacyFetched.Key)
	require.NoError(t, DB.Save(&legacyFetched).Error)

	var migratedKey string
	require.NoError(t, DB.Table("channels").
		Select("key").
		Where("id = ?", channel.Id).
		Scan(&migratedKey).Error)
	require.NotEqual(t, "legacy-plain-key", migratedKey)
	require.True(t, strings.HasPrefix(migratedKey, "enc:v1:"))
}
