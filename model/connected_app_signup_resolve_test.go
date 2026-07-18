package model

import (
	"fmt"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestResolveConnectedAppIDForSignup(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:signup_resolve_%s?mode=memory&cache=shared", t.Name())), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)

	prev := DB
	prevSQLite, prevMySQL, prevPG := common.UsingSQLite, common.UsingMySQL, common.UsingPostgreSQL
	DB = db
	common.UsingSQLite, common.UsingMySQL, common.UsingPostgreSQL = true, false, false
	t.Cleanup(func() {
		DB = prev
		common.UsingSQLite, common.UsingMySQL, common.UsingPostgreSQL = prevSQLite, prevMySQL, prevPG
	})

	require.NoError(t, db.AutoMigrate(&ConnectedApp{}))
	require.NoError(t, DB.Create(&ConnectedApp{
		Id:                55,
		Slug:              "niaoweisi",
		Name:              "鸟维斯",
		Status:            ConnectedAppStatusEnabled,
		AuthorizationFlow: ConnectedAppAuthorizationFlowDeviceCode,
		ClientId:          "niaoweisi",
	}).Error)

	require.Equal(t, 0, ResolveConnectedAppIDForSignup(""))
	require.Equal(t, 0, ResolveConnectedAppIDForSignup("missing-app"))
	require.Equal(t, 55, ResolveConnectedAppIDForSignup("niaoweisi"))
	require.Equal(t, 55, ResolveConnectedAppIDForSignup("55"))
	require.Equal(t, 0, ResolveConnectedAppIDForSignup("999"))
}
