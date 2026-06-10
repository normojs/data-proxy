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
import {
  createMCPProxyServer,
  mcpQueryKeys,
  updateMCPProxyServer,
} from '../api'
import {
  getProxyAuthTypeOptions,
  getProxyServerStatusOptions,
  getProxyTransportOptions,
  getProxyVisibilityOptions,
} from '../constants'
import type { MCPProxyServer, MCPProxyServerPayload } from '../types'

type ProxyServerEditDialogProps = {
  open: boolean
  server: MCPProxyServer | null
  onOpenChange: (open: boolean) => void
}

type ProxyServerForm = {
  name: string
  namespace: string
  transport: string
  endpoint: string
  authType: string
  authRef: string
  timeoutMS: string
  maxResultSize: string
  maxMetadataSize: string
  visibility: string
  allowedGroups: string
  status: string
}

function buildInitialForm(server: MCPProxyServer | null): ProxyServerForm {
  return {
    name: server?.name ?? '',
    namespace: server?.namespace ?? '',
    transport: server?.transport ?? 'http',
    endpoint: server?.endpoint ?? '',
    authType: server?.auth_type ?? 'none',
    authRef: '',
    timeoutMS: String(server?.timeout_ms ?? 30000),
    maxResultSize: String(server?.max_result_size ?? 1048576),
    maxMetadataSize: String(server?.max_metadata_size ?? 65536),
    visibility: server?.visibility ?? 'admin',
    allowedGroups: (server?.allowed_groups ?? []).join(','),
    status: server?.status ?? 'disabled',
  }
}

function parseInteger(value: string, fallback: number): number {
  const parsed = Number(value)
  return Number.isFinite(parsed) ? Math.trunc(parsed) : fallback
}

function parseAllowedGroups(value: string): string[] {
  return value
    .split(',')
    .map((item) => item.trim())
    .filter(Boolean)
}

function isBridgeTransport(transport: string): boolean {
  return transport === 'bridge' || transport === 'qidian_browser'
}

function buildPayload(
  form: ProxyServerForm,
  server: MCPProxyServer | null
): MCPProxyServerPayload {
  const bridgeTransport = isBridgeTransport(form.transport)
  const payload: MCPProxyServerPayload = {
    name: form.name.trim(),
    namespace: form.namespace.trim(),
    transport: form.transport,
    endpoint: form.endpoint.trim(),
    auth_type: bridgeTransport ? 'none' : form.authType,
    timeout_ms: Math.max(1000, parseInteger(form.timeoutMS, 30000)),
    max_result_size: Math.max(1024, parseInteger(form.maxResultSize, 1048576)),
    max_metadata_size: Math.max(
      1024,
      parseInteger(form.maxMetadataSize, 65536)
    ),
    visibility: form.visibility,
    allowed_groups: parseAllowedGroups(form.allowedGroups),
    status: form.status,
  }
  if (bridgeTransport) {
    payload.auth_ref = ''
  } else {
    const authRef = form.authRef.trim()
    if (authRef || !server) {
      payload.auth_ref = authRef
    }
  }
  return payload
}

