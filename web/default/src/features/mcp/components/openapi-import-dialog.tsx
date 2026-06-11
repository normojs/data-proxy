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
import { useMemo, useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { Loader2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Textarea } from '@/components/ui/textarea'
import { StatusBadge } from '@/components/status-badge'
import {
  diffMCPOpenAPI,
  importMCPOpenAPI,
  mcpQueryKeys,
  previewMCPOpenAPI,
} from '../api'
import {
  MCP_TOOL_STATUS,
  getPriceUnitOptions,
  getProxyAuthTypeOptions,
  getToolStatusOptions,
} from '../constants'
import type {
  MCPOpenAPIImportPayload,
  MCPOpenAPIImportResponse,
  MCPOpenAPIPreviewOperation,
  MCPOpenAPIPreviewResponse,
} from '../types'
import {
  normalizeOpenAPISchemaMetrics,
  summarizeOpenAPIImportResult,
} from '../lib/openapi-import-summary'

type OpenAPIImportDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
}

type OpenAPIImportForm = {
  openapiUrl: string
  document: string
  namespace: string
  category: string
  authType: string
  authRef: string
  authHeaderName: string
  updateExisting: boolean
  pricePerCall: string
  priceUnit: string
  freeQuota: string
  status: string
  sortOrder: string
}

const initialForm: OpenAPIImportForm = {
  openapiUrl: '',
  document: '',
  namespace: '',
  category: 'openapi',
  authType: 'none',
  authRef: '',
  authHeaderName: '',
  updateExisting: false,
  pricePerCall: '0',
  priceUnit: 'per_call',
  freeQuota: '0',
  status: String(MCP_TOOL_STATUS.DISABLED),
  sortOrder: '0',
}

function parseNumber(value: string, fallback: number): number {
  const parsed = Number(value)
  return Number.isFinite(parsed) ? parsed : fallback
}

function buildPreviewPayload(form: OpenAPIImportForm) {
  return {
    openapi_url: form.openapiUrl.trim(),
    document: form.document.trim() || undefined,
    namespace: form.namespace.trim(),
    category: form.category.trim(),
  }
}

function buildImportPayload(
  form: OpenAPIImportForm,
  selectedOperations: string[]
): MCPOpenAPIImportPayload {
  const authType = form.authType || 'none'
  const payload: MCPOpenAPIImportPayload = {
    ...buildPreviewPayload(form),
    auth_type: authType,
    update_existing: form.updateExisting,
    free_quota: Math.max(0, Math.trunc(parseNumber(form.freeQuota, 0))),
    price_per_call: Math.max(0, parseNumber(form.pricePerCall, 0)),
    price_unit: form.priceUnit,
    selected_operations: selectedOperations,
    sort_order: Math.trunc(parseNumber(form.sortOrder, 0)),
    status: Math.trunc(parseNumber(form.status, MCP_TOOL_STATUS.DISABLED)),
  }
  if (authType !== 'none') {
    payload.auth_ref = form.authRef.trim()
  }
  if (authType === 'header') {
    payload.auth_header_name = form.authHeaderName.trim()
  }
  return payload
}

function OperationRow(props: {
  checked: boolean
  operation: MCPOpenAPIPreviewOperation
  onCheckedChange: (checked: boolean) => void
}) {
  const { t } = useTranslation()
  const title =
    props.operation.summary ||
    props.operation.operation_id ||
    `${props.operation.method} ${props.operation.path}`

  return (
    <label className='border-input hover:bg-muted/30 flex gap-3 rounded-lg border px-3 py-2.5 text-sm transition-colors'>
      <Checkbox
        checked={props.checked}
        onCheckedChange={(checked) => props.onCheckedChange(checked === true)}
        className='mt-0.5'
      />
      <span className='min-w-0 flex-1 space-y-1'>
        <span className='flex min-w-0 flex-wrap items-center gap-2'>
          <StatusBadge
            label={props.operation.method}
            variant='neutral'
            copyable={false}
          />
          <span className='font-medium'>{title}</span>
          {props.operation.has_request_body && (
            <StatusBadge label={t('Body')} variant='purple' copyable={false} />
          )}
        </span>
        <span className='text-muted-foreground block truncate font-mono text-xs'>
          {props.operation.key}
        </span>
        <span className='text-muted-foreground block truncate font-mono text-xs'>
          {props.operation.tool_name}
        </span>
      </span>
    </label>
  )
}

