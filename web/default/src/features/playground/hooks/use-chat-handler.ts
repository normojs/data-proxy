/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { useCallback } from 'react'
import { toast } from 'sonner'
import { sendChatCompletion } from '../api'
import {
  MESSAGE_STATUS,
  ERROR_MESSAGES,
  PLAYGROUND_ENDPOINTS,
} from '../constants'
import {
  buildPlaygroundPayload,
  updateAssistantMessageWithError,
  updateLastAssistantMessage,
  processStreamingContent,
  finalizeMessage,
} from '../lib'
import type {
  ChatCompletionResponse,
  Message,
  PlaygroundConfig,
  ParameterEnabled,
  PlaygroundResponseDetails,
  PlaygroundResponse,
  ResponsesResponse,
} from '../types'
import { useStreamRequest } from './use-stream-request'

interface UseChatHandlerOptions {
  config: PlaygroundConfig
  parameterEnabled: ParameterEnabled
  onMessageUpdate: (updater: (prev: Message[]) => Message[]) => void
}

function isChatCompletionResponse(
  response: PlaygroundResponse
): response is ChatCompletionResponse {
  return Array.isArray((response as ChatCompletionResponse).choices)
}

function extractResponsesOutputText(response: ResponsesResponse): string {
  const output = Array.isArray(response.output) ? response.output : []
  const parts: string[] = []

  for (const item of output) {
    if (!item || typeof item !== 'object') continue

    const record = item as Record<string, unknown>
    if (record.type === 'output_text' && typeof record.text === 'string') {
      parts.push(record.text)
      continue
    }

    const content = Array.isArray(record.content) ? record.content : []
    for (const part of content) {
      if (!part || typeof part !== 'object') continue
      const contentRecord = part as Record<string, unknown>
      if (
        contentRecord.type === 'output_text' &&
        typeof contentRecord.text === 'string'
      ) {
        parts.push(contentRecord.text)
      }
    }
  }

  return parts.join('')
}

/**
 * Hook for handling chat message sending and receiving
 */
