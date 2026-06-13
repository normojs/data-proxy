package vertex

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	geminitask "github.com/QuantumNous/new-api/relay/channel/task/gemini"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestVertexBuildRequestBodyIncludesVeoAdvancedImages(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", nil)
	c.Set("task_request", relaycommon.TaskSubmitReq{
		Prompt: "animate",
		Images: []string{tinyPNGBase64ForVertex},
		Metadata: map[string]interface{}{
			"lastFrame": tinyPNGBase64ForVertex,
			"referenceImages": []any{
				tinyPNGBase64ForVertex,
			},
		},
	})
	info := &relaycommon.RelayInfo{
		ChannelMeta:   &relaycommon.ChannelMeta{UpstreamModelName: "veo-3.1-fast-generate-preview"},
		TaskRelayInfo: &relaycommon.TaskRelayInfo{},
	}

	body, err := (&TaskAdaptor{}).BuildRequestBody(c, info)

	require.NoError(t, err)
	data, err := io.ReadAll(body)
	require.NoError(t, err)
	var payload geminitask.VeoRequestPayload
	require.NoError(t, common.Unmarshal(data, &payload))
	require.Len(t, payload.Instances, 1)
	require.NotNil(t, payload.Instances[0].LastFrame)
	require.Len(t, payload.Instances[0].ReferenceImages, 1)
	require.Equal(t, "asset", payload.Instances[0].ReferenceImages[0].ReferenceType)
	require.Equal(t, 8, payload.Parameters.DurationSeconds)
}

const tinyPNGBase64ForVertex = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAIAAACQd1PeAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg=="
