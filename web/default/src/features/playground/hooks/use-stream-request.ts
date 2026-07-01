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
import { useCallback, useRef } from 'react'
import { SSE, type ReadyStateEvent, type SSEvent } from 'sse.js'
import { getCommonHeaders } from '@/lib/api'
import { API_ENDPOINTS, ERROR_MESSAGES } from '../constants'
import type {
  ChatCompletionRequest,
  ChatCompletionChunk,
  PlaygroundResponseDetails,
  PlaygroundStreamEventDetail,
} from '../types'

const MAX_STORED_STREAM_EVENTS = 200

function normalizeSseHeaders(
  headers?: Record<string, string[]>
): Record<string, string | string[]> {
  if (!headers) return {}
  return Object.entries(headers).reduce<Record<string, string | string[]>>(
    (acc, [key, value]) => {
      acc[key] = value.length === 1 ? value[0] : value
      return acc
    },
    {}
  )
}

function serializeEventData(data: unknown): string {
  if (typeof data === 'string') return data
  try {
    return JSON.stringify(data)
  } catch {
    return String(data ?? '')
  }
}

function buildStreamEventDetail(
  event: Pick<SSEvent, 'data' | 'id' | 'lastEventId'>,
  index: number,
  data: unknown
): PlaygroundStreamEventDetail {
  return {
    index,
    received_at: new Date().toISOString(),
    event_id: event.lastEventId || event.id || undefined,
    raw: serializeEventData(event.data),
    data,
  }
}

function appendStreamEvent(
  details: PlaygroundResponseDetails,
  event: PlaygroundStreamEventDetail
): PlaygroundResponseDetails {
  const rawChunks = details.raw_chunks ?? []
  const nextRawChunks =
    rawChunks.length < MAX_STORED_STREAM_EVENTS
      ? [...rawChunks, event]
      : rawChunks

  return {
    ...details,
    raw_chunks: nextRawChunks,
    chunk_count: event.index,
    stored_chunk_count: nextRawChunks.length,
    truncated_chunks: event.index > nextRawChunks.length,
  }
}

function getChunkDetailsPatch(
  chunk: ChatCompletionChunk
): Partial<PlaygroundResponseDetails> {
  const patch: Partial<PlaygroundResponseDetails> = {}
  const finishReason = chunk.choices?.find(
    (choice) => choice.finish_reason !== null
  )?.finish_reason

  if (chunk.id) patch.response_id = chunk.id
  if (chunk.object) patch.object = chunk.object
  if (chunk.created) patch.created = chunk.created
  if (chunk.model) patch.model = chunk.model
  if (finishReason !== undefined) patch.finish_reason = finishReason
  if (chunk.usage) patch.usage = chunk.usage

  return patch
}

function getParsedError(data: string): { message?: string; code?: string } {
  try {
    const parsed = JSON.parse(data) as {
      message?: string
      error?: { message?: string; code?: string }
      error_code?: string
    }
    return {
      message: parsed.error?.message || parsed.message,
      code: parsed.error?.code || parsed.error_code,
    }
  } catch {
    return {}
  }
}

/**
 * Hook for handling streaming chat completion requests
 */
