package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

func TestTokenGroupUnavailableReasonHonorsUserBindings(t *testing.T) {
	user := &model.UserBase{
		Group:       "vip",
		TokenGroups: `["default"]`,
	}

	require.Empty(t, tokenGroupUnavailableReason("default", user))
	require.Contains(t, tokenGroupUnavailableReason("", user), "vip")
	require.Contains(t, tokenGroupUnavailableReason("vip", user), "vip")
}
