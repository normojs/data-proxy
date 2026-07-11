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
import { api } from '@/lib/api'
import { API_ENDPOINTS, PLAYGROUND_ENDPOINTS } from './constants'
import type {
  ChatCompletionResponse,
  ChatCompletionUsage,
  ModelOption,
  GroupOption,
  PlaygroundEndpoint,
  PlaygroundRequest,
  PlaygroundResponse,
  PlaygroundResponseDetails,
  ProviderCheckRequest,
  ProviderCheckResult,
  ResponsesResponse,
} from './types'

export interface ChatCompletionResult {
  data: PlaygroundResponse
  details: PlaygroundResponseDetails
}

interface ErrorWithPlaygroundDetails {
  playgroundDetails?: PlaygroundResponseDetails
  response?: {
    status?: number
    statusText?: string
    headers?: unknown
    data?: unknown
  }
  message?: string
}

interface ApiResponse<T> {
  success: boolean
  message?: string
  data: T
}

function normalizeHeaders(headers: unknown): Record<string, string | string[]> {
  const source =
    headers &&
    typeof (headers as { toJSON?: () => unknown }).toJSON === 'function'
      ? (headers as { toJSON: () => unknown }).toJSON()
      : headers

  if (!source || typeof source !== 'object') return {}

  return Object.entries(source as Record<string, unknown>).reduce<
    Record<string, string | string[]>
  >((acc, [key, value]) => {
    if (Array.isArray(value)) {
      acc[key] = value.map((item) => String(item))
      return acc
    }
    if (value !== undefined && value !== null) {
      acc[key] = String(value)
    }
    return acc
  }, {})
}

function getErrorInfo(data: unknown, fallback?: string) {
  const record =
    data && typeof data === 'object' ? (data as Record<string, unknown>) : {}
  const openAIError =
    record.error && typeof record.error === 'object'
      ? (record.error as Record<string, unknown>)
      : {}

  return {
    message:
      (typeof record.message === 'string' && record.message) ||
      (typeof openAIError.message === 'string' && openAIError.message) ||
      fallback ||
      'Request failed',
    code:
      (typeof openAIError.code === 'string' && openAIError.code) ||
      (typeof record.error_code === 'string' && record.error_code) ||
      undefined,
  }
}

export function getPlaygroundEndpointPath(endpoint: PlaygroundEndpoint): string {
  return endpoint === PLAYGROUND_ENDPOINTS.RESPONSES
    ? API_ENDPOINTS.RESPONSES
    : API_ENDPOINTS.CHAT_COMPLETIONS
}

function isChatCompletionResponse(
  data: PlaygroundResponse
): data is ChatCompletionResponse {
  return Array.isArray((data as ChatCompletionResponse).choices)
}

function responsesUsageToChatUsage(
  usage?: ResponsesResponse['usage']
): ChatCompletionUsage | undefined {
  if (!usage) return undefined
  return {
    prompt_tokens: usage.input_tokens ?? 0,
    completion_tokens: usage.output_tokens ?? 0,
    total_tokens: usage.total_tokens ?? 0,
  }
}

function getResponseDetailsPatch(
  data: PlaygroundResponse,
  endpoint: PlaygroundEndpoint
): Partial<PlaygroundResponseDetails> {
  if (isChatCompletionResponse(data)) {
    const choice = data.choices?.[0]
    return {
      response_id: data.id,
      object: data.object,
      created: data.created,
      model: data.model,
      finish_reason: choice?.finish_reason ?? null,
      usage: data.usage,
    }
  }

  return {
    response_id: data.id,
    object: data.object,
    created: data.created_at,
    model: data.model,
    finish_reason:
      endpoint === PLAYGROUND_ENDPOINTS.RESPONSES ? data.status || null : null,
    usage: responsesUsageToChatUsage(data.usage),
  }
}

/**
 * Send playground request (non-streaming)
 */
export async function sendChatCompletion(
  payload: PlaygroundRequest,
  endpoint: PlaygroundEndpoint = PLAYGROUND_ENDPOINTS.CHAT_COMPLETIONS
): Promise<ChatCompletionResult> {
  const startedAtMs = Date.now()
  const startedAt = new Date(startedAtMs).toISOString()
  const endpointPath = getPlaygroundEndpointPath(endpoint)

  try {
    const res = await api.post(endpointPath, payload, {
      skipErrorHandler: true,
    } as Record<string, unknown>)
    const completedAtMs = Date.now()
    const data = res.data as PlaygroundResponse

    return {
      data,
      details: {
        mode: 'non_stream',
        endpoint: endpointPath,
        protocol: endpoint,
        request: payload,
        started_at: startedAt,
        completed_at: new Date(completedAtMs).toISOString(),
        duration_ms: completedAtMs - startedAtMs,
        http_status: res.status,
        http_status_text: res.statusText,
        response_headers: normalizeHeaders(res.headers),
        ...getResponseDetailsPatch(data, endpoint),
        raw_response: data,
      },
    }
  } catch (error: unknown) {
    const err = error as ErrorWithPlaygroundDetails
    const completedAtMs = Date.now()
    const errorInfo = getErrorInfo(err.response?.data, err.message)

    err.playgroundDetails = {
      mode: 'non_stream',
      endpoint: endpointPath,
      protocol: endpoint,
      request: payload,
      started_at: startedAt,
      completed_at: new Date(completedAtMs).toISOString(),
      duration_ms: completedAtMs - startedAtMs,
      http_status: err.response?.status,
      http_status_text: err.response?.statusText,
      response_headers: normalizeHeaders(err.response?.headers),
      raw_response: err.response?.data,
      error: {
        message: errorInfo.message,
        code: errorInfo.code,
        status: err.response?.status,
        raw: err.response?.data,
      },
    }

    throw error
  }
}

export async function checkPlaygroundProvider(
  payload: ProviderCheckRequest
): Promise<ProviderCheckResult> {
  const res = await api.post<ApiResponse<ProviderCheckResult>>(
    API_ENDPOINTS.PROVIDER_CHECK,
    payload,
    {
      skipBusinessError: true,
      skipErrorHandler: true,
    }
  )

  if (!res.data.success) {
    throw new Error(res.data.message || 'Provider check failed')
  }

  return res.data.data
}

/**
 * Get user available models
 */
export async function getUserModels(): Promise<ModelOption[]> {
  const res = await api.get(API_ENDPOINTS.USER_MODELS)
  const { data } = res

  if (!data.success || !Array.isArray(data.data)) {
    return []
  }

  return data.data.map((model: string) => ({
    label: model,
    value: model,
  }))
}

/**
 * Get user groups
 */
export async function getUserGroups(): Promise<GroupOption[]> {
  const res = await api.get(API_ENDPOINTS.USER_GROUPS)
  const { data } = res

  if (!data.success || !data.data) {
    return []
  }

  const groupData = data.data as Record<string, { desc: string; ratio: number }>

  // label is for button display (name only); desc is for dropdown content
  return Object.entries(groupData).map(([group, info]) => ({
    label: group,
    value: group,
    ratio: info.ratio,
    desc: info.desc,
  }))
}
