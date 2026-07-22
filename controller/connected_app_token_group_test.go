package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

func TestConnectedAppFixedTokenGroupFromConfig(t *testing.T) {
	require.Equal(t, "", connectedAppFixedTokenGroup(nil))
	require.Equal(t, "", connectedAppFixedTokenGroup(&model.ConnectedApp{Slug: model.ConnectedAppSlugNiaoweisi}))
	require.Equal(t, "鸟维斯", connectedAppFixedTokenGroup(&model.ConnectedApp{
		Slug:              model.ConnectedAppSlugNiaoweisi,
		DefaultTokenGroup: "鸟维斯",
	}))
	require.Equal(t, "custom-group", connectedAppFixedTokenGroup(&model.ConnectedApp{
		Slug:              "other",
		DefaultTokenGroup: " custom-group ",
	}))
}
