package service

import (
	"errors"
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/require"
)

func TestRetryParamSelectionRetryResetsWhenChannelExcluded(t *testing.T) {
	retry := 2
	param := &RetryParam{Retry: &retry}

	require.Equal(t, 2, param.SelectionRetry())

	param.AddExcludedChannel(10)
	require.Equal(t, 0, param.SelectionRetry())
	require.Equal(t, map[int]bool{10: true}, param.ExcludeChannelIds)
}

func TestCacheGetRandomSatisfiedChannelExcludesTemporarilyUnavailableChannels(t *testing.T) {
	require.NoError(t, model.DB.AutoMigrate(&model.Channel{}, &model.Ability{}))
	truncateChannelSelectTables(t)
	withAutomaticDisableForTest(t)
	resetChannelHealthForTest()
	t.Cleanup(resetChannelHealthForTest)

	previousMemoryCacheEnabled := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = true
	t.Cleanup(func() {
		common.MemoryCacheEnabled = previousMemoryCacheEnabled
		model.InitChannelCache()
	})

	const modelName = "gpt-channel-health-failover"
	seedChannelSelectChannel(t, 1101, modelName, 10)
	seedChannelSelectChannel(t, 1102, modelName, 10)
	model.InitChannelCache()

	channelError := *types.NewChannelError(1101, 0, "cooling-channel", false, "", true)
	apiErr := types.NewOpenAIError(errors.New("rate limited"), types.ErrorCodeBadResponseStatusCode, http.StatusTooManyRequests)
	HandleChannelFailure(channelError, apiErr)
	HandleChannelFailure(channelError, apiErr)
	HandleChannelFailure(channelError, apiErr)

	retry := 0
	channel, selectGroup, selectErr := CacheGetRandomSatisfiedChannel(&RetryParam{
		TokenGroup: "default",
		ModelName:  modelName,
		Retry:      &retry,
	})

	require.NoError(t, selectErr)
	require.Equal(t, "default", selectGroup)
	require.NotNil(t, channel)
	require.Equal(t, 1102, channel.Id)
}

func TestCacheGetRandomSatisfiedChannelScopesBySubsite(t *testing.T) {
	require.NoError(t, model.DB.AutoMigrate(&model.Channel{}, &model.Ability{}))

	for _, memoryCacheEnabled := range []bool{false, true} {
		t.Run(map[bool]string{false: "database", true: "memory_cache"}[memoryCacheEnabled], func(t *testing.T) {
			truncateChannelSelectTables(t)
			previousMemoryCacheEnabled := common.MemoryCacheEnabled
			common.MemoryCacheEnabled = memoryCacheEnabled
			t.Cleanup(func() {
				common.MemoryCacheEnabled = previousMemoryCacheEnabled
				model.InitChannelCache()
			})

			const modelName = "gpt-subsite-channel-scope"
			seedChannelSelectChannelForSubsite(t, 1201, modelName, 100, 0)
			seedChannelSelectChannelForSubsite(t, 1202, modelName, 10, 42)
			model.InitChannelCache()

			retry := 0
			mainChannel, selectGroup, selectErr := CacheGetRandomSatisfiedChannel(&RetryParam{
				TokenGroup: "default",
				ModelName:  modelName,
				Retry:      &retry,
			})
			require.NoError(t, selectErr)
			require.Equal(t, "default", selectGroup)
			require.NotNil(t, mainChannel)
			require.Equal(t, 1201, mainChannel.Id)

			retry = 0
			subsiteChannel, selectGroup, selectErr := CacheGetRandomSatisfiedChannel(&RetryParam{
				TokenGroup: "default",
				ModelName:  modelName,
				Retry:      &retry,
				SubsiteId:  42,
			})
			require.NoError(t, selectErr)
			require.Equal(t, "default", selectGroup)
			require.NotNil(t, subsiteChannel)
			require.Equal(t, 1202, subsiteChannel.Id)
		})
	}
}

func truncateChannelSelectTables(t *testing.T) {
	t.Helper()
	cleanup := func() {
		model.DB.Exec("DELETE FROM abilities")
		model.DB.Exec("DELETE FROM channels")
	}
	cleanup()
	t.Cleanup(cleanup)
}

func seedChannelSelectChannel(t *testing.T, channelID int, modelName string, priority int64) {
	seedChannelSelectChannelForSubsite(t, channelID, modelName, priority, 0)
}

func seedChannelSelectChannelForSubsite(t *testing.T, channelID int, modelName string, priority int64, subsiteId int64) {
	t.Helper()
	require.NoError(t, model.DB.Create(&model.Channel{
		Id:        channelID,
		SubsiteId: subsiteId,
		Key:       "sk-channel-health",
		Status:    common.ChannelStatusEnabled,
		Name:      "channel-health",
		Models:    modelName,
		Group:     "default",
		Priority:  &priority,
	}).Error)
	require.NoError(t, model.DB.Create(&model.Ability{
		Group:     "default",
		Model:     modelName,
		ChannelId: channelID,
		Enabled:   true,
		Priority:  &priority,
		Weight:    100,
	}).Error)
}