export function ProxyServerEditDialog(props: ProxyServerEditDialogProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [form, setForm] = useState<ProxyServerForm>(() =>
    buildInitialForm(props.server)
  )
  const isEdit = props.server != null
  const bridgeTransport = isBridgeTransport(form.transport)

  const mutation = useMutation({
    mutationFn: async () => {
      const payload = buildPayload(form, props.server)
      if (props.server) {
        return updateMCPProxyServer(props.server.id, payload)
      }
      return createMCPProxyServer(payload)
    },
    onSuccess: (res) => {
      if (!res.success) {
        toast.error(res.message || t('Failed to save MCP proxy server'))
        return
      }
      toast.success(t('MCP proxy server saved successfully'))
      queryClient.invalidateQueries({ queryKey: mcpQueryKeys.proxyServers() })
      props.onOpenChange(false)
    },
  })

  return (
    <Dialog open={props.open} onOpenChange={props.onOpenChange}>
      <DialogContent className='max-w-[calc(100%-2rem)] sm:max-w-2xl'>
        <DialogHeader>
          <DialogTitle>
            {t(isEdit ? 'Edit Proxy Server' : 'New Proxy Server')}
          </DialogTitle>
        </DialogHeader>

        <div className='grid gap-3 sm:grid-cols-2'>
          <div className='space-y-1.5'>
            <Label htmlFor='mcp-proxy-server-name'>{t('Name')}</Label>
            <Input
              id='mcp-proxy-server-name'
              value={form.name}
              onChange={(event) =>
                setForm((current) => ({ ...current, name: event.target.value }))
              }
            />
          </div>

          <div className='space-y-1.5'>
            <Label htmlFor='mcp-proxy-server-namespace'>{t('Namespace')}</Label>
            <Input
              id='mcp-proxy-server-namespace'
              value={form.namespace}
              disabled={isEdit}
              onChange={(event) =>
                setForm((current) => ({
                  ...current,
                  namespace: event.target.value,
                }))
              }
            />
          </div>

          <div className='space-y-1.5'>
            <Label htmlFor='mcp-proxy-server-transport'>{t('Transport')}</Label>
            <NativeSelect
              id='mcp-proxy-server-transport'
              value={form.transport}
              onChange={(event) =>
                setForm((current) => ({
                  ...current,
                  transport: event.target.value,
                  authType: isBridgeTransport(event.target.value)
                    ? 'none'
                    : current.authType,
                  authRef: isBridgeTransport(event.target.value)
                    ? ''
                    : current.authRef,
                }))
              }
              className='w-full'
            >
              {getProxyTransportOptions(t).map((option) => (
                <NativeSelectOption key={option.value} value={option.value}>
                  {option.label}
                </NativeSelectOption>
              ))}
            </NativeSelect>
          </div>

          <div className='space-y-1.5'>
            <Label htmlFor='mcp-proxy-server-status'>{t('Status')}</Label>
            <NativeSelect
              id='mcp-proxy-server-status'
              value={form.status}
              onChange={(event) =>
                setForm((current) => ({
                  ...current,
                  status: event.target.value,
                }))
              }
              className='w-full'
            >
              {getProxyServerStatusOptions(t).map((option) => (
                <NativeSelectOption key={option.value} value={option.value}>
                  {option.label}
                </NativeSelectOption>
              ))}
            </NativeSelect>
          </div>

          <div className='space-y-1.5 sm:col-span-2'>
            <Label htmlFor='mcp-proxy-server-endpoint'>
              {t(bridgeTransport ? 'Bridge Client Endpoint' : 'Endpoint')}
            </Label>
            <Input
              id='mcp-proxy-server-endpoint'
              value={form.endpoint}
              placeholder={
                bridgeTransport
                  ? 'bridge://client-id?target=http://127.0.0.1:8765/mcp'
                  : 'https://mcp.example.com/mcp'
              }
              onChange={(event) =>
                setForm((current) => ({
                  ...current,
                  endpoint: event.target.value,
                }))
              }
            />
          </div>

          {!bridgeTransport && (
            <>
              <div className='space-y-1.5'>
                <Label htmlFor='mcp-proxy-server-auth-type'>
                  {t('Auth Type')}
                </Label>
                <NativeSelect
                  id='mcp-proxy-server-auth-type'
                  value={form.authType}
                  onChange={(event) =>
                    setForm((current) => ({
                      ...current,
                      authType: event.target.value,
                    }))
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

              <div className='space-y-1.5'>
                <Label htmlFor='mcp-proxy-server-auth-ref'>
                  {t('Auth Ref')}
                </Label>
                <Input
                  id='mcp-proxy-server-auth-ref'
                  value={form.authRef}
                  placeholder={
                    isEdit
                      ? t('Leave blank to keep current secret')
                      : 'env:MCP_PROXY_TOKEN'
                  }
                  onChange={(event) =>
                    setForm((current) => ({
                      ...current,
                      authRef: event.target.value,
                    }))
                  }
                />
              </div>
            </>
          )}

          <div className='space-y-1.5'>
            <Label htmlFor='mcp-proxy-server-visibility'>
              {t('Visibility')}
            </Label>
            <NativeSelect
              id='mcp-proxy-server-visibility'
              value={form.visibility}
              onChange={(event) =>
                setForm((current) => ({
                  ...current,
                  visibility: event.target.value,
                }))
              }
              className='w-full'
            >
              {getProxyVisibilityOptions(t).map((option) => (
                <NativeSelectOption key={option.value} value={option.value}>
                  {option.label}
                </NativeSelectOption>
              ))}
            </NativeSelect>
          </div>

          <div className='space-y-1.5'>
            <Label htmlFor='mcp-proxy-server-allowed-groups'>
              {t('Allowed Groups')}
            </Label>
            <Input
              id='mcp-proxy-server-allowed-groups'
              value={form.allowedGroups}
              onChange={(event) =>
                setForm((current) => ({
                  ...current,
                  allowedGroups: event.target.value,
                }))
              }
            />
          </div>

          <div className='space-y-1.5'>
            <Label htmlFor='mcp-proxy-server-timeout'>{t('Timeout MS')}</Label>
            <Input
              id='mcp-proxy-server-timeout'
              type='number'
              min={1000}
              value={form.timeoutMS}
              onChange={(event) =>
                setForm((current) => ({
                  ...current,
                  timeoutMS: event.target.value,
                }))
              }
            />
          </div>

          <div className='space-y-1.5'>
            <Label htmlFor='mcp-proxy-server-result-size'>
              {t('Max Result Size')}
            </Label>
            <Input
              id='mcp-proxy-server-result-size'
              type='number'
              min={1024}
              value={form.maxResultSize}
              onChange={(event) =>
                setForm((current) => ({
                  ...current,
                  maxResultSize: event.target.value,
                }))
              }
            />
          </div>

          <div className='space-y-1.5 sm:col-span-2'>
            <Label htmlFor='mcp-proxy-server-metadata-size'>
              {t('Max Metadata Size')}
            </Label>
            <Input
              id='mcp-proxy-server-metadata-size'
              type='number'
              min={1024}
              value={form.maxMetadataSize}
              onChange={(event) =>
                setForm((current) => ({
                  ...current,
                  maxMetadataSize: event.target.value,
                }))
              }
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
            disabled={mutation.isPending}
          >
            {mutation.isPending && <Loader2 className='animate-spin' />}
            {t('Save')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
