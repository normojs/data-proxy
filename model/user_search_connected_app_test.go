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

func setupSearchUsersConnectedAppDB(t *testing.T) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:search_users_%s?mode=memory&cache=shared", t.Name())), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)

	prevDB := DB
	prevSQLite := common.UsingSQLite
	prevMySQL := common.UsingMySQL
	prevPG := common.UsingPostgreSQL
	DB = db
	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	t.Cleanup(func() {
		DB = prevDB
		common.UsingSQLite = prevSQLite
		common.UsingMySQL = prevMySQL
		common.UsingPostgreSQL = prevPG
	})

	require.NoError(t, db.AutoMigrate(&User{}, &ConnectedApp{}, &ConnectedAppGrant{}))
}

func TestSearchUsersFiltersByAuthorizedConnectedApp(t *testing.T) {
	setupSearchUsersConnectedAppDB(t)

	now := common.GetTimestamp()
	require.NoError(t, DB.Create(&ConnectedApp{
		Id:                11,
		Slug:              "filter-app-a",
		Name:              "App A",
		Status:            ConnectedAppStatusEnabled,
		AuthorizationFlow: ConnectedAppAuthorizationFlowDeviceCode,
		ClientId:          "filter-app-a",
	}).Error)
	require.NoError(t, DB.Create(&ConnectedApp{
		Id:                12,
		Slug:              "filter-app-b",
		Name:              "App B",
		Status:            ConnectedAppStatusEnabled,
		AuthorizationFlow: ConnectedAppAuthorizationFlowDeviceCode,
		ClientId:          "filter-app-b",
	}).Error)

	users := []User{
		{Id: 101, Username: "u-auth-a", DisplayName: "Auth A", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, AffCode: "aff101"},
		{Id: 102, Username: "u-auth-b", DisplayName: "Auth B", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, AffCode: "aff102"},
		{Id: 103, Username: "u-revoked-a", DisplayName: "Revoked A", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, AffCode: "aff103"},
		{Id: 104, Username: "u-none", DisplayName: "None", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, AffCode: "aff104"},
	}
	for i := range users {
		require.NoError(t, DB.Create(&users[i]).Error)
	}

	grants := []ConnectedAppGrant{
		{AppId: 11, UserId: 101, Status: ConnectedAppGrantStatusAuthorized, AuthorizedAt: now, Scopes: "openai.chat"},
		{AppId: 12, UserId: 102, Status: ConnectedAppGrantStatusAuthorized, AuthorizedAt: now, Scopes: "openai.chat"},
		{AppId: 11, UserId: 103, Status: ConnectedAppGrantStatusRevoked, AuthorizedAt: now, RevokedAt: now, Scopes: "openai.chat"},
	}
	for i := range grants {
		require.NoError(t, DB.Create(&grants[i]).Error)
	}

	found, total, err := SearchUsers("", "", nil, nil, 11, 0, 0, 50)
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Len(t, found, 1)
	require.Equal(t, 101, found[0].Id)

	foundB, totalB, err := SearchUsers("", "", nil, nil, 12, 0, 0, 50)
	require.NoError(t, err)
	require.Equal(t, int64(1), totalB)
	require.Len(t, foundB, 1)
	require.Equal(t, 102, foundB[0].Id)

	// keyword + connected app filter
	foundKW, totalKW, err := SearchUsers("Auth", "", nil, nil, 11, 0, 0, 50)
	require.NoError(t, err)
	require.Equal(t, int64(1), totalKW)
	require.Equal(t, 101, foundKW[0].Id)
}

func TestSearchUsersFiltersBySignupConnectedApp(t *testing.T) {
	setupSearchUsersConnectedAppDB(t)

	require.NoError(t, DB.Create(&User{
		Id:                   201,
		Username:             "signup-a",
		DisplayName:          "Signup A",
		Role:                 common.RoleCommonUser,
		Status:               common.UserStatusEnabled,
		AffCode:              "aff201",
		SignupConnectedAppId: 21,
	}).Error)
	require.NoError(t, DB.Create(&User{
		Id:                   202,
		Username:             "signup-b",
		DisplayName:          "Signup B",
		Role:                 common.RoleCommonUser,
		Status:               common.UserStatusEnabled,
		AffCode:              "aff202",
		SignupConnectedAppId: 22,
	}).Error)

	found, total, err := SearchUsers("", "", nil, nil, 0, 21, 0, 50)
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Len(t, found, 1)
	require.Equal(t, 201, found[0].Id)
}