function MetricCell(props: { label: string; value: number }) {
  return (
    <div className='bg-muted/30 min-w-0 rounded-md px-3 py-2'>
      <div className='text-muted-foreground truncate text-xs'>
        {props.label}
      </div>
      <div className='truncate text-sm font-semibold'>
        {props.value.toLocaleString()}
      </div>
    </div>
  )
}

function OpenAPIPreviewMetrics(props: {
  preview: MCPOpenAPIPreviewResponse
  selectedCount: number
}) {
  const { t } = useTranslation()
  const metrics = normalizeOpenAPISchemaMetrics(props.preview.schema_metrics)

  return (
    <div className='grid gap-2 sm:grid-cols-3 lg:grid-cols-6'>
      <MetricCell label={t('Selected Operations')} value={props.selectedCount} />
      <MetricCell
        label={t('Operation Count')}
        value={metrics.operation_count}
      />
      <MetricCell
        label={t('Importable Tools')}
        value={metrics.imported_tool_count}
      />
      <MetricCell label={t('Schema Count')} value={metrics.schema_count} />
      <MetricCell
        label={t('Unique Schemas')}
        value={metrics.unique_schema_count}
      />
      <MetricCell
        label={t('Reused Schemas')}
        value={metrics.reused_schema_count}
      />
    </div>
  )
}

function ImportResultSummary(props: {
  result: MCPOpenAPIImportResponse
  title?: string
}) {
  const { t } = useTranslation()
  const summary = summarizeOpenAPIImportResult(props.result)
  const changedItems = props.result.updated.filter(
    (item) => (item.changes?.length ?? 0) > 0
  )

  return (
    <div className='border-input bg-muted/20 mt-4 space-y-3 rounded-lg border p-3'>
      {props.title && (
        <div className='text-muted-foreground text-xs font-medium'>
          {props.title}
        </div>
      )}
      <div className='flex flex-wrap items-center gap-2'>
        <StatusBadge
          label={`${t('Imported')}: ${summary.importedCount}`}
          variant='success'
          copyable={false}
        />
        <StatusBadge
          label={`${t('Updated')}: ${summary.updatedCount}`}
          variant='info'
          copyable={false}
        />
        <StatusBadge
          label={`${t('Skipped')}: ${summary.skippedCount}`}
          variant={summary.skippedCount > 0 ? 'warning' : 'neutral'}
          copyable={false}
        />
      </div>

      <div className='grid gap-2 sm:grid-cols-4'>
        <MetricCell
          label={t('Affected Tools')}
          value={summary.affectedToolCount}
        />
        <MetricCell label={t('Changed Tools')} value={summary.changedToolCount} />
        <MetricCell
          label={t('Changed Fields')}
          value={summary.changedFieldCount}
        />
        <MetricCell
          label={t('Skipped Reasons')}
          value={summary.skippedReasons.length}
        />
      </div>

      {changedItems.length > 0 && (
        <div className='space-y-2'>
          <div className='text-muted-foreground text-xs font-medium'>
            {t('Changed Fields')}
          </div>
          <div className='grid gap-2'>
            {changedItems.map((item) => (
              <div key={item.operation_key} className='min-w-0 space-y-1'>
                <div className='truncate font-mono text-xs font-medium'>
                  {item.tool.name}
                </div>
                <div className='grid gap-1'>
                  {(item.changes ?? []).slice(0, 8).map((change) => (
                    <div
                      key={`${item.operation_key}-${change.field}`}
                      className='grid gap-1 rounded-md border px-2 py-1.5 text-xs sm:grid-cols-[120px_minmax(0,1fr)]'
                    >
                      <span className='text-muted-foreground font-medium'>
                        {change.field}
                      </span>
                      <span className='min-w-0 truncate font-mono'>
                        {change.previous || '-'}
                        {' -> '}
                        {change.current || '-'}
                      </span>
                    </div>
                  ))}
                </div>
              </div>
            ))}
          </div>
        </div>
      )}

      {summary.skippedReasons.length > 0 && (
        <div className='space-y-1'>
          <div className='text-muted-foreground text-xs font-medium'>
            {t('Skipped Reasons')}
          </div>
          {summary.skippedReasons.slice(0, 6).map((item) => (
            <div key={item} className='text-muted-foreground text-xs'>
              {item}
            </div>
          ))}
        </div>
      )}
    </div>
  )
}

