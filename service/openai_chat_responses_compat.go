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

func ResponsesRequestToChatCompletionsRequest(req *dto.OpenAIResponsesRequest) (*dto.GeneralOpenAIRequest, *openaicompat.ResponsesToChatContext, error) {
	return openaicompat.ResponsesRequestToChatCompletionsRequest(req)
}

func ChatCompletionResponseToResponses(resp *dto.OpenAITextResponse, req *dto.OpenAIResponsesRequest, ctx *openaicompat.ResponsesToChatContext) (map[string]any, *dto.Usage, error) {
	return openaicompat.ChatCompletionResponseToResponses(resp, req, ctx)
}
