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
import { useMemo, useState, type ReactNode } from 'react'
import { Check, ChevronDown, Copy } from 'lucide-react'
import { cn } from '@/lib/utils'
import { useCopyToClipboard } from '@/hooks/use-copy-to-clipboard'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from '@/components/ui/collapsible'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import type { PlaygroundResponseDetails } from '../types'

interface MessageDetailsProps {
  details: PlaygroundResponseDetails
  className?: string
}

function formatJsonValue(value: unknown): string {
  if (typeof value === 'string') {
    const trimmed = value.trim()
    if (!trimmed) return ''
    try {
      return JSON.stringify(JSON.parse(trimmed), null, 2)
    } catch {
      return value
    }
  }

  try {
    return JSON.stringify(value ?? {}, null, 2)
  } catch {
    return String(value ?? '')
  }
}

function formatDuration(durationMs?: number): string {
  if (durationMs === undefined) return '-'
  if (durationMs < 1000) return `${durationMs} ms`
  return `${(durationMs / 1000).toFixed(2)} s`
}

function formatTimestamp(value?: string): string {
  if (!value) return '-'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString()
}

function getStatusLabel(details: PlaygroundResponseDetails): string {
  if (details.error) return 'Error'
  if (details.aborted) return 'Stopped'
  if (details.completed_at) return 'Complete'
  return 'Running'
}

function getResponseSummary(details: PlaygroundResponseDetails) {
  return {
    id: details.response_id,
    object: details.object,
    created: details.created,
    model: details.model,
    finish_reason: details.finish_reason,
    usage: details.usage,
    error: details.error,
  }
}

function DetailField({ label, value }: { label: string; value: ReactNode }) {
  const displayValue =
    value === undefined || value === null || value === '' ? '-' : value

  return (
    <div className='min-w-0 space-y-1'>
      <dt className='text-muted-foreground text-[11px] font-medium'>{label}</dt>
      <dd className='min-w-0 font-mono text-xs break-all'>{displayValue}</dd>
    </div>
  )
}

function JsonBlock({
  copiedText,
  title,
  value,
  onCopy,
}: {
  copiedText: string | null
  title: string
  value: unknown
  onCopy: (text: string) => void
}) {
  const content = useMemo(() => formatJsonValue(value), [value])
  const isCopied = copiedText === content

  return (
    <section className='bg-background/70 overflow-hidden rounded-lg border'>
      <div className='bg-muted/30 flex min-h-8 items-center justify-between gap-2 border-b px-3 py-1.5'>
        <h4 className='text-xs font-medium'>{title}</h4>
        <Button
          type='button'
          variant='ghost'
          size='icon'
          className='size-7'
          onClick={() => onCopy(content || '-')}
          aria-label={`Copy ${title}`}
          title={`Copy ${title}`}
        >
          {isCopied ? (
            <Check className='size-3.5 text-green-600' />
          ) : (
            <Copy className='size-3.5' />
          )}
        </Button>
      </div>
      <ScrollArea className='max-h-72 overflow-y-auto'>
        <pre className='m-0 min-w-0 overflow-x-auto p-3 font-mono text-xs leading-relaxed break-words whitespace-pre-wrap'>
          {content || '-'}
        </pre>
      </ScrollArea>
    </section>
  )
}

