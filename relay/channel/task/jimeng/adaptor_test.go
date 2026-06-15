package jimeng

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

func TestParseTaskResultSuccessDone(t *testing.T) {
	adaptor := &TaskAdaptor{}

	taskInfo, err := adaptor.ParseTaskResult([]byte(`{
		"code":10000,
		"message":"success",
		"data":{
			"status":"done",
			"video_url":"https://example.com/video.mp4"
		}
	}`))

	require.NoError(t, err)
	require.Equal(t, 0, taskInfo.Code)
	require.Equal(t, string(model.TaskStatusSuccess), taskInfo.Status)
	require.Equal(t, "100%", taskInfo.Progress)
	require.Equal(t, "https://example.com/video.mp4", taskInfo.Url)
	require.Empty(t, taskInfo.Reason)
}

func TestParseTaskResultSuccessQueued(t *testing.T) {
	adaptor := &TaskAdaptor{}

	taskInfo, err := adaptor.ParseTaskResult([]byte(`{
		"code":10000,
		"message":"success",
		"data":{
			"status":"in_queue"
		}
	}`))

	require.NoError(t, err)
	require.Equal(t, 0, taskInfo.Code)
	require.Equal(t, string(model.TaskStatusQueued), taskInfo.Status)
	require.Equal(t, "10%", taskInfo.Progress)
	require.Empty(t, taskInfo.Reason)
}

func TestParseTaskResultProviderFailurePreservesUpstreamCode(t *testing.T) {
	adaptor := &TaskAdaptor{}

	taskInfo, err := adaptor.ParseTaskResult([]byte(`{
		"code":50401,
		"message":"provider rejected task",
		"data":{
			"status":"failed"
		}
	}`))

	require.NoError(t, err)
	require.Equal(t, 50401, taskInfo.Code)
	require.Equal(t, string(model.TaskStatusFailure), taskInfo.Status)
	require.Equal(t, "100%", taskInfo.Progress)
	require.Equal(t, "provider rejected task", taskInfo.Reason)
}

func TestParseTaskResultProviderFailureStatusDoesNotOverrideCode(t *testing.T) {
	adaptor := &TaskAdaptor{}

	taskInfo, err := adaptor.ParseTaskResult([]byte(`{
		"code":50401,
		"message":"provider rejected task",
		"data":{
			"status":"done",
			"video_url":"https://example.com/video.mp4"
		}
	}`))

	require.NoError(t, err)
	require.Equal(t, 50401, taskInfo.Code)
	require.Equal(t, string(model.TaskStatusFailure), taskInfo.Status)
	require.Equal(t, "100%", taskInfo.Progress)
	require.Equal(t, "provider rejected task", taskInfo.Reason)
	require.Equal(t, "https://example.com/video.mp4", taskInfo.Url)
}

func TestParseTaskResultInvalidJSON(t *testing.T) {
	adaptor := &TaskAdaptor{}

	taskInfo, err := adaptor.ParseTaskResult([]byte(`{`))

	require.Error(t, err)
	require.Nil(t, taskInfo)
}
