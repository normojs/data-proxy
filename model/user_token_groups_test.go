package model

import (
	"fmt"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestTokenGroupsNormalizeParseAndMarshal(t *testing.T) {
	require.Equal(t, []string{"default", "vip"}, NormalizeTokenGroups([]string{" default ", "", "vip", "default"}))
	require.Equal(t, []string{"default", "vip"}, ParseTokenGroups(`["default"," vip ","default"]`))
	require.Equal(t, []string{"default", "vip", "archive"}, ParseTokenGroups("default, vip\narchive，default"))
	require.Equal(t, `["default","vip"]`, MarshalTokenGroups([]string{"default", "vip", "default"}))
	require.Empty(t, MarshalTokenGroups([]string{"", "  "}))
}

func TestTokenGroupAllowedByBindings(t *testing.T) {
	require.True(t, TokenGroupAllowedByBindings(nil, "default"))
	require.True(t, TokenGroupAllowedByBindings([]string{" default ", "vip"}, "default"))
	require.False(t, TokenGroupAllowedByBindings([]string{"vip"}, "default"))
	require.False(t, TokenGroupAllowedByBindings([]string{"vip"}, ""))
}

func TestUserEditPreservesAndClearsTokenGroups(t *testing.T) {
	truncateTables(t)
	user := User{
		Id:          9001,
		Username:    "token-groups-user",
		Password:    "password",
		DisplayName: "Token Groups User",
		Group:       "default",
		Status:      common.UserStatusEnabled,
		AffCode:     "token-groups-user-aff",
	}
	user.SetTokenGroups([]string{"default", "vip"})
	require.NoError(t, DB.Create(&user).Error)

	update := User{
		Id:          user.Id,
		Username:    user.Username,
		DisplayName: "Renamed",
		Group:       user.Group,
		Remark:      "kept",
		TokenGroups: user.TokenGroups,
	}
	require.NoError(t, update.Edit(false))

	var afterKeep User
	require.NoError(t, DB.First(&afterKeep, user.Id).Error)
	require.Equal(t, []string{"default", "vip"}, afterKeep.GetTokenGroups())

	clear := User{
		Id:          user.Id,
		Username:    user.Username,
		DisplayName: "Renamed",
		Group:       user.Group,
		Remark:      fmt.Sprintf("%s-clear", afterKeep.Remark),
		TokenGroups: "",
	}
	require.NoError(t, clear.Edit(false))

	var afterClear User
	require.NoError(t, DB.First(&afterClear, user.Id).Error)
	require.Empty(t, afterClear.GetTokenGroups())
}
