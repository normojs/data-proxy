package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFilterUserUsableGroupsByBindings(t *testing.T) {
	groups := map[string]string{
		"default": "Default",
		"vip":     "VIP",
	}

	require.Equal(t, groups, FilterUserUsableGroupsByBindings(groups, nil))
	require.Equal(t, map[string]string{"vip": "VIP"}, FilterUserUsableGroupsByBindings(groups, []string{"vip", "missing"}))
	require.Empty(t, FilterUserUsableGroupsByBindings(groups, []string{"missing"}))
}

func TestGetUserAutoGroupWithBindings(t *testing.T) {
	require.Equal(t, []string{"default"}, GetUserAutoGroupWithBindings("vip", nil))
	require.Equal(t, []string{"default"}, GetUserAutoGroupWithBindings("vip", []string{"default", "vip"}))
	require.Empty(t, GetUserAutoGroupWithBindings("vip", []string{"vip"}))
}
