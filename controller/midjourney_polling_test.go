package controller

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/stretchr/testify/require"
)

func TestFetchMidjourneyTaskList(t *testing.T) {
	service.InitHttpClient()

	requestErr := make(chan error, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			requestErr <- fmt.Errorf("method = %s", r.Method)
			http.Error(w, "bad method", http.StatusBadRequest)
			return
		}
		if r.URL.Path != "/mj/task/list-by-condition" {
			requestErr <- fmt.Errorf("path = %s", r.URL.Path)
			http.Error(w, "bad path", http.StatusBadRequest)
			return
		}
		if r.Header.Get("mj-api-secret") != "mj-secret" {
			requestErr <- fmt.Errorf("mj-api-secret = %s", r.Header.Get("mj-api-secret"))
			http.Error(w, "bad secret", http.StatusBadRequest)
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			requestErr <- err
			http.Error(w, "bad body", http.StatusBadRequest)
			return
		}
		var payload map[string][]string
		if err := common.Unmarshal(body, &payload); err != nil {
			requestErr <- err
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}
		if got := fmt.Sprint(payload["ids"]); got != "[mj-1]" {
			requestErr <- fmt.Errorf("ids = %s", got)
			http.Error(w, "bad ids", http.StatusBadRequest)
			return
		}
		requestErr <- nil

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"id":"mj-1","progress":"50%","status":"IN_PROGRESS","videoUrls":[{"url":"https://example.test/video.mp4"}]}]`))
	}))
	t.Cleanup(server.Close)

	items, err := fetchMidjourneyTaskList(context.Background(), &model.Channel{
		BaseURL: &server.URL,
		Key:     "mj-secret",
	}, []string{"mj-1"})

	require.NoError(t, err)
	require.NoError(t, <-requestErr)
	require.Len(t, items, 1)
	require.Equal(t, "mj-1", items[0].MjId)
	require.Equal(t, "50%", items[0].Progress)
	require.Equal(t, "IN_PROGRESS", items[0].Status)
	require.Len(t, items[0].VideoUrls, 1)
	require.Equal(t, "https://example.test/video.mp4", items[0].VideoUrls[0].Url)
}

func TestFetchMidjourneyTaskListReturnsStatusError(t *testing.T) {
	service.InitHttpClient()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(`upstream unavailable`))
	}))
	t.Cleanup(server.Close)

	_, err := fetchMidjourneyTaskList(context.Background(), &model.Channel{
		BaseURL: &server.URL,
		Key:     "mj-secret",
	}, []string{"mj-1"})

	require.Error(t, err)
	require.Contains(t, err.Error(), "task list status code: 502")
}

func TestFetchMidjourneyTaskListReturnsParseError(t *testing.T) {
	service.InitHttpClient()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`not json`))
	}))
	t.Cleanup(server.Close)

	_, err := fetchMidjourneyTaskList(context.Background(), &model.Channel{
		BaseURL: &server.URL,
		Key:     "mj-secret",
	}, []string{"mj-1"})

	require.Error(t, err)
	require.Contains(t, err.Error(), "parse task list response body")
}

func TestFetchMidjourneyTaskListRejectsMissingBaseURL(t *testing.T) {
	_, err := fetchMidjourneyTaskList(context.Background(), &model.Channel{}, []string{"mj-1"})

	require.Error(t, err)
	require.Contains(t, err.Error(), "base_url is empty")
}
