export function mcpQueryError(message: string | undefined, fallbackKey: string) {
  const error = new Error(message || fallbackKey)
  error.name = message ? 'MCPQueryError' : 'MCPQueryFallbackError'
  return error
}

export function mcpQueryErrorMessage(
  error: unknown,
  fallbackMessage: string
) {
  if (!(error instanceof Error) || !error.message) return fallbackMessage
  if (error.name === 'MCPQueryFallbackError') return fallbackMessage
  return error.message
}
