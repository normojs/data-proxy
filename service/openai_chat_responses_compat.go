package service

import (
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/service/openaicompat"
)

func ChatCompletionsRequestToResponsesRequest(req *dto.GeneralOpenAIRequest) (*dto.OpenAIResponsesRequest, error) {
	return openaicompat.ChatCompletionsRequestToResponsesRequest(req)
}

func ResponsesResponseToChatCompletionsResponse(resp *dto.OpenAIResponsesResponse, id string) (*dto.OpenAITextResponse, *dto.Usage, error) {
	return openaicompat.ResponsesResponseToChatCompletionsResponse(resp, id)
}

func ExtractOutputTextFromResponses(resp *dto.OpenAIResponsesResponse) string {
	return openaicompat.ExtractOutputTextFromResponses(resp)
}

func ResponsesStreamBodyToResponse(body []byte) (*dto.OpenAIResponsesResponse, bool, error) {
	return openaicompat.ResponsesStreamBodyToResponse(body)
}

func ChatCompletionsStreamBodyToResponses(body []byte, req *dto.OpenAIResponsesRequest, ctx *openaicompat.ResponsesToChatContext) (map[string]any, *dto.Usage, bool, error) {
	return openaicompat.ChatCompletionsStreamBodyToResponses(body, req, ctx)
}

func ResponsesRequestToChatCompletionsRequest(req *dto.OpenAIResponsesRequest) (*dto.GeneralOpenAIRequest, *openaicompat.ResponsesToChatContext, error) {
	return openaicompat.ResponsesRequestToChatCompletionsRequest(req)
}

func ResponsesRequestToChatCompletionsRequestWithOptions(req *dto.OpenAIResponsesRequest, opts *openaicompat.ResponsesToChatOptions) (*dto.GeneralOpenAIRequest, *openaicompat.ResponsesToChatContext, error) {
	return openaicompat.ResponsesRequestToChatCompletionsRequestWithOptions(req, opts)
}

func ChatCompletionResponseToResponses(resp *dto.OpenAITextResponse, req *dto.OpenAIResponsesRequest, ctx *openaicompat.ResponsesToChatContext) (map[string]any, *dto.Usage, error) {
	return openaicompat.ChatCompletionResponseToResponses(resp, req, ctx)
}

func NewResponsesToChatStreamConverter(responseID string, createdAt int64, model string) *openaicompat.ResponsesToChatStreamConverter {
	return openaicompat.NewResponsesToChatStreamConverter(responseID, createdAt, model)
}
