package servicestatus

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupServiceStatusTestDB(t *testing.T) {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Channel{}, &model.PerfChannelMetric{}))

	originalDB := model.DB
	model.DB = db
	t.Cleanup(func() {
		model.DB = originalDB
	})
}

func TestQueryAggregatesChannelStatusAndAdminAlerts(t *testing.T) {
	setupServiceStatusTestDB(t)

	lowPriority := int64(1)
	highPriority := int64(10)
	channels := []*model.Channel{
		{
			Id:       1,
			Type:     constant.ChannelTypeOpenAI,
			Key:      "secret-normal",
			Name:     "OpenAI primary",
			Status:   common.ChannelStatusEnabled,
			Models:   "gpt-4o,gpt-4o-mini",
			Group:    "default",
			Priority: &highPriority,
		},
		{
			Id:       2,
			Type:     constant.ChannelTypeAnthropic,
			Key:      "secret-disabled",
			Name:     "Disabled outage",
			Status:   common.ChannelStatusManuallyDisabled,
			Models:   "claude-3-5-sonnet",
			Group:    "vip",
			Priority: &lowPriority,
		},
		{
			Id:       3,
			Type:     constant.ChannelTypeGemini,
			Key:      "secret-idle",
			Name:     "Idle channel",
			Status:   common.ChannelStatusEnabled,
			Models:   "gemini-2.5-pro",
			Group:    "default",
			Priority: &lowPriority,
		},
		{
			Id:       4,
			Type:     constant.ChannelTypeOpenRouter,
			Key:      "secret-alert",
			Name:     "OpenRouter outage",
			Status:   common.ChannelStatusEnabled,
			Models:   "openrouter/auto",
			Group:    "default",
			Priority: &lowPriority,
		},
	}
	require.NoError(t, model.DB.Create(&channels).Error)

	loadedChannels, err := model.GetChannelsForServiceStatus()
	require.NoError(t, err)
	for _, channel := range loadedChannels {
		require.Empty(t, channel.Key)
	}

	bucketTs := time.Now().Add(-1 * time.Hour).Unix()
	require.NoError(t, model.UpsertPerfChannelMetric(&model.PerfChannelMetric{
		ChannelId:      1,
		ModelName:      "gpt-4o",
		Group:          "default",
		BucketTs:       bucketTs,
		RequestCount:   20,
		SuccessCount:   20,
		TotalLatencyMs: 20 * 250,
		TtftSumMs:      20 * 90,
		TtftCount:      20,
	}))
	require.NoError(t, model.UpsertPerfChannelMetric(&model.PerfChannelMetric{
		ChannelId:      2,
		ModelName:      "claude-3-5-sonnet",
		Group:          "vip",
		BucketTs:       bucketTs,
		RequestCount:   20,
		SuccessCount:   5,
		TotalLatencyMs: 20 * 500,
		TtftSumMs:      20 * 120,
		TtftCount:      20,
	}))
	require.NoError(t, model.UpsertPerfChannelMetric(&model.PerfChannelMetric{
		ChannelId:      4,
		ModelName:      "openrouter/auto",
		Group:          "default",
		BucketTs:       bucketTs,
		RequestCount:   10,
		SuccessCount:   5,
		TotalLatencyMs: 10 * 700,
		TtftSumMs:      10 * 180,
		TtftCount:      10,
	}))
	require.NoError(t, model.UpsertPerfChannelMetric(&model.PerfChannelMetric{
		ChannelId:      4,
		ModelName:      "openrouter/auto",
		Group:          "default",
		BucketTs:       bucketTs,
		RequestCount:   10,
		SuccessCount:   5,
		TotalLatencyMs: 10 * 700,
		TtftSumMs:      10 * 180,
		TtftCount:      10,
	}))

	regularResult, err := Query(QueryOptions{Hours: 24})
	require.NoError(t, err)
	require.Equal(t, 4, regularResult.Summary.TotalChannels)
	require.Equal(t, 1, regularResult.Summary.Normal)
	require.Equal(t, 2, regularResult.Summary.Outage)
	require.Equal(t, 1, regularResult.Summary.NoTraffic)
	require.Zero(t, regularResult.Summary.ActiveAlerts)
	require.Empty(t, regularResult.Alerts)

	byId := make(map[int]ChannelStatus)
	for _, channel := range regularResult.Channels {
		byId[channel.ChannelId] = channel
	}
	require.Equal(t, StatusNormal, byId[1].Status)
	require.Equal(t, ConfidenceHigh, byId[1].Confidence)
	require.Equal(t, int64(20), byId[1].RequestCount)
	require.Equal(t, 100.0, byId[1].SuccessRate)
	require.Equal(t, int64(250), byId[1].AvgLatencyMs)
	require.Equal(t, []string{"gpt-4o", "gpt-4o-mini"}, byId[1].ConfiguredModels)
	require.Equal(t, []string{"gpt-4o"}, byId[1].ObservedModels)
	require.Equal(t, SignalObserved, byId[1].Signals.RealTraffic)
	require.Equal(t, SignalNotConfigured, byId[1].Signals.Probe)
	require.Equal(t, SignalNotConfigured, byId[1].Signals.Connectivity)
	require.Len(t, byId[1].Series, 1)

	require.Equal(t, StatusOutage, byId[2].Status)
	require.Equal(t, StatusNoTraffic, byId[3].Status)
	require.Equal(t, ConfidenceNone, byId[3].Confidence)
	require.Empty(t, byId[3].ObservedModels)
	require.Equal(t, StatusOutage, byId[4].Status)
	require.Equal(t, int64(20), byId[4].RequestCount)
	require.Equal(t, 50.0, byId[4].SuccessRate)

	adminResult, err := Query(QueryOptions{Hours: 24, IncludeAlerts: true})
	require.NoError(t, err)
	require.Equal(t, 1, adminResult.Summary.ActiveAlerts)
	require.Len(t, adminResult.Alerts, 1)
	require.Equal(t, 4, adminResult.Alerts[0].ChannelId)
	require.Equal(t, "critical", adminResult.Alerts[0].Severity)
}

func TestNormalizeHoursUsesDefaultAndMaximum(t *testing.T) {
	require.Equal(t, defaultWindowHours, normalizeHours(0))
	require.Equal(t, defaultWindowHours, normalizeHours(-3))
	require.Equal(t, 6, normalizeHours(6))
	require.Equal(t, maxWindowHours, normalizeHours(maxWindowHours+1))
}