export function useChatHandler({
  config,
  parameterEnabled,
  onMessageUpdate,
}: UseChatHandlerOptions) {
  const { sendStreamRequest, stopStream, isStreaming } = useStreamRequest()

  const handleStreamDetailsUpdate = useCallback(
    (details: PlaygroundResponseDetails) => {
      onMessageUpdate((prev) =>
        updateLastAssistantMessage(prev, (message) => ({
          ...message,
          details,
        }))
      )
    },
    [onMessageUpdate]
  )

  // Handle stream update
  const handleStreamUpdate = useCallback(
    (type: 'reasoning' | 'content', chunk: string) => {
      onMessageUpdate((prev) =>
        updateLastAssistantMessage(prev, (message) => {
          if (message.status === MESSAGE_STATUS.ERROR) return message

          if (type === 'reasoning') {
            // Direct API reasoning_content
            return {
              ...message,
              reasoning: {
                content: (message.reasoning?.content || '') + chunk,
                duration: 0,
              },
              isReasoningStreaming: true,
              status: MESSAGE_STATUS.STREAMING,
            }
          }

          // Content streaming: handle <think> tags
          return {
            ...processStreamingContent(message, chunk),
            status: MESSAGE_STATUS.STREAMING,
          }
        })
      )
    },
    [onMessageUpdate]
  )

  // Handle stream complete
  const handleStreamComplete = useCallback(
    (details?: PlaygroundResponseDetails) => {
      onMessageUpdate((prev) =>
        updateLastAssistantMessage(prev, (message) =>
          message.status === MESSAGE_STATUS.COMPLETE ||
          message.status === MESSAGE_STATUS.ERROR
            ? message
            : {
                ...finalizeMessage(message),
                status: MESSAGE_STATUS.COMPLETE,
                details: details ?? message.details,
              }
        )
      )
    },
    [onMessageUpdate]
  )

  // Handle stream error
  const handleStreamError = useCallback(
    (
      error: string,
      errorCode?: string,
      details?: PlaygroundResponseDetails
    ) => {
      toast.error(error)
      onMessageUpdate((prev) =>
        updateAssistantMessageWithError(prev, error, errorCode, details)
      )
    },
    [onMessageUpdate]
  )

  // Send streaming chat request
  const sendStreamingChat = useCallback(
    (messages: Message[]) => {
      const payload = buildPlaygroundPayload(
        messages,
        config,
        parameterEnabled
      )
      sendStreamRequest(
        payload,
        config.endpoint,
        handleStreamUpdate,
        handleStreamComplete,
        handleStreamError,
        handleStreamDetailsUpdate
      )
    },
    [
      config,
      parameterEnabled,
      sendStreamRequest,
      handleStreamUpdate,
      handleStreamComplete,
      handleStreamError,
      handleStreamDetailsUpdate,
    ]
  )

  // Send non-streaming chat request
  const sendNonStreamingChat = useCallback(
    async (messages: Message[]) => {
      const payload = buildPlaygroundPayload(
        messages,
        config,
        parameterEnabled
      )

      try {
        const result = await sendChatCompletion(payload, config.endpoint)
        const response = result.data

        if (
          config.endpoint === PLAYGROUND_ENDPOINTS.RESPONSES &&
          !isChatCompletionResponse(response)
        ) {
          if (response.error?.message) {
            handleStreamError(
              response.error.message,
              response.error.code,
              result.details
            )
            return
          }

          const content = extractResponsesOutputText(response)
          onMessageUpdate((prev) =>
            updateLastAssistantMessage(prev, (message) => ({
              ...finalizeMessage({
                ...message,
                versions: [
                  {
                    ...message.versions[0],
                    content,
                  },
                ],
              }),
              status: MESSAGE_STATUS.COMPLETE,
              details: result.details,
            }))
          )
          return
        }

        if (!isChatCompletionResponse(response)) {
          handleStreamError(ERROR_MESSAGES.PARSE_ERROR, undefined, result.details)
          return
        }

        const choice = response.choices?.[0]
        if (!choice) {
          handleStreamError(ERROR_MESSAGES.PARSE_ERROR, undefined, result.details)
          return
        }

        onMessageUpdate((prev) =>
          updateLastAssistantMessage(prev, (message) => ({
            ...finalizeMessage(
              {
                ...message,
                versions: [
                  {
                    ...message.versions[0],
                    content: choice.message?.content || '',
                  },
                ],
              },
              choice.message?.reasoning_content
            ),
            status: MESSAGE_STATUS.COMPLETE,
            details: result.details,
          }))
        )
      } catch (error: unknown) {
        const err = error as {
          playgroundDetails?: PlaygroundResponseDetails
          response?: {
            data?: {
              message?: string
              error?: { message?: string; code?: string }
              error_code?: string
            }
          }
          message?: string
        }
        handleStreamError(
          err?.response?.data?.message ||
            err?.response?.data?.error?.message ||
            err?.message ||
            ERROR_MESSAGES.API_REQUEST_ERROR,
          err?.response?.data?.error?.code ||
            err?.response?.data?.error_code ||
            undefined,
          err.playgroundDetails
        )
      }
    },
    [config, parameterEnabled, onMessageUpdate, handleStreamError]
  )

  // Send chat request (stream or non-stream based on config)
  const sendChat = useCallback(
    (messages: Message[]) => {
      if (config.stream) {
        sendStreamingChat(messages)
      } else {
        sendNonStreamingChat(messages)
      }
    },
    [config.stream, sendStreamingChat, sendNonStreamingChat]
  )

  // Stop generation
  const stopGeneration = useCallback(() => {
    const details = stopStream()
    onMessageUpdate((prev) =>
      updateLastAssistantMessage(prev, (message) =>
        message.status === MESSAGE_STATUS.LOADING ||
        message.status === MESSAGE_STATUS.STREAMING
          ? {
              ...finalizeMessage(message),
              status: MESSAGE_STATUS.COMPLETE,
              details: details ?? message.details,
            }
          : message
      )
    )
  }, [stopStream, onMessageUpdate])

  return {
    sendChat,
    stopGeneration,
    isGenerating: isStreaming,
  }
}