export function MessageDetails({ details, className }: MessageDetailsProps) {
  const [open, setOpen] = useState(false)
  const { copiedText, copyToClipboard } = useCopyToClipboard({ notify: false })

  const responseValue = details.raw_response ?? getResponseSummary(details)
  const chunkTotal = details.chunk_count ?? 0
  const storedChunkTotal = details.stored_chunk_count ?? 0
  const statusLabel = getStatusLabel(details)
  const statusVariant = details.error
    ? 'destructive'
    : details.completed_at
      ? 'outline'
      : 'secondary'
  const httpVariant =
    details.http_status && details.http_status >= 400
      ? 'destructive'
      : 'outline'

  return (
    <Collapsible
      open={open}
      onOpenChange={(nextOpen: boolean) => setOpen(nextOpen)}
      className={cn(
        'bg-muted/20 mt-2 max-w-full overflow-hidden rounded-lg border text-xs',
        className
      )}
    >
      <CollapsibleTrigger className='hover:bg-muted/60 focus-visible:border-ring focus-visible:ring-ring/50 flex w-full items-center justify-between gap-3 px-2.5 py-2 text-left transition-colors focus-visible:ring-[3px] focus-visible:outline-1'>
        <div className='flex min-w-0 flex-wrap items-center gap-1.5'>
          <span className='font-medium'>Details</span>
          <Badge variant={statusVariant}>{statusLabel}</Badge>
          <Badge variant='outline'>
            {details.mode === 'stream' ? 'Stream' : 'Non-stream'}
          </Badge>
          {details.http_status !== undefined && (
            <Badge variant={httpVariant}>HTTP {details.http_status}</Badge>
          )}
          {details.duration_ms !== undefined && (
            <Badge variant='outline'>
              {formatDuration(details.duration_ms)}
            </Badge>
          )}
          {details.model && (
            <span className='text-muted-foreground max-w-56 truncate font-mono'>
              {details.model}
            </span>
          )}
          {details.finish_reason && (
            <Badge variant='outline'>{details.finish_reason}</Badge>
          )}
          {details.usage && (
            <Badge variant='outline'>{details.usage.total_tokens} tokens</Badge>
          )}
          {details.mode === 'stream' && (
            <Badge variant='outline'>
              {storedChunkTotal}/{chunkTotal} chunks
              {details.truncated_chunks ? ' shown' : ''}
            </Badge>
          )}
        </div>
        <ChevronDown
          className={cn(
            'text-muted-foreground size-4 shrink-0 transition-transform',
            open && 'rotate-180'
          )}
        />
      </CollapsibleTrigger>

      <CollapsibleContent className='max-h-[min(70svh,44rem)] overflow-y-auto overscroll-contain border-t'>
        <Tabs defaultValue='overview' className='gap-0'>
          <TabsList
            variant='line'
            className='max-w-full justify-start overflow-x-auto rounded-none border-b bg-transparent px-2'
          >
            <TabsTrigger value='overview'>Overview</TabsTrigger>
            <TabsTrigger value='request'>Request</TabsTrigger>
            <TabsTrigger value='response'>Response</TabsTrigger>
            {details.mode === 'stream' && (
              <TabsTrigger value='chunks'>Chunks</TabsTrigger>
            )}
            <TabsTrigger value='headers'>Headers</TabsTrigger>
          </TabsList>

          <TabsContent value='overview' className='p-3'>
            <dl className='grid gap-3 sm:grid-cols-2'>
              <DetailField label='Endpoint' value={details.endpoint} />
              <DetailField label='HTTP status' value={details.http_status} />
              <DetailField
                label='HTTP status text'
                value={details.http_status_text}
              />
              <DetailField
                label='Duration'
                value={formatDuration(details.duration_ms)}
              />
              <DetailField
                label='Started'
                value={formatTimestamp(details.started_at)}
              />
              <DetailField
                label='Completed'
                value={formatTimestamp(details.completed_at)}
              />
              <DetailField label='Response ID' value={details.response_id} />
              <DetailField label='Object' value={details.object} />
              <DetailField label='Model' value={details.model} />
              <DetailField
                label='Finish reason'
                value={details.finish_reason}
              />
              <DetailField
                label='Prompt tokens'
                value={details.usage?.prompt_tokens}
              />
              <DetailField
                label='Completion tokens'
                value={details.usage?.completion_tokens}
              />
              <DetailField
                label='Total tokens'
                value={details.usage?.total_tokens}
              />
              {details.mode === 'stream' && (
                <>
                  <DetailField
                    label='Stream ready state'
                    value={details.stream_ready_state}
                  />
                  <DetailField label='Chunk count' value={chunkTotal} />
                  <DetailField
                    label='Stored chunks'
                    value={`${storedChunkTotal}${details.truncated_chunks ? ' (truncated)' : ''}`}
                  />
                </>
              )}
              {details.error && (
                <>
                  <DetailField label='Error code' value={details.error.code} />
                  <DetailField
                    label='Error message'
                    value={details.error.message}
                  />
                </>
              )}
            </dl>
          </TabsContent>

          <TabsContent value='request' className='p-3'>
            <JsonBlock
              copiedText={copiedText}
              title='Request JSON'
              value={details.request}
              onCopy={copyToClipboard}
            />
          </TabsContent>

          <TabsContent value='response' className='p-3'>
            <JsonBlock
              copiedText={copiedText}
              title='Response JSON'
              value={responseValue}
              onCopy={copyToClipboard}
            />
          </TabsContent>

          {details.mode === 'stream' && (
            <TabsContent value='chunks' className='space-y-2 p-3'>
              {details.truncated_chunks && (
                <p className='text-muted-foreground text-xs'>
                  Showing the first {storedChunkTotal} of {chunkTotal} stream
                  events.
                </p>
              )}
              <JsonBlock
                copiedText={copiedText}
                title='Raw stream chunks'
                value={details.raw_chunks ?? []}
                onCopy={copyToClipboard}
              />
            </TabsContent>
          )}

          <TabsContent value='headers' className='p-3'>
            <JsonBlock
              copiedText={copiedText}
              title='Response headers'
              value={details.response_headers ?? {}}
              onCopy={copyToClipboard}
            />
          </TabsContent>
        </Tabs>
      </CollapsibleContent>
    </Collapsible>
  )
}
