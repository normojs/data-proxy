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
import { mcpQueryKeys, updateMCPProxyTool } from '../api'
import { getPriceUnitOptions, getProxyToolStatusOptions } from '../constants'
import type { MCPProxyTool, MCPProxyToolUpdatePayload } from '../types'

type ProxyToolEditDialogProps = {
  open: boolean
  tool: MCPProxyTool | null
  onOpenChange: (open: boolean) => void
}

type ProxyToolForm = {
  exposedToolName: string
  exposedDescription: string
  status: string
  pricePerCall: string
  priceUnit: string
  freeQuota: string
  sortOrder: string
}

function buildInitialForm(tool: MCPProxyTool | null): ProxyToolForm {
  return {
    exposedToolName: tool?.exposed_tool_name ?? '',
    exposedDescription: tool?.exposed_description ?? '',
    status: tool?.status ?? 'pending',
    pricePerCall: String(tool?.price_per_call ?? 0),
    priceUnit: tool?.price_unit ?? 'per_call',
    freeQuota: String(tool?.free_quota ?? 0),
    sortOrder: String(tool?.sort_order ?? 0),
  }
}

function parseNumber(value: string, fallback: number): number {
  const parsed = Number(value)
  return Number.isFinite(parsed) ? parsed : fallback
}

function buildPayload(form: ProxyToolForm): MCPProxyToolUpdatePayload {
  return {
    exposed_tool_name: form.exposedToolName.trim(),
    exposed_description: form.exposedDescription.trim(),
    status: form.status,
    price_per_call: Math.max(0, parseNumber(form.pricePerCall, 0)),
    price_unit: form.priceUnit,
    free_quota: Math.max(0, Math.trunc(parseNumber(form.freeQuota, 0))),
    sort_order: Math.trunc(parseNumber(form.sortOrder, 0)),
  }
}

export function ProxyToolEditDialog(props: ProxyToolEditDialogProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [form, setForm] = useState<ProxyToolForm>(() =>
    buildInitialForm(props.tool)
  )

  const mutation = useMutation({
    mutationFn: async () => {
      if (!props.tool) {
        throw new Error(t('No proxy tool selected'))
      }
      return updateMCPProxyTool(props.tool.id, buildPayload(form))
    },
    onSuccess: (res) => {
      if (!res.success) {
        toast.error(res.message || t('Failed to update MCP proxy tool'))
        return
      }
      toast.success(t('MCP proxy tool updated successfully'))
      queryClient.invalidateQueries({ queryKey: mcpQueryKeys.proxyTools() })
      queryClient.invalidateQueries({ queryKey: mcpQueryKeys.tools() })
      props.onOpenChange(false)
    },
  })

  return (
    <Dialog open={props.open} onOpenChange={props.onOpenChange}>
      <DialogContent className='max-w-[calc(100%-2rem)] sm:max-w-xl'>
        <DialogHeader>
          <DialogTitle>{t('Edit Proxy Tool')}</DialogTitle>
        </DialogHeader>

        <div className='grid gap-3 sm:grid-cols-2'>
          <div className='space-y-1.5 sm:col-span-2'>
            <Label htmlFor='mcp-proxy-tool-exposed-name'>
              {t('Exposed Tool Name')}
            </Label>
            <Input
              id='mcp-proxy-tool-exposed-name'
              value={form.exposedToolName}
              onChange={(event) =>
                setForm((current) => ({
                  ...current,
                  exposedToolName: event.target.value,
                }))
              }
            />
          </div>

          <div className='space-y-1.5'>
            <Label htmlFor='mcp-proxy-tool-status'>{t('Status')}</Label>
            <NativeSelect
              id='mcp-proxy-tool-status'
              value={form.status}
              onChange={(event) =>
                setForm((current) => ({
                  ...current,
                  status: event.target.value,
                }))
              }
              className='w-full'
            >
              {getProxyToolStatusOptions(t).map((option) => (
                <NativeSelectOption key={option.value} value={option.value}>
                  {option.label}
                </NativeSelectOption>
              ))}
            </NativeSelect>
          </div>

          <div className='space-y-1.5'>
            <Label htmlFor='mcp-proxy-tool-price-unit'>{t('Price Unit')}</Label>
            <NativeSelect
              id='mcp-proxy-tool-price-unit'
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
            <Label htmlFor='mcp-proxy-tool-price'>{t('Price')}</Label>
            <Input
              id='mcp-proxy-tool-price'
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
            <Label htmlFor='mcp-proxy-tool-free-quota'>
              {t('Daily Free Quota')}
            </Label>
            <Input
              id='mcp-proxy-tool-free-quota'
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

          <div className='space-y-1.5 sm:col-span-2'>
            <Label htmlFor='mcp-proxy-tool-sort-order'>{t('Sort Order')}</Label>
            <Input
              id='mcp-proxy-tool-sort-order'
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
            <Label htmlFor='mcp-proxy-tool-description'>
              {t('Description')}
            </Label>
            <Textarea
              id='mcp-proxy-tool-description'
              value={form.exposedDescription}
              onChange={(event) =>
                setForm((current) => ({
                  ...current,
                  exposedDescription: event.target.value,
                }))
              }
              className='min-h-24'
            />
          </div>
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
            disabled={mutation.isPending || !props.tool}
          >
            {mutation.isPending && <Loader2 className='animate-spin' />}
            {t('Save')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
