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
import type {
  ChatCompletionRequest,
  Message,
  PlaygroundConfig,
  ParameterEnabled,
  ResponsesInputMessage,
  ResponsesRequest,
  PlaygroundRequest,
} from '../types'
import {
  formatMessageForAPI,
  getCurrentVersion,
  isValidMessage,
} from './message-utils'

/**
 * Build API request payload from messages and config
 */
export function buildChatCompletionPayload(
  messages: Message[],
  config: PlaygroundConfig,
  parameterEnabled: ParameterEnabled
): ChatCompletionRequest {
  // Filter and format valid messages
  const processedMessages = messages
    .filter(isValidMessage)
    .map(formatMessageForAPI)

  const payload: ChatCompletionRequest = {
    model: config.model,
    group: config.group,
    messages: processedMessages,
    stream: config.stream,
  }

  // Add enabled parameters
  const parameterKeys: Array<keyof ParameterEnabled> = [
    'temperature',
    'top_p',
    'max_tokens',
    'frequency_penalty',
    'presence_penalty',
    'seed',
  ]

  parameterKeys.forEach((key) => {
    if (parameterEnabled[key]) {
      const value = config[key as keyof PlaygroundConfig]
      if (value !== undefined && value !== null) {
        ;(payload as unknown as Record<string, unknown>)[key] = value
      }
    }
  })

  return payload
}

function formatMessageForResponsesInput(
  message: Message
): ResponsesInputMessage | null {
  if (message.from === 'system') return null

  const content = getCurrentVersion(message).content
  const type = message.from === 'assistant' ? 'output_text' : 'input_text'

  return {
    type: 'message',
    role: message.from,
    content: [
      {
        type,
        text: content,
      },
    ],
  }
}

export function buildResponsesPayload(
  messages: Message[],
  config: PlaygroundConfig,
  parameterEnabled: ParameterEnabled
): ResponsesRequest {
  const validMessages = messages.filter(isValidMessage)
  const instructions = validMessages
    .filter((message) => message.from === 'system')
    .map((message) => getCurrentVersion(message).content.trim())
    .filter(Boolean)
    .join('\n\n')

  const input = validMessages
    .map(formatMessageForResponsesInput)
    .filter((message): message is ResponsesInputMessage => message !== null)

  const payload: ResponsesRequest = {
    model: config.model,
    group: config.group,
    input,
    stream: config.stream,
  }

  if (instructions) {
    payload.instructions = instructions
  }

  if (parameterEnabled.temperature) {
    payload.temperature = config.temperature
  }
  if (parameterEnabled.top_p) {
    payload.top_p = config.top_p
  }
  if (parameterEnabled.max_tokens && config.max_tokens > 0) {
    payload.max_output_tokens = config.max_tokens
  }

  return payload
}

export function buildPlaygroundPayload(
  messages: Message[],
  config: PlaygroundConfig,
  parameterEnabled: ParameterEnabled
): PlaygroundRequest {
  if (config.endpoint === 'responses') {
    return buildResponsesPayload(messages, config, parameterEnabled)
  }
  return buildChatCompletionPayload(messages, config, parameterEnabled)
}
