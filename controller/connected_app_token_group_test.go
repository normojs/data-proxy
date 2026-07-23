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

func TestBuildSnaplessAppResponseIncludesDefaultTokenGroup(t *testing.T) {
	resp := buildSnaplessAppResponse(&model.ConnectedApp{
		Id:                7,
		Slug:              model.ConnectedAppSlugNiaoweisi,
		Name:              "鸟维斯",
		DefaultTokenGroup: " 鸟维斯 ",
	})
	require.Equal(t, "鸟维斯", resp.DefaultTokenGroup)
	require.Equal(t, model.ConnectedAppSlugNiaoweisi, resp.Slug)

	empty := buildSnaplessAppResponse(&model.ConnectedApp{Slug: "other"})
	require.Equal(t, "", empty.DefaultTokenGroup)
}

func TestBuildSnaplessTokenSummaryIncludesGroup(t *testing.T) {
	summary := buildSnaplessTokenSummary(&model.Token{
		Id:    9,
		Name:  "niaoweisi-device",
		Group: " 鸟维斯 ",
	}, nil)
	require.Equal(t, "鸟维斯", summary.Group)
	require.Equal(t, 9, summary.ID)
}