export function useStreamRequest() {
  const sseSourceRef = useRef<SSE | null>(null)
  const isStreamCompleteRef = useRef(false)
  const streamDetailsRef = useRef<PlaygroundResponseDetails | null>(null)
  const streamStartedAtMsRef = useRef<number | null>(null)
  const onDetailsUpdateRef = useRef<
    ((details: PlaygroundResponseDetails) => void) | undefined
  >(undefined)

  const updateDetails = useCallback(
    (
      updater: (details: PlaygroundResponseDetails) => PlaygroundResponseDetails
    ) => {
      if (!streamDetailsRef.current) return null

      const next = updater(streamDetailsRef.current)
      streamDetailsRef.current = next
      onDetailsUpdateRef.current?.(next)
      return next
    },
    []
  )

  const patchDetails = useCallback(
    (patch: Partial<PlaygroundResponseDetails>) => {
      return updateDetails((details) => ({ ...details, ...patch }))
    },
    [updateDetails]
  )

  const finalizeDetails = useCallback(
    (patch: Partial<PlaygroundResponseDetails> = {}) => {
      const completedAtMs = Date.now()
      const startedAtMs = streamStartedAtMsRef.current ?? completedAtMs
      return patchDetails({
        completed_at: new Date(completedAtMs).toISOString(),
        duration_ms: completedAtMs - startedAtMs,
        ...patch,
      })
    },
    [patchDetails]
  )

  const clearStreamState = useCallback(() => {
    sseSourceRef.current = null
    streamDetailsRef.current = null
    streamStartedAtMsRef.current = null
    onDetailsUpdateRef.current = undefined
  }, [])

  const sendStreamRequest = useCallback(
    (
      payload: ChatCompletionRequest,
      onUpdate: (type: 'reasoning' | 'content', chunk: string) => void,
      onComplete: (details?: PlaygroundResponseDetails) => void,
      onError: (
        error: string,
        errorCode?: string,
        details?: PlaygroundResponseDetails
      ) => void,
      onDetailsUpdate?: (details: PlaygroundResponseDetails) => void
    ) => {
      const startedAtMs = Date.now()
      const source = new SSE(API_ENDPOINTS.CHAT_COMPLETIONS, {
        headers: getCommonHeaders(),
        method: 'POST',
        payload: JSON.stringify(payload),
      })

      sseSourceRef.current = source
      isStreamCompleteRef.current = false
      streamStartedAtMsRef.current = startedAtMs
      onDetailsUpdateRef.current = onDetailsUpdate
      streamDetailsRef.current = {
        mode: 'stream',
        endpoint: API_ENDPOINTS.CHAT_COMPLETIONS,
        request: payload,
        started_at: new Date(startedAtMs).toISOString(),
        raw_chunks: [],
        chunk_count: 0,
        stored_chunk_count: 0,
      }
      onDetailsUpdate?.(streamDetailsRef.current)

      const closeSource = () => {
        source.close()
        clearStreamState()
      }

      const handleError = (
        errorMessage: string,
        errorCode?: string,
        raw?: unknown
      ) => {
        if (!isStreamCompleteRef.current) {
          isStreamCompleteRef.current = true
          const status =
            source.xhr?.status || streamDetailsRef.current?.http_status
          const finalDetails = finalizeDetails({
            http_status: status,
            http_status_text: source.xhr?.statusText,
            stream_ready_state: source.readyState,
            error: {
              message: errorMessage,
              code: errorCode,
              status,
              raw,
            },
          })
          onError(errorMessage, errorCode, finalDetails ?? undefined)
          closeSource()
        }
      }

      source.addEventListener('open', (e: SSEvent) => {
        patchDetails({
          http_status: e.responseCode || source.xhr?.status,
          http_status_text: source.xhr?.statusText,
          response_headers: normalizeSseHeaders(e.headers),
          stream_ready_state: source.readyState,
        })
      })

      source.addEventListener('message', (e: SSEvent) => {
        const eventIndex = (streamDetailsRef.current?.chunk_count ?? 0) + 1
        if (e.data === '[DONE]') {
          updateDetails((details) =>
            appendStreamEvent(
              details,
              buildStreamEventDetail(e, eventIndex, '[DONE]')
            )
          )
          isStreamCompleteRef.current = true
          const finalDetails = finalizeDetails({
            http_status:
              source.xhr?.status || streamDetailsRef.current?.http_status,
            http_status_text: source.xhr?.statusText,
            stream_ready_state: source.readyState,
          })
          closeSource()
          onComplete(finalDetails ?? undefined)
          return
        }

        try {
          const chunk: ChatCompletionChunk = JSON.parse(e.data)
          updateDetails((details) => ({
            ...appendStreamEvent(
              details,
              buildStreamEventDetail(e, eventIndex, chunk)
            ),
            ...getChunkDetailsPatch(chunk),
          }))
          const delta = chunk.choices?.[0]?.delta

          if (delta) {
            if (delta.reasoning_content) {
              onUpdate('reasoning', delta.reasoning_content)
            }
            if (delta.content) {
              onUpdate('content', delta.content)
            }
          }
        } catch (error) {
          // eslint-disable-next-line no-console
          console.error('Failed to parse SSE message:', error)
          updateDetails((details) =>
            appendStreamEvent(
              details,
              buildStreamEventDetail(e, eventIndex, e.data)
            )
          )
          handleError(ERROR_MESSAGES.PARSE_ERROR, undefined, e.data)
        }
      })

      source.addEventListener('error', (e: SSEvent) => {
        if (isStreamCompleteRef.current) return

        // eslint-disable-next-line no-console
        console.error('SSE Error:', e)
        const rawData = typeof e.data === 'string' ? e.data : undefined
        const parsedError = rawData ? getParsedError(rawData) : {}
        const errorMessage =
          parsedError.message || rawData || ERROR_MESSAGES.API_REQUEST_ERROR
        const errorCode = parsedError.code

        if (rawData) {
          const eventIndex = (streamDetailsRef.current?.chunk_count ?? 0) + 1
          updateDetails((details) =>
            appendStreamEvent(
              details,
              buildStreamEventDetail(e, eventIndex, rawData)
            )
          )
        }
        handleError(errorMessage, errorCode, e.data)
      })

      source.addEventListener('readystatechange', (e: ReadyStateEvent) => {
        const status = source.xhr?.status
        patchDetails({
          http_status: status || streamDetailsRef.current?.http_status,
          http_status_text: source.xhr?.statusText,
          stream_ready_state: e.readyState,
        })
        if (
          e.readyState !== undefined &&
          e.readyState >= 2 &&
          status !== undefined &&
          status !== 200
        ) {
          handleError(`HTTP ${status}: ${ERROR_MESSAGES.CONNECTION_CLOSED}`)
        }
      })

      try {
        source.stream()
      } catch (error: unknown) {
        // eslint-disable-next-line no-console
        console.error('Failed to start SSE stream:', error)
        const finalDetails = finalizeDetails({
          stream_ready_state: source.readyState,
          error: {
            message: ERROR_MESSAGES.STREAM_START_ERROR,
            raw: error,
          },
        })
        onError(
          ERROR_MESSAGES.STREAM_START_ERROR,
          undefined,
          finalDetails ?? undefined
        )
        clearStreamState()
      }
    },
    [clearStreamState, finalizeDetails, patchDetails, updateDetails]
  )

  const stopStream = useCallback(() => {
    if (sseSourceRef.current) {
      const source = sseSourceRef.current
      const finalDetails = finalizeDetails({
        http_status:
          source.xhr?.status || streamDetailsRef.current?.http_status,
        http_status_text: source.xhr?.statusText,
        stream_ready_state: source.readyState,
        aborted: true,
      })
      isStreamCompleteRef.current = true
      sseSourceRef.current.close()
      clearStreamState()
      return finalDetails
    }
    return null
  }, [clearStreamState, finalizeDetails])

  // eslint-disable-next-line react-hooks/refs
  const isStreaming = sseSourceRef.current !== null

  return {
    sendStreamRequest,
    stopStream,
    // eslint-disable-next-line react-hooks/refs
    isStreaming,
  }
}
