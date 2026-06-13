package gemini

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestApplyVeoAdvancedImageInputsAddsLastFrameAndReferences(t *testing.T) {
	instance := VeoInstance{
		Prompt: "animate",
		Image:  ParseImageInput(tinyPNGBase64),
	}
	params := &VeoParameters{}

	err := ApplyVeoAdvancedImageInputs(&instance, params, map[string]any{
		"lastFrame": tinyPNGBase64,
		"referenceImages": []any{
			tinyPNGBase64,
			map[string]any{
				"image":         tinyPNGBase64,
				"referenceType": "style",
			},
		},
	}, "veo-3.1-generate-preview")

	require.NoError(t, err)
	require.NotNil(t, instance.LastFrame)
	require.Len(t, instance.ReferenceImages, 2)
	require.Equal(t, "asset", instance.ReferenceImages[0].ReferenceType)
	require.Equal(t, "style", instance.ReferenceImages[1].ReferenceType)
	require.Equal(t, defaultVeoReferenceDuration, params.DurationSeconds)
}

func TestApplyVeoAdvancedImageInputsValidatesAdvancedInputs(t *testing.T) {
	tests := []struct {
		name     string
		instance VeoInstance
		params   VeoParameters
		metadata map[string]any
		model    string
		wantErr  string
	}{
		{
			name:     "last frame requires primary image",
			instance: VeoInstance{Prompt: "animate"},
			metadata: map[string]any{"lastFrame": tinyPNGBase64},
			model:    "veo-3.1-generate-preview",
			wantErr:  "primary image",
		},
		{
			name:     "last frame requires veo 3.1",
			instance: VeoInstance{Prompt: "animate", Image: ParseImageInput(tinyPNGBase64)},
			metadata: map[string]any{"lastFrame": tinyPNGBase64},
			model:    "veo-3.0-generate-001",
			wantErr:  "Veo 3.1",
		},
		{
			name:     "reference images require veo 3.1",
			instance: VeoInstance{Prompt: "animate", Image: ParseImageInput(tinyPNGBase64)},
			metadata: map[string]any{"referenceImages": []any{tinyPNGBase64}},
			model:    "veo-3.0-generate-001",
			wantErr:  "Veo 3.1",
		},
		{
			name:     "reference images require 8 second duration",
			instance: VeoInstance{Prompt: "animate", Image: ParseImageInput(tinyPNGBase64)},
			params:   VeoParameters{DurationSeconds: 4},
			metadata: map[string]any{"referenceImages": []any{tinyPNGBase64}},
			model:    "veo-3.1-generate-preview",
			wantErr:  "durationSeconds=8",
		},
		{
			name:     "reference images are capped",
			instance: VeoInstance{Prompt: "animate", Image: ParseImageInput(tinyPNGBase64)},
			metadata: map[string]any{"referenceImages": []any{tinyPNGBase64, tinyPNGBase64, tinyPNGBase64, tinyPNGBase64}},
			model:    "veo-3.1-generate-preview",
			wantErr:  "at most 3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ApplyVeoAdvancedImageInputs(&tt.instance, &tt.params, tt.metadata, tt.model)
			require.Error(t, err)
			require.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestGeminiBuildRequestBodyIncludesAdvancedImages(t *testing.T) {
	c := newVeoTaskContext(t, relaycommon.TaskSubmitReq{
		Prompt: "animate",
		Images: []string{tinyPNGBase64},
		Metadata: map[string]interface{}{
			"last_frame": tinyPNGBase64,
			"reference_images": []any{
				map[string]any{
					"image":          tinyPNGBase64,
					"reference_type": "style",
				},
			},
		},
	})
	info := &relaycommon.RelayInfo{
		ChannelMeta:   &relaycommon.ChannelMeta{UpstreamModelName: "veo-3.1-generate-preview"},
		TaskRelayInfo: &relaycommon.TaskRelayInfo{},
	}

	body, err := (&TaskAdaptor{}).BuildRequestBody(c, info)

	require.NoError(t, err)
	payload := decodeVeoRequestPayload(t, body)
	require.Len(t, payload.Instances, 1)
	require.NotNil(t, payload.Instances[0].Image)
	require.NotNil(t, payload.Instances[0].LastFrame)
	require.Len(t, payload.Instances[0].ReferenceImages, 1)
	require.Equal(t, "style", payload.Instances[0].ReferenceImages[0].ReferenceType)
	require.Equal(t, defaultVeoReferenceDuration, payload.Parameters.DurationSeconds)
}

func newVeoTaskContext(t *testing.T, req relaycommon.TaskSubmitReq) *gin.Context {
	t.Helper()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", nil)
	c.Set("task_request", req)
	return c
}

func decodeVeoRequestPayload(t *testing.T, body io.Reader) VeoRequestPayload {
	t.Helper()
	data, err := io.ReadAll(body)
	require.NoError(t, err)
	var payload VeoRequestPayload
	require.NoError(t, common.Unmarshal(data, &payload))
	return payload
}
