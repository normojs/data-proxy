package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type channelBalanceRefreshTestResponse struct {
	Success bool                          `json:"success"`
	Message string                        `json:"message"`
	Started bool                          `json:"started"`
	Refresh channelBalanceRefreshSnapshot `json:"refresh"`
}

func TestUpdateAllChannelsBalanceStartsAsyncAndPreventsOverlap(t *testing.T) {
	gin.SetMode(gin.TestMode)
	resetChannelBalanceRefreshForTest(t)

	runnerStarted := make(chan struct{})
	releaseRunner := make(chan struct{})
	channelBalanceRefreshRunner = func() error {
		close(runnerStarted)
		<-releaseRunner
		return nil
	}

	first := requestUpdateAllChannelsBalanceForTest(t)
	require.True(t, first.Success)
	require.True(t, first.Started)
	require.True(t, first.Refresh.Running)
	require.Equal(t, "admin", first.Refresh.Source)
	require.NotZero(t, first.Refresh.StartedAt)

	select {
	case <-runnerStarted:
	case <-time.After(time.Second):
		t.Fatal("channel balance refresh runner did not start")
	}

	second := requestUpdateAllChannelsBalanceForTest(t)
	require.True(t, second.Success)
	require.False(t, second.Started)
	require.True(t, second.Refresh.Running)
	require.Equal(t, first.Refresh.StartedAt, second.Refresh.StartedAt)

	close(releaseRunner)
	require.Eventually(t, func() bool {
		return !getChannelBalanceRefreshSnapshot().Running
	}, time.Second, 10*time.Millisecond)

	finalSnapshot := getChannelBalanceRefreshSnapshot()
	require.False(t, finalSnapshot.Running)
	require.Equal(t, "admin", finalSnapshot.Source)
	require.NotZero(t, finalSnapshot.FinishedAt)
	require.Empty(t, finalSnapshot.LastError)
}

func TestChannelSupportsBalanceQuerySkipsAzure(t *testing.T) {
	require.False(t, channelSupportsBalanceQuery(nil))
	require.False(t, channelSupportsBalanceQuery(&model.Channel{Type: constant.ChannelTypeAzure}))
	require.True(t, channelSupportsBalanceQuery(&model.Channel{Type: constant.ChannelTypeOpenAI}))
}

func TestUpdateChannelBalanceReturnsUnsupportedForAzure(t *testing.T) {
	_, err := updateChannelBalance(&model.Channel{Type: constant.ChannelTypeAzure})

	require.ErrorIs(t, err, errChannelBalanceQueryUnsupported)
	require.Contains(t, err.Error(), "Azure")
}

func resetChannelBalanceRefreshForTest(t *testing.T) {
	t.Helper()
	originalRunner := channelBalanceRefreshRunner
	channelBalanceRefreshMu.Lock()
	channelBalanceRefreshState = channelBalanceRefreshSnapshot{}
	channelBalanceRefreshMu.Unlock()
	t.Cleanup(func() {
		channelBalanceRefreshMu.Lock()
		channelBalanceRefreshState = channelBalanceRefreshSnapshot{}
		channelBalanceRefreshMu.Unlock()
		channelBalanceRefreshRunner = originalRunner
	})
}

func requestUpdateAllChannelsBalanceForTest(t *testing.T) channelBalanceRefreshTestResponse {
	t.Helper()
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/channel/update_balance", nil)

	UpdateAllChannelsBalance(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response channelBalanceRefreshTestResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	return response
}
