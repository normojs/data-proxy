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
import { useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { Loader2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
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
import { Textarea } from '@/components/ui/textarea'
import { createMCPTool, mcpQueryKeys, updateMCPTool } from '../api'
import {
  MCP_TOOL_STATUS,
  getPriceUnitOptions,
  getToolStatusOptions,
} from '../constants'
import type {
  MCPTool,
  MCPToolCreatePayload,
  MCPToolUpdatePayload,
} from '../types'

type ToolEditDialogProps = {
  open: boolean
  tool: MCPTool | null
  onOpenChange: (open: boolean) => void
}

type ToolEditForm = {
  category: string
  description: string
  displayName: string
  freeQuota: string
  inputSchema: string
  name: string
  pricePerCall: string
  priceUnit: string
  sortOrder: string
  status: string
}

const DEFAULT_INPUT_SCHEMA = {
  type: 'object',
  properties: {},
}

function buildInitialForm(tool: MCPTool | null): ToolEditForm {
  const status =
    tool == null || tool.source === 'custom'
      ? MCP_TOOL_STATUS.DISABLED
      : tool.status

  return {
    category: tool?.category ?? '',
    description: tool?.description ?? '',
    displayName: tool?.display_name ?? '',
    freeQuota: String(tool?.free_quota ?? 0),
    inputSchema: JSON.stringify(
      tool?.input_schema ?? DEFAULT_INPUT_SCHEMA,
      null,
      2
    ),
    name: tool?.name ?? '',
    pricePerCall: String(tool?.price_per_call ?? 0),
    priceUnit: tool?.price_unit ?? 'per_call',
    sortOrder: String(tool?.sort_order ?? 0),
    status: String(status),
  }
}

function parseNumber(value: string, fallback: number): number {
  const parsed = Number(value)
  return Number.isFinite(parsed) ? parsed : fallback
}

function buildPayload(
  form: ToolEditForm,
  options?: { forceDisabledStatus?: boolean }
): MCPToolUpdatePayload {
  return {
    category: form.category.trim(),
    description: form.description.trim(),
    display_name: form.displayName.trim(),
    free_quota: Math.max(0, Math.trunc(parseNumber(form.freeQuota, 0))),
    price_per_call: Math.max(0, parseNumber(form.pricePerCall, 0)),
    price_unit: form.priceUnit,
    sort_order: Math.trunc(parseNumber(form.sortOrder, 0)),
    status: options?.forceDisabledStatus
      ? MCP_TOOL_STATUS.DISABLED
      : Math.trunc(parseNumber(form.status, MCP_TOOL_STATUS.ENABLED)),
  }
}

function parseInputSchema(value: string): unknown {
  const trimmed = value.trim()
  if (!trimmed) {
    return DEFAULT_INPUT_SCHEMA
  }
  const parsed = JSON.parse(trimmed)
  if (parsed == null || typeof parsed !== 'object' || Array.isArray(parsed)) {
    throw new Error('Input schema must be a JSON object')
  }
  return parsed
}

function buildCreatePayload(form: ToolEditForm): MCPToolCreatePayload {
  const payload = buildPayload(form, { forceDisabledStatus: true })
  return {
    category: payload.category ?? '',
    description: payload.description ?? '',
    display_name: payload.display_name ?? '',
    free_quota: payload.free_quota ?? 0,
    input_schema: parseInputSchema(form.inputSchema),
    name: form.name.trim(),
    price_per_call: payload.price_per_call ?? 0,
    price_unit: payload.price_unit ?? 'per_call',
    sort_order: payload.sort_order ?? 0,
    status: MCP_TOOL_STATUS.DISABLED,
  }
}

export function ToolEditDialog(props: ToolEditDialogProps) {
  const dialogKey = props.open ? String(props.tool?.id ?? 'new') : 'closed'

  return (
    <Dialog open={props.open} onOpenChange={props.onOpenChange}>
      <ToolEditDialogContent
        key={dialogKey}
        tool={props.tool}
        onOpenChange={props.onOpenChange}
      />
    </Dialog>
  )
}

function ToolEditDialogContent(props: Omit<ToolEditDialogProps, 'open'>) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [form, setForm] = useState<ToolEditForm>(() =>
    buildInitialForm(props.tool)
  )
  const isCreate = props.tool == null
  const isCustomTool = isCreate || props.tool?.source === 'custom'

  const mutation = useMutation({
    mutationFn: async () => {
      const tool = props.tool
      if (!tool) {
        return createMCPTool(buildCreatePayload(form))
      }
      return updateMCPTool(
        tool.id,
        buildPayload(form, { forceDisabledStatus: tool.source === 'custom' })
      )
    },
    onSuccess: (res) => {
      if (!res.success) {
        toast.error(
          res.message ||
            t(
              isCreate
                ? 'Failed to create MCP tool'
                : 'Failed to update MCP tool'
            )
        )
        return
      }
      toast.success(
        t(
          isCreate
            ? 'MCP tool created successfully'
            : 'MCP tool updated successfully'
        )
      )
      queryClient.invalidateQueries({ queryKey: mcpQueryKeys.tools() })
      props.onOpenChange(false)
    },
    onError: (error) => {
      toast.error(
        error instanceof Error ? t(error.message) : t('Operation failed')
      )
    },
  })

  return (
    <DialogContent className='max-w-[calc(100%-2rem)] sm:max-w-xl'>
      <DialogHeader>
        <DialogTitle>
          {t(isCreate ? 'Create MCP Tool' : 'Edit MCP Tool')}
        </DialogTitle>
      </DialogHeader>

      <div className='grid gap-3 sm:grid-cols-2'>
        <div className='space-y-1.5 sm:col-span-2'>
          <Label htmlFor='mcp-tool-name'>{t('Tool Name')}</Label>
          <Input
            id='mcp-tool-name'
            value={form.name}
            disabled={!isCreate}
            onChange={(event) =>
              setForm((current) => ({
                ...current,
                name: event.target.value,
              }))
            }
            className='font-mono'
          />
        </div>

        <div className='space-y-1.5 sm:col-span-2'>
          <Label htmlFor='mcp-tool-display-name'>{t('Display Name')}</Label>
          <Input
            id='mcp-tool-display-name'
            value={form.displayName}
            onChange={(event) =>
              setForm((current) => ({
                ...current,
                displayName: event.target.value,
              }))
            }
          />
        </div>

        <div className='space-y-1.5'>
          <Label htmlFor='mcp-tool-category'>{t('Category')}</Label>
          <Input
            id='mcp-tool-category'
            value={form.category}
            onChange={(event) =>
              setForm((current) => ({
                ...current,
                category: event.target.value,
              }))
            }
          />
        </div>

        <div className='space-y-1.5'>
          <Label htmlFor='mcp-tool-status'>{t('Status')}</Label>
          <NativeSelect
            id='mcp-tool-status'
            value={form.status}
            disabled={isCustomTool}
            onChange={(event) =>
              setForm((current) => ({
                ...current,
                status: event.target.value,
              }))
            }
            className='w-full'
          >
            {getToolStatusOptions(t).map((option) => (
              <NativeSelectOption key={option.value} value={option.value}>
                {option.label}
              </NativeSelectOption>
            ))}
          </NativeSelect>
        </div>

        <div className='space-y-1.5'>
          <Label htmlFor='mcp-tool-price'>{t('Price')}</Label>
          <Input
            id='mcp-tool-price'
            type='number'
            min={0}
            step='0.0001'
            value={form.pricePerCall}
            onChange={(event) =>
              setForm((current) => ({
                ...current,
                pricePerCall: event.target.value,
              }))
            }
          />
        </div>

        <div className='space-y-1.5'>
          <Label htmlFor='mcp-tool-price-unit'>{t('Price Unit')}</Label>
          <NativeSelect
            id='mcp-tool-price-unit'
            value={form.priceUnit}
            onChange={(event) =>
              setForm((current) => ({
                ...current,
                priceUnit: event.target.value,
              }))
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
          <Label htmlFor='mcp-tool-free-quota'>{t('Daily Free Quota')}</Label>
          <Input
            id='mcp-tool-free-quota'
            type='number'
            min={0}
            value={form.freeQuota}
            onChange={(event) =>
              setForm((current) => ({
                ...current,
                freeQuota: event.target.value,
              }))
            }
          />
        </div>

        <div className='space-y-1.5'>
          <Label htmlFor='mcp-tool-sort-order'>{t('Sort Order')}</Label>
          <Input
            id='mcp-tool-sort-order'
            type='number'
            value={form.sortOrder}
            onChange={(event) =>
              setForm((current) => ({
                ...current,
                sortOrder: event.target.value,
              }))
            }
          />
        </div>

        <div className='space-y-1.5 sm:col-span-2'>
          <Label htmlFor='mcp-tool-description'>{t('Description')}</Label>
          <Textarea
            id='mcp-tool-description'
            value={form.description}
            onChange={(event) =>
              setForm((current) => ({
                ...current,
                description: event.target.value,
              }))
            }
            className='min-h-24'
          />
        </div>

        {isCreate && (
          <div className='space-y-1.5 sm:col-span-2'>
            <Label htmlFor='mcp-tool-input-schema'>{t('Input Schema')}</Label>
            <Textarea
              id='mcp-tool-input-schema'
              value={form.inputSchema}
              onChange={(event) =>
                setForm((current) => ({
                  ...current,
                  inputSchema: event.target.value,
                }))
              }
              className='min-h-32 font-mono text-xs'
            />
          </div>
        )}
      </div>

      <DialogFooter>
        <Button
          type='button'
          variant='outline'
          onClick={() => props.onOpenChange(false)}
        >
          {t('Cancel')}
        </Button>
        <Button
          type='button'
          onClick={() => mutation.mutate()}
          disabled={mutation.isPending || (isCreate && form.name.trim() === '')}
        >
          {mutation.isPending && <Loader2 className='animate-spin' />}
          {t(isCreate ? 'Create' : 'Save')}
        </Button>
      </DialogFooter>
    </DialogContent>
  )
}
