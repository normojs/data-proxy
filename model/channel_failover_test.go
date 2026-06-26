package model

import (
	"fmt"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func seedFailoverChannel(t *testing.T, channelID int, modelName string, priority int64) {
	t.Helper()
	require.NoError(t, DB.Create(&Channel{
		Id:       channelID,
		Key:      fmt.Sprintf("sk-failover-%d", channelID),
		Status:   common.ChannelStatusEnabled,
		Name:     fmt.Sprintf("failover-channel-%d", channelID),
		Models:   modelName,
		Group:    "default",
		Priority: &priority,
	}).Error)
	require.NoError(t, DB.Create(&Ability{
		Group:     "default",
		Model:     modelName,
		ChannelId: channelID,
		Enabled:   true,
		Priority:  &priority,
		Weight:    100,
	}).Error)
}

func TestGetRandomSatisfiedChannelExcludingKeepsSamePriorityMemoryCache(t *testing.T) {
	truncateTables(t)
	previousMemoryCacheEnabled := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = true
	t.Cleanup(func() {
		common.MemoryCacheEnabled = previousMemoryCacheEnabled
		InitChannelCache()
	})

	const modelName = "gpt-failover-memory"
	seedFailoverChannel(t, 1, modelName, 10)
	seedFailoverChannel(t, 2, modelName, 10)
	seedFailoverChannel(t, 3, modelName, 5)
	InitChannelCache()

	channel, err := GetRandomSatisfiedChannelExcluding("default", modelName, 0, map[int]bool{1: true})
	require.NoError(t, err)
	require.NotNil(t, channel)
	require.Equal(t, 2, channel.Id)
}

func TestGetRandomSatisfiedChannelExcludingKeepsSamePriorityDatabase(t *testing.T) {
	truncateTables(t)
	previousMemoryCacheEnabled := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = false
	t.Cleanup(func() {
		common.MemoryCacheEnabled = previousMemoryCacheEnabled
	})

	const modelName = "gpt-failover-database"
	seedFailoverChannel(t, 1, modelName, 10)
	seedFailoverChannel(t, 2, modelName, 10)
	seedFailoverChannel(t, 3, modelName, 5)

	channel, err := GetRandomSatisfiedChannelExcluding("default", modelName, 0, map[int]bool{1: true})
	require.NoError(t, err)
	require.NotNil(t, channel)
	require.Equal(t, 2, channel.Id)
}