export function OpenAPIImportDialog(props: OpenAPIImportDialogProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [form, setForm] = useState<OpenAPIImportForm>(initialForm)
  const [preview, setPreview] = useState<MCPOpenAPIPreviewResponse | null>(null)
  const [diffResult, setDiffResult] = useState<MCPOpenAPIImportResponse | null>(
    null
  )
  const [importResult, setImportResult] =
    useState<MCPOpenAPIImportResponse | null>(null)
  const [selectedOperations, setSelectedOperations] = useState<string[]>([])

  const previewMutation = useMutation({
    mutationFn: () => previewMCPOpenAPI(buildPreviewPayload(form)),
    onSuccess: (res) => {
      if (!res.success || !res.data) {
        toast.error(res.message || t('Failed to preview OpenAPI document'))
        return
      }
      setPreview(res.data)
      setDiffResult(null)
      setImportResult(null)
      setSelectedOperations(res.data.operations.map((item) => item.key))
      toast.success(t('OpenAPI document previewed successfully'))
    },
  })

  const diffMutation = useMutation({
    mutationFn: () =>
      diffMCPOpenAPI(buildImportPayload(form, selectedOperations)),
    onSuccess: (res) => {
      if (!res.success || !res.data) {
        toast.error(res.message || t('Failed to diff OpenAPI tools'))
        return
      }
      setDiffResult(res.data)
      setImportResult(null)
      toast.success(t('OpenAPI import diff is ready'))
    },
  })

  const importMutation = useMutation({
    mutationFn: () =>
      importMCPOpenAPI(buildImportPayload(form, selectedOperations)),
    onSuccess: (res) => {
      if (!res.success || !res.data) {
        toast.error(res.message || t('Failed to import OpenAPI tools'))
        return
      }
      toast.success(
        t('OpenAPI tools imported: {{count}}, updated: {{updated}}', {
          count: res.data.imported_count,
          updated: res.data.updated_count,
        })
      )
      setImportResult(res.data)
      setDiffResult(null)
      queryClient.invalidateQueries({ queryKey: mcpQueryKeys.tools() })
    },
  })

  const selectedSet = useMemo(
    () => new Set(selectedOperations),
    [selectedOperations]
  )
  const isBusy =
    previewMutation.isPending ||
    diffMutation.isPending ||
    importMutation.isPending

  const updateForm = (updates: Partial<OpenAPIImportForm>) => {
    setDiffResult(null)
    setImportResult(null)
    setForm((current) => ({ ...current, ...updates }))
  }

  const toggleOperation = (operationKey: string, checked: boolean) => {
    setDiffResult(null)
    setImportResult(null)
    setSelectedOperations((current) =>
      checked
        ? Array.from(new Set([...current, operationKey]))
        : current.filter((item) => item !== operationKey)
    )
  }

  const handleOpenChange = (open: boolean) => {
    if (!open && isBusy) return
    if (!open) {
      setDiffResult(null)
      setImportResult(null)
      setPreview(null)
      setSelectedOperations([])
    }
    props.onOpenChange(open)
  }

  return (
    <Dialog open={props.open} onOpenChange={handleOpenChange}>
      <DialogContent className='max-h-[90vh] max-w-[calc(100%-2rem)] overflow-hidden sm:max-w-4xl'>
        <DialogHeader>
          <DialogTitle>{t('Import OpenAPI Tools')}</DialogTitle>
        </DialogHeader>

        <ScrollArea className='max-h-[70vh] pr-3'>
          <div className='grid gap-3 sm:grid-cols-2'>
            <div className='space-y-1.5 sm:col-span-2'>
              <Label htmlFor='mcp-openapi-url'>{t('OpenAPI URL')}</Label>
              <Input
                id='mcp-openapi-url'
                value={form.openapiUrl}
                onChange={(event) =>
                  updateForm({ openapiUrl: event.target.value })
                }
                placeholder='https://api.example.com/openapi.json'
              />
            </div>

            <div className='space-y-1.5'>
              <Label htmlFor='mcp-openapi-namespace'>{t('Namespace')}</Label>
              <Input
                id='mcp-openapi-namespace'
                value={form.namespace}
                onChange={(event) =>
                  updateForm({ namespace: event.target.value })
                }
                placeholder='github'
              />
            </div>

            <div className='space-y-1.5'>
              <Label htmlFor='mcp-openapi-category'>{t('Category')}</Label>
              <Input
                id='mcp-openapi-category'
                value={form.category}
                onChange={(event) =>
                  updateForm({ category: event.target.value })
                }
              />
            </div>

            <div className='space-y-1.5'>
              <Label htmlFor='mcp-openapi-auth-type'>{t('Auth Type')}</Label>
              <NativeSelect
                id='mcp-openapi-auth-type'
                value={form.authType}
                onChange={(event) =>
                  updateForm({
                    authType: event.target.value,
                    authHeaderName:
                      event.target.value === 'header'
                        ? form.authHeaderName
                        : '',
                  })
                }
                className='w-full'
              >
                {getProxyAuthTypeOptions(t).map((option) => (
                  <NativeSelectOption key={option.value} value={option.value}>
                    {option.label}
                  </NativeSelectOption>
                ))}
              </NativeSelect>
            </div>

            {form.authType !== 'none' && (
              <div className='space-y-1.5'>
                <Label htmlFor='mcp-openapi-auth-ref'>{t('Auth Ref')}</Label>
                <Input
                  id='mcp-openapi-auth-ref'
                  value={form.authRef}
                  placeholder='env:PET_API_TOKEN'
                  onChange={(event) =>
                    updateForm({ authRef: event.target.value })
                  }
                />
              </div>
            )}

            {form.authType === 'header' && (
              <div className='space-y-1.5'>
                <Label htmlFor='mcp-openapi-auth-header-name'>
                  {t('Header Name')}
                </Label>
                <Input
                  id='mcp-openapi-auth-header-name'
                  value={form.authHeaderName}
                  placeholder='X-API-Key'
                  onChange={(event) =>
                    updateForm({ authHeaderName: event.target.value })
                  }
                />
              </div>
            )}

            <label className='border-input flex items-center gap-3 rounded-md border px-3 py-2.5 sm:col-span-2'>
              <Checkbox
                checked={form.updateExisting}
                onCheckedChange={(checked) =>
                  updateForm({ updateExisting: checked === true })
                }
              />
              <span className='text-sm font-medium'>
                {t('Update Existing')}
              </span>
            </label>

            <div className='space-y-1.5'>
              <Label htmlFor='mcp-openapi-price'>{t('Price')}</Label>
              <Input
                id='mcp-openapi-price'
                type='number'
                min={0}
                step='0.0001'
                value={form.pricePerCall}
                onChange={(event) =>
                  updateForm({ pricePerCall: event.target.value })
                }
              />
            </div>

            <div className='space-y-1.5'>
              <Label htmlFor='mcp-openapi-price-unit'>{t('Price Unit')}</Label>
              <NativeSelect
                id='mcp-openapi-price-unit'
                value={form.priceUnit}
                onChange={(event) =>
                  updateForm({ priceUnit: event.target.value })
                }
                className='w-full'
              >
                {getPriceUnitOptions(t).map((option) => (
                  <NativeSelectOption key={option.value} value={option.value}>
                    {option.label}
                  </NativeSelectOption>
                ))}
              </NativeSelect>
            </div>

            <div className='space-y-1.5'>
              <Label htmlFor='mcp-openapi-free-quota'>
                {t('Daily Free Quota')}
              </Label>
              <Input
                id='mcp-openapi-free-quota'
                type='number'
                min={0}
                value={form.freeQuota}
                onChange={(event) =>
                  updateForm({ freeQuota: event.target.value })
                }
              />
            </div>

            <div className='space-y-1.5'>
              <Label htmlFor='mcp-openapi-status'>{t('Status')}</Label>
              <NativeSelect
                id='mcp-openapi-status'
                value={form.status}
                onChange={(event) => updateForm({ status: event.target.value })}
                className='w-full'
              >
                {getToolStatusOptions(t).map((option) => (
                  <NativeSelectOption key={option.value} value={option.value}>
                    {option.label}
                  </NativeSelectOption>
                ))}
              </NativeSelect>
            </div>

            <div className='space-y-1.5 sm:col-span-2'>
              <Label htmlFor='mcp-openapi-document'>
                {t('OpenAPI Document')}
              </Label>
              <Textarea
                id='mcp-openapi-document'
                value={form.document}
                onChange={(event) =>
                  updateForm({ document: event.target.value })
                }
                className='min-h-28 font-mono text-xs'
              />
            </div>
          </div>

          {preview && (
            <div className='mt-4 space-y-3'>
              <div className='flex flex-wrap items-center justify-between gap-2'>
                <div className='min-w-0'>
                  <div className='font-medium'>{preview.title || '-'}</div>
                  <div className='text-muted-foreground truncate text-xs'>
                    {preview.server_url || '-'}
                  </div>
                </div>
                <div className='flex items-center gap-2'>
                  <Button
                    type='button'
                    variant='outline'
                    size='sm'
                    onClick={() =>
                      setSelectedOperations(
                        preview.operations.map((item) => item.key)
                      )
                    }
                  >
                    {t('Select All')}
                  </Button>
                  <Button
                    type='button'
                    variant='outline'
                    size='sm'
                    onClick={() => setSelectedOperations([])}
                  >
                    {t('Clear')}
                  </Button>
                </div>
              </div>

              <OpenAPIPreviewMetrics
                preview={preview}
                selectedCount={selectedOperations.length}
              />

              <div className='grid gap-2'>
                {preview.operations.map((operation) => (
                  <OperationRow
                    key={operation.key}
                    operation={operation}
                    checked={selectedSet.has(operation.key)}
                    onCheckedChange={(checked) =>
                      toggleOperation(operation.key, checked)
                    }
                  />
                ))}
              </div>
            </div>
          )}

          {diffResult && (
            <ImportResultSummary
              result={diffResult}
              title={t('Planned Changes')}
            />
          )}
          {importResult && (
            <ImportResultSummary
              result={importResult}
              title={t('Import Result')}
            />
          )}
        </ScrollArea>

        <DialogFooter>
          <Button
            type='button'
            variant='outline'
            onClick={() => props.onOpenChange(false)}
            disabled={isBusy}
          >
            {t('Cancel')}
          </Button>
          <Button
            type='button'
            variant='outline'
            onClick={() => previewMutation.mutate()}
            disabled={
              isBusy || (!form.openapiUrl.trim() && !form.document.trim())
            }
            className={cn(previewMutation.isPending && 'opacity-80')}
          >
            {previewMutation.isPending && <Loader2 className='animate-spin' />}
            {t('Preview')}
          </Button>
          <Button
            type='button'
            variant='outline'
            onClick={() => diffMutation.mutate()}
            disabled={isBusy || !preview || selectedOperations.length === 0}
            className={cn(diffMutation.isPending && 'opacity-80')}
          >
            {diffMutation.isPending && <Loader2 className='animate-spin' />}
            {t('Review Diff')}
          </Button>
          <Button
            type='button'
            onClick={() => importMutation.mutate()}
            disabled={
              isBusy ||
              !preview ||
              selectedOperations.length === 0 ||
              !diffResult
            }
          >
            {importMutation.isPending && <Loader2 className='animate-spin' />}
            {t('Import')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
