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
import { useEffect, useMemo, useState } from 'react'
import { useMutation, useQuery } from '@tanstack/react-query'
import { getRouteApi } from '@tanstack/react-router'
import {
  type ColumnDef,
  type VisibilityState,
  getCoreRowModel,
  useReactTable,
} from '@tanstack/react-table'
import { useMediaQuery } from '@/hooks'
import {
  FileText,
  History,
  KeyRound,
  Loader2,
  Pencil,
  Plus,
  RefreshCw,
  ShieldAlert,
  Trash2,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { cn } from '@/lib/utils'
import { useTableUrlState } from '@/hooks/use-table-url-state'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogMedia,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'
import { Textarea } from '@/components/ui/textarea'
import { CopyButton } from '@/components/copy-button'
import { DataTableColumnHeader, DataTablePage } from '@/components/data-table'
import { LongText } from '@/components/long-text'
import { StatusBadge, type StatusBadgeProps } from '@/components/status-badge'
import {
  createTunnelConnection,
  ensureTunnelAgentSetup,
  listTunnelAuditLogs,
  listTunnelConnections,
  listUserTunnelApps,
  mcpQueryKeys,
  revokeTunnelConnection,
  updateTunnelConnection,
} from '../api'
import { mcpQueryError, mcpQueryErrorMessage } from '../lib/query-errors'
import type {
  TunnelApp,
  TunnelAuditLog,
  TunnelAgentSetupResponse,
  TunnelConnection,
  TunnelConnectionCreateResponse,
} from '../types'
import { JsonDetailDialog } from './json-detail-dialog'
import {
  DurationCell,
  IdCell,
  LongTextCell,
  SizeCell,
  TimestampCell,
  TraceCell,
} from './table-cells'

const route = getRouteApi('/_authenticated/mcp/$section')

type TunnelConnectionCreateForm = {
  name: string
  permissionMode: string
  expiryPreset: string
  maxRequestsPerMinute: string
  maxBytesInPerMinute: string
  maxBytesOutPerMinute: string
}

type TunnelConnectionEditForm = {
  name: string
  expiryPreset: string
  customExpiresAt: string
  maxRequestsPerMinute: string
  maxBytesInPerMinute: string
  maxBytesOutPerMinute: string
}

type DetailState = {
  description?: string
  title: string
  value: unknown
}

const permissionOrder = ['read_only', 'write', 'exec_safe', 'exec_trusted']

function tunnelLabel(value: string, t: (key: string) => string): string {
  const labels: Record<string, string> = {
    active: t('Active'),
    revoked: t('Revoked'),
    expired: t('Expired'),
    http_tunnel: t('HTTP Tunnel'),
    mcp_code: t('MCP Code Tunnel'),
    read_only: t('Read Only'),
    traffic: t('Traffic'),
    write: t('Write'),
    exec_safe: t('Exec Safe'),
    exec_trusted: t('Exec Trusted'),
  }
  return labels[value] || value
}

function connectionStatusVariant(
  status: string,
  expiresAt: number
): StatusBadgeProps['variant'] {
  if (isExpired(expiresAt)) return 'yellow'
  if (status === 'active') return 'green'
  if (status === 'revoked') return 'grey'
  return 'neutral'
}

function isExpired(expiresAt: number): boolean {
  return expiresAt > 0 && expiresAt <= Math.floor(Date.now() / 1000)
}

function formatExpiry(value: number, t: (key: string) => string): string {
  if (!value) return t('Never')
  return isExpired(value) ? t('Expired') : ''
}

function formatFullEndpoint(endpointPath: string): string {
  if (typeof window === 'undefined') return endpointPath
  return `${window.location.origin}${endpointPath}`
}

function formatJSON(value: unknown): string {
  return JSON.stringify(value, null, 2)
}

function getPermissionOptions(app?: TunnelApp | null) {
  if (app?.app_type === 'http_tunnel' || app?.permission_mode === 'traffic') {
    return ['traffic']
  }
  const maxIndex = Math.max(
    0,
    permissionOrder.indexOf(app?.permission_mode || '')
  )
  return permissionOrder.slice(0, maxIndex + 1)
}

function expiryPresetToTimestamp(value: string): number {
  if (value === 'never') return 0
  const days = Number(value)
  if (!Number.isFinite(days) || days <= 0) return 0
  return Math.floor(Date.now() / 1000) + Math.trunc(days) * 86400
}

function editExpiryToTimestamp(form: TunnelConnectionEditForm): number {
  if (form.expiryPreset === 'never') return 0
  if (form.expiryPreset === 'custom') {
    const parsed = Number(form.customExpiresAt)
    return Number.isFinite(parsed) && parsed > 0 ? Math.trunc(parsed) : 0
  }
  return expiryPresetToTimestamp(form.expiryPreset)
}

function buildInitialForm(app?: TunnelApp | null): TunnelConnectionCreateForm {
  return {
    name: app?.app_type === 'http_tunnel' ? 'HTTP Client' : 'Desktop MCP',
    permissionMode: getPermissionOptions(app)[0] ?? 'read_only',
    expiryPreset: 'never',
    maxRequestsPerMinute: '',
    maxBytesInPerMinute: '',
    maxBytesOutPerMinute: '',
  }
}

function stringFromConfigNumber(value: unknown): string {
  if (typeof value === 'number' && Number.isFinite(value) && value > 0) {
    return String(Math.trunc(value))
  }
  if (typeof value === 'string' && value.trim()) return value.trim()
  return ''
}

function connectionRateLimit(connection?: TunnelConnection | null) {
  const config = connection?.config
  const nested =
    config && typeof config.rate_limit === 'object' && config.rate_limit != null
      ? (config.rate_limit as Record<string, unknown>)
      : {}
  return nested
}

function buildEditForm(
  connection?: TunnelConnection | null
): TunnelConnectionEditForm {
  const rateLimit = connectionRateLimit(connection)
  return {
    name: connection?.name ?? '',
    expiryPreset: connection?.expires_at ? 'custom' : 'never',
    customExpiresAt: connection?.expires_at
      ? String(connection.expires_at)
      : '',
    maxRequestsPerMinute: stringFromConfigNumber(
      rateLimit.max_requests_per_minute
    ),
    maxBytesInPerMinute: stringFromConfigNumber(
      rateLimit.max_bytes_in_per_minute
    ),
    maxBytesOutPerMinute: stringFromConfigNumber(
      rateLimit.max_bytes_out_per_minute
    ),
  }
}

function normalizeFormForApp(
  form: TunnelConnectionCreateForm,
  app?: TunnelApp | null
): TunnelConnectionCreateForm {
  const options = getPermissionOptions(app)
  if (options.includes(form.permissionMode)) return form
  return { ...form, permissionMode: options[0] ?? 'read_only' }
}

function getDisplayStatus(connection: TunnelConnection): string {
  if (isExpired(connection.expires_at)) return 'expired'
  return connection.status
}

function optionalPositiveInteger(value: string): number | undefined {
  const trimmed = value.trim()
  if (!trimmed) return undefined
  const parsed = Number(trimmed)
  if (!Number.isFinite(parsed) || parsed <= 0) return undefined
  return Math.trunc(parsed)
}

function buildConnectionConfig(form: TunnelConnectionCreateForm) {
  const rateLimit: Record<string, number> = {}
  const maxRequestsPerMinute = optionalPositiveInteger(
    form.maxRequestsPerMinute
  )
  const maxBytesInPerMinute = optionalPositiveInteger(form.maxBytesInPerMinute)
  const maxBytesOutPerMinute = optionalPositiveInteger(
    form.maxBytesOutPerMinute
  )
  if (maxRequestsPerMinute) {
    rateLimit.max_requests_per_minute = maxRequestsPerMinute
  }
  if (maxBytesInPerMinute) {
    rateLimit.max_bytes_in_per_minute = maxBytesInPerMinute
  }
  if (maxBytesOutPerMinute) {
    rateLimit.max_bytes_out_per_minute = maxBytesOutPerMinute
  }
  return Object.keys(rateLimit).length > 0 ? { rate_limit: rateLimit } : {}
}

function tunnelDecisionVariant(decision: string): StatusBadgeProps['variant'] {
  if (decision === 'allow') return 'green'
  if (decision === 'deny') return 'red'
  if (decision === 'update') return 'yellow'
  return 'neutral'
}

function auditLabel(value: string, t: (key: string) => string): string {
  const labels: Record<string, string> = {
    agent_setup: t('Agent Setup'),
    allow: t('Allow'),
    create: t('Create'),
    deny: t('Deny'),
    disconnect: t('Disconnect'),
    mcp_tool_call: t('MCP Tool Call'),
    policy_deny: t('Policy Deny'),
    prompts_get: t('Prompts Get'),
    prompts_list: t('Prompts List'),
    proxy_request: t('Proxy Request'),
    rate_limit: t('Rate Limit'),
    resources_read: t('Resources Read'),
    resources_list: t('Resources List'),
    revoke: t('Revoke'),
    review: t('Review'),
    tools_list: t('Tools List'),
    update: t('Update'),
  }
  return labels[value] || value || '-'
}

function AppSelector(props: {
  apps: TunnelApp[]
  loading: boolean
  selectedAppId?: number
  onChange: (appId: number) => void
}) {
  const { t } = useTranslation()
  return (
    <div className='flex min-w-[220px] items-center gap-2'>
      <Label htmlFor='mcp-tunnel-app-select' className='shrink-0 text-xs'>
        {t('Tunnel App')}
      </Label>
      <NativeSelect
        id='mcp-tunnel-app-select'
        size='sm'
        value={props.selectedAppId ? String(props.selectedAppId) : ''}
        disabled={props.loading || props.apps.length === 0}
        onChange={(event) => props.onChange(Number(event.target.value))}
        className='w-[240px] max-w-full'
      >
        {props.apps.length === 0 ? (
          <NativeSelectOption value=''>
            {props.loading ? t('Loading...') : t('No Approved Tunnel Apps')}
          </NativeSelectOption>
        ) : (
          props.apps.map((app) => (
            <NativeSelectOption key={app.id} value={String(app.id)}>
              {app.name}
            </NativeSelectOption>
          ))
        )}
      </NativeSelect>
    </div>
  )
}

function SecretField(props: { label: string; value: string }) {
  return (
    <div className='space-y-1.5'>
      <Label>{props.label}</Label>
      <div className='flex min-w-0 items-center gap-2'>
        <Input
          value={props.value}
          readOnly
          className='font-mono text-xs'
          onFocus={(event) => event.target.select()}
        />
        <CopyButton
          value={props.value}
          variant='outline'
          size='icon'
          tooltip={props.label}
        />
      </div>
    </div>
  )
}

function TunnelAgentSetupDialog(props: {
  app: TunnelApp | null
  connection: TunnelConnection | null
  open: boolean
  onOpenChange: (open: boolean) => void
  onUpdated: () => void
}) {
  const { t } = useTranslation()
  const [setup, setSetup] = useState<TunnelAgentSetupResponse | null>(null)
  const [rotate, setRotate] = useState(false)

  useEffect(() => {
    if (props.open) return
    setSetup(null)
    setRotate(false)
  }, [props.open])

  const mutation = useMutation({
    mutationFn: async () => {
      if (!props.app || !props.connection) {
        throw new Error(t('Please select a tunnel connection'))
      }
      return ensureTunnelAgentSetup(props.app.id, {
        connection_id: props.connection.id,
        rotate,
        client_name: props.connection.name || props.app.name,
      })
    },
    onSuccess: (result) => {
      if (!result.success || !result.data) {
        toast.error(result.message || t('Failed to prepare agent setup'))
        return
      }
      setSetup(result.data)
      props.onUpdated()
      toast.success(
        result.data.rotated
          ? t('Tunnel agent key rotated')
          : t('Tunnel agent setup ready')
      )
    },
    onError: (error) => {
      toast.error(
        mcpQueryErrorMessage(error, t('Failed to prepare agent setup'))
      )
    },
  })

  const configSnippet = setup ? formatJSON(setup.config) : ''
  const registerSnippet = setup ? formatJSON(setup.register) : ''
  const envSnippet = setup
    ? Object.entries(setup.environment)
        .map(([key, value]) => `${key}=${JSON.stringify(value)}`)
        .join('\n')
    : ''

  return (
    <Dialog open={props.open} onOpenChange={props.onOpenChange}>
      <DialogContent className='max-w-[calc(100%-2rem)] sm:max-w-3xl'>
        <DialogHeader>
          <DialogTitle>{t('Tunnel Agent Setup')}</DialogTitle>
          <DialogDescription>
            {setup
              ? t(
                  'Copy the API key now if it is shown. Full keys are only returned after creation or rotation.'
                )
              : t(
                  'Generate the local agent configuration for this tunnel connection.'
                )}
          </DialogDescription>
        </DialogHeader>

        {setup ? (
          <div className='space-y-3'>
            <div className='grid gap-3 sm:grid-cols-2'>
              <SecretField
                label={t('Bridge WebSocket')}
                value={setup.bridge_ws_url}
              />
              <SecretField label={t('Tunnel MCP URL')} value={setup.mcp_url} />
              <SecretField
                label={t('Bridge Client ID')}
                value={setup.client_id}
              />
              <SecretField
                label={setup.api_key ? t('API Key') : t('Masked API Key')}
                value={setup.api_key || setup.token_masked_key}
              />
            </div>
            <div className='grid gap-3 lg:grid-cols-2'>
              <div className='space-y-1.5'>
                <Label>{t('Environment')}</Label>
                <div className='relative'>
                  <Textarea
                    value={envSnippet}
                    readOnly
                    className='min-h-32 resize-none font-mono text-xs'
                    onFocus={(event) => event.target.select()}
                  />
                  <CopyButton
                    value={envSnippet}
                    variant='ghost'
                    size='icon'
                    tooltip={t('Copy Environment')}
                    className='absolute top-1.5 right-1.5'
                  />
                </div>
              </div>
              <div className='space-y-1.5'>
                <Label>{t('Bridge Register Message')}</Label>
                <div className='relative'>
                  <Textarea
                    value={registerSnippet}
                    readOnly
                    className='min-h-32 resize-none font-mono text-xs'
                    onFocus={(event) => event.target.select()}
                  />
                  <CopyButton
                    value={registerSnippet}
                    variant='ghost'
                    size='icon'
                    tooltip={t('Copy Register Message')}
                    className='absolute top-1.5 right-1.5'
                  />
                </div>
              </div>
            </div>
            <div className='space-y-1.5'>
              <Label>{t('Agent Config JSON')}</Label>
              <div className='relative'>
                <Textarea
                  value={configSnippet}
                  readOnly
                  className='min-h-40 resize-none font-mono text-xs'
                  onFocus={(event) => event.target.select()}
                />
                <CopyButton
                  value={configSnippet}
                  variant='ghost'
                  size='icon'
                  tooltip={t('Copy Agent Config')}
                  className='absolute top-1.5 right-1.5'
                />
              </div>
            </div>
          </div>
        ) : (
          <div className='space-y-3'>
            <div className='grid gap-3 sm:grid-cols-2'>
              <div className='space-y-1.5'>
                <Label>{t('Tunnel App')}</Label>
                <Input value={props.app?.name ?? ''} readOnly />
              </div>
              <div className='space-y-1.5'>
                <Label>{t('Connection')}</Label>
                <Input value={props.connection?.name ?? ''} readOnly />
              </div>
              <div className='space-y-1.5'>
                <Label>{t('Bridge Client ID')}</Label>
                <Input
                  value={props.app?.bridge_client_id ?? ''}
                  readOnly
                  className='font-mono text-xs'
                />
              </div>
              <div className='space-y-1.5'>
                <Label>{t('Key Prefix')}</Label>
                <Input
                  value={props.connection?.key_prefix ?? ''}
                  readOnly
                  className='font-mono text-xs'
                />
              </div>
            </div>
            <label className='flex items-start gap-2 rounded-lg border p-3 text-sm'>
              <input
                type='checkbox'
                checked={rotate}
                onChange={(event) => setRotate(event.target.checked)}
                className='mt-0.5'
              />
              <span className='space-y-0.5'>
                <span className='block font-medium'>
                  {t('Rotate existing agent API key')}
                </span>
                <span className='text-muted-foreground block text-xs'>
                  {t(
                    'Rotation disables the previous agent key and returns a new full key once.'
                  )}
                </span>
              </span>
            </label>
          </div>
        )}

        <DialogFooter>
          {setup ? (
            <Button onClick={() => props.onOpenChange(false)}>
              {t('Done')}
            </Button>
          ) : (
            <>
              <Button
                variant='outline'
                onClick={() => props.onOpenChange(false)}
              >
                {t('Cancel')}
              </Button>
              <Button
                onClick={() => mutation.mutate()}
                disabled={!props.app || !props.connection || mutation.isPending}
              >
                {mutation.isPending ? (
                  <Loader2 className='size-4 animate-spin' />
                ) : (
                  <KeyRound className='size-4' />
                )}
                {t('Prepare Setup')}
              </Button>
            </>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

function CreateTunnelConnectionDialog(props: {
  app: TunnelApp | null
  open: boolean
  onOpenChange: (open: boolean) => void
  onCreated: () => void
}) {
  const { t } = useTranslation()
  const [form, setForm] = useState<TunnelConnectionCreateForm>(() =>
    buildInitialForm(props.app)
  )
  const [created, setCreated] = useState<TunnelConnectionCreateResponse | null>(
    null
  )

  useEffect(() => {
    if (!props.open) return
    setForm((current) => normalizeFormForApp(current, props.app))
  }, [props.app, props.open])

  useEffect(() => {
    if (props.open) return
    setCreated(null)
    setForm(buildInitialForm(props.app))
  }, [props.app, props.open])

  const mutation = useMutation({
    mutationFn: async () => {
      if (!props.app) throw new Error(t('Please select a tunnel app'))
      return createTunnelConnection(props.app.id, {
        name: form.name.trim(),
        permission_mode: form.permissionMode,
        expires_at: expiryPresetToTimestamp(form.expiryPreset),
        config: buildConnectionConfig(form),
      })
    },
    onSuccess: (result) => {
      if (!result.success || !result.data) {
        toast.error(result.message || t('Failed to create tunnel connection'))
        return
      }
      setCreated(result.data)
      props.onCreated()
      toast.success(t('Tunnel connection created'))
    },
    onError: (error) => {
      toast.error(
        mcpQueryErrorMessage(error, t('Failed to create tunnel connection'))
      )
    },
  })

  const permissionOptions = getPermissionOptions(props.app)
  const endpoint = created
    ? formatFullEndpoint(
        created.endpoint_path || created.connection.endpoint_path
      )
    : ''
  const configSnippet = created
    ? props.app?.app_type === 'http_tunnel'
      ? `curl ${endpoint}`
      : JSON.stringify(
          {
            mcpServers: {
              [props.app?.name || 'data-proxy-tunnel']: {
                url: endpoint,
              },
            },
          },
          null,
          2
        )
    : ''

  return (
    <Dialog open={props.open} onOpenChange={props.onOpenChange}>
      <DialogContent className='max-w-[calc(100%-2rem)] sm:max-w-xl'>
        <DialogHeader>
          <DialogTitle>
            {t(created ? 'Tunnel Connection Created' : 'New Tunnel Connection')}
          </DialogTitle>
          <DialogDescription>
            {t(
              created
                ? 'Copy the connection key or endpoint now. The full key is only shown once.'
                : 'Create a dedicated tunnel connection for the selected app.'
            )}
          </DialogDescription>
        </DialogHeader>

        {created ? (
          <div className='space-y-3'>
            <SecretField
              label={t('Connection Key')}
              value={created.connection_key}
            />
            <SecretField
              label={t(
                props.app?.app_type === 'http_tunnel'
                  ? 'HTTP Endpoint'
                  : 'MCP Endpoint'
              )}
              value={endpoint}
            />
            <div className='space-y-1.5'>
              <Label>
                {t(
                  props.app?.app_type === 'http_tunnel'
                    ? 'Curl Example'
                    : 'Client Config'
                )}
              </Label>
              <div className='relative'>
                <Textarea
                  value={configSnippet}
                  readOnly
                  className='min-h-28 resize-none font-mono text-xs'
                  onFocus={(event) => event.target.select()}
                />
                <CopyButton
                  value={configSnippet}
                  variant='ghost'
                  size='icon'
                  tooltip={t('Copy Client Config')}
                  className='absolute top-1.5 right-1.5'
                />
              </div>
            </div>
          </div>
        ) : (
          <div className='grid gap-3 sm:grid-cols-2'>
            <div className='space-y-1.5 sm:col-span-2'>
              <Label htmlFor='mcp-tunnel-connection-app'>
                {t('Tunnel App')}
              </Label>
              <Input
                id='mcp-tunnel-connection-app'
                value={props.app?.name ?? ''}
                readOnly
                disabled={!props.app}
              />
            </div>
            <div className='space-y-1.5'>
              <Label htmlFor='mcp-tunnel-connection-name'>{t('Name')}</Label>
              <Input
                id='mcp-tunnel-connection-name'
                value={form.name}
                onChange={(event) =>
                  setForm((current) => ({
                    ...current,
                    name: event.target.value,
                  }))
                }
              />
            </div>
            <div className='space-y-1.5'>
              <Label htmlFor='mcp-tunnel-connection-permission'>
                {t('Permission')}
              </Label>
              <NativeSelect
                id='mcp-tunnel-connection-permission'
                value={form.permissionMode}
                onChange={(event) =>
                  setForm((current) => ({
                    ...current,
                    permissionMode: event.target.value,
                  }))
                }
                className='w-full'
              >
                {permissionOptions.map((option) => (
                  <NativeSelectOption key={option} value={option}>
                    {tunnelLabel(option, t)}
                  </NativeSelectOption>
                ))}
              </NativeSelect>
            </div>
            <div className='space-y-1.5'>
              <Label htmlFor='mcp-tunnel-connection-expiry'>
                {t('Expires')}
              </Label>
              <NativeSelect
                id='mcp-tunnel-connection-expiry'
                value={form.expiryPreset}
                onChange={(event) =>
                  setForm((current) => ({
                    ...current,
                    expiryPreset: event.target.value,
                  }))
                }
                className='w-full'
              >
                <NativeSelectOption value='never'>
                  {t('Never')}
                </NativeSelectOption>
                <NativeSelectOption value='7'>{t('7 Days')}</NativeSelectOption>
                <NativeSelectOption value='30'>
                  {t('30 Days')}
                </NativeSelectOption>
                <NativeSelectOption value='90'>
                  {t('90 Days')}
                </NativeSelectOption>
              </NativeSelect>
            </div>
            <div className='space-y-1.5'>
              <Label htmlFor='mcp-tunnel-connection-rate-requests'>
                {t('Max Requests Per Minute')}
              </Label>
              <Input
                id='mcp-tunnel-connection-rate-requests'
                inputMode='numeric'
                value={form.maxRequestsPerMinute}
                placeholder={t('Unlimited')}
                onChange={(event) =>
                  setForm((current) => ({
                    ...current,
                    maxRequestsPerMinute: event.target.value,
                  }))
                }
              />
            </div>
            <div className='space-y-1.5'>
              <Label htmlFor='mcp-tunnel-connection-rate-in'>
                {t('Max Inbound Bytes Per Minute')}
              </Label>
              <Input
                id='mcp-tunnel-connection-rate-in'
                inputMode='numeric'
                value={form.maxBytesInPerMinute}
                placeholder={t('Unlimited')}
                onChange={(event) =>
                  setForm((current) => ({
                    ...current,
                    maxBytesInPerMinute: event.target.value,
                  }))
                }
              />
            </div>
            <div className='space-y-1.5'>
              <Label htmlFor='mcp-tunnel-connection-rate-out'>
                {t('Max Outbound Bytes Per Minute')}
              </Label>
              <Input
                id='mcp-tunnel-connection-rate-out'
                inputMode='numeric'
                value={form.maxBytesOutPerMinute}
                placeholder={t('Unlimited')}
                onChange={(event) =>
                  setForm((current) => ({
                    ...current,
                    maxBytesOutPerMinute: event.target.value,
                  }))
                }
              />
            </div>
          </div>
        )}

        <DialogFooter>
          {created ? (
            <Button onClick={() => props.onOpenChange(false)}>
              {t('Done')}
            </Button>
          ) : (
            <>
              <Button
                variant='outline'
                onClick={() => props.onOpenChange(false)}
              >
                {t('Cancel')}
              </Button>
              <Button
                onClick={() => mutation.mutate()}
                disabled={!props.app || mutation.isPending}
              >
                {mutation.isPending ? (
                  <Loader2 className='size-4 animate-spin' />
                ) : (
                  <Plus className='size-4' />
                )}
                {t('Create')}
              </Button>
            </>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

function EditTunnelConnectionDialog(props: {
  app: TunnelApp | null
  connection: TunnelConnection | null
  open: boolean
  onOpenChange: (open: boolean) => void
  onUpdated: () => void
}) {
  const { t } = useTranslation()
  const [form, setForm] = useState<TunnelConnectionEditForm>(() =>
    buildEditForm(props.connection)
  )

  useEffect(() => {
    if (!props.open) return
    setForm(buildEditForm(props.connection))
  }, [props.connection, props.open])

  const mutation = useMutation({
    mutationFn: async () => {
      if (!props.app || !props.connection) {
        throw new Error(t('Please select a tunnel connection'))
      }
      return updateTunnelConnection(props.app.id, props.connection.id, {
        name: form.name.trim(),
        expires_at: editExpiryToTimestamp(form),
        config: buildConnectionConfig({
          ...form,
          permissionMode: props.connection.permission_mode,
        }),
      })
    },
    onSuccess: (result) => {
      if (!result.success) {
        toast.error(result.message || t('Failed to update tunnel connection'))
        return
      }
      toast.success(t('Tunnel connection updated'))
      props.onUpdated()
      props.onOpenChange(false)
    },
    onError: (error) => {
      toast.error(
        mcpQueryErrorMessage(error, t('Failed to update tunnel connection'))
      )
    },
  })

  return (
    <Dialog open={props.open} onOpenChange={props.onOpenChange}>
      <DialogContent className='max-w-[calc(100%-2rem)] sm:max-w-xl'>
        <DialogHeader>
          <DialogTitle>{t('Edit Tunnel Connection')}</DialogTitle>
          <DialogDescription>
            {t('Update connection metadata, expiration, and rate limits.')}
          </DialogDescription>
        </DialogHeader>

        <div className='grid gap-3 sm:grid-cols-2'>
          <div className='space-y-1.5 sm:col-span-2'>
            <Label htmlFor='mcp-tunnel-edit-name'>{t('Name')}</Label>
            <Input
              id='mcp-tunnel-edit-name'
              value={form.name}
              onChange={(event) =>
                setForm((current) => ({
                  ...current,
                  name: event.target.value,
                }))
              }
            />
          </div>
          <div className='space-y-1.5'>
            <Label htmlFor='mcp-tunnel-edit-expiry'>{t('Expires')}</Label>
            <NativeSelect
              id='mcp-tunnel-edit-expiry'
              value={form.expiryPreset}
              onChange={(event) =>
                setForm((current) => ({
                  ...current,
                  expiryPreset: event.target.value,
                }))
              }
              className='w-full'
            >
              <NativeSelectOption value='never'>
                {t('Never')}
              </NativeSelectOption>
              <NativeSelectOption value='7'>{t('7 Days')}</NativeSelectOption>
              <NativeSelectOption value='30'>{t('30 Days')}</NativeSelectOption>
              <NativeSelectOption value='90'>{t('90 Days')}</NativeSelectOption>
              <NativeSelectOption value='custom'>
                {t('Custom Timestamp')}
              </NativeSelectOption>
            </NativeSelect>
          </div>
          <div className='space-y-1.5'>
            <Label htmlFor='mcp-tunnel-edit-expiry-custom'>
              {t('Expires At Timestamp')}
            </Label>
            <Input
              id='mcp-tunnel-edit-expiry-custom'
              inputMode='numeric'
              value={form.customExpiresAt}
              disabled={form.expiryPreset !== 'custom'}
              placeholder='0'
              onChange={(event) =>
                setForm((current) => ({
                  ...current,
                  customExpiresAt: event.target.value,
                }))
              }
            />
          </div>
          <div className='space-y-1.5'>
            <Label htmlFor='mcp-tunnel-edit-rate-requests'>
              {t('Max Requests Per Minute')}
            </Label>
            <Input
              id='mcp-tunnel-edit-rate-requests'
              inputMode='numeric'
              value={form.maxRequestsPerMinute}
              placeholder={t('Unlimited')}
              onChange={(event) =>
                setForm((current) => ({
                  ...current,
                  maxRequestsPerMinute: event.target.value,
                }))
              }
            />
          </div>
          <div className='space-y-1.5'>
            <Label htmlFor='mcp-tunnel-edit-rate-in'>
              {t('Max Inbound Bytes Per Minute')}
            </Label>
            <Input
              id='mcp-tunnel-edit-rate-in'
              inputMode='numeric'
              value={form.maxBytesInPerMinute}
              placeholder={t('Unlimited')}
              onChange={(event) =>
                setForm((current) => ({
                  ...current,
                  maxBytesInPerMinute: event.target.value,
                }))
              }
            />
          </div>
          <div className='space-y-1.5'>
            <Label htmlFor='mcp-tunnel-edit-rate-out'>
              {t('Max Outbound Bytes Per Minute')}
            </Label>
            <Input
              id='mcp-tunnel-edit-rate-out'
              inputMode='numeric'
              value={form.maxBytesOutPerMinute}
              placeholder={t('Unlimited')}
              onChange={(event) =>
                setForm((current) => ({
                  ...current,
                  maxBytesOutPerMinute: event.target.value,
                }))
              }
            />
          </div>
        </div>

        <DialogFooter>
          <Button variant='outline' onClick={() => props.onOpenChange(false)}>
            {t('Cancel')}
          </Button>
          <Button
            onClick={() => mutation.mutate()}
            disabled={!props.app || !props.connection || mutation.isPending}
          >
            {mutation.isPending ? (
              <Loader2 className='size-4 animate-spin' />
            ) : (
              <Pencil className='size-4' />
            )}
            {t('Save')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

function RevokeTunnelConnectionDialog(props: {
  connection: TunnelConnection | null
  open: boolean
  onOpenChange: (open: boolean) => void
  onConfirm: (connection: TunnelConnection) => void
  pending: boolean
}) {
  const { t } = useTranslation()
  return (
    <AlertDialog open={props.open} onOpenChange={props.onOpenChange}>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogMedia>
            <ShieldAlert className='size-5' />
          </AlertDialogMedia>
          <AlertDialogTitle>{t('Revoke tunnel connection?')}</AlertDialogTitle>
          <AlertDialogDescription>
            {t(
              'This connection key will stop working immediately. Existing clients using it must be updated.'
            )}
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>{t('Cancel')}</AlertDialogCancel>
          <AlertDialogAction
            variant='destructive'
            disabled={!props.connection || props.pending}
            onClick={() =>
              props.connection && props.onConfirm(props.connection)
            }
          >
            {props.pending && <Loader2 className='size-4 animate-spin' />}
            {t('Revoke')}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  )
}

function TunnelConnectionAuditDialog(props: {
  app: TunnelApp | null
  connection: TunnelConnection | null
  open: boolean
  onOpenChange: (open: boolean) => void
}) {
  const { t } = useTranslation()
  const [detail, setDetail] = useState<DetailState | null>(null)
  const requestParams = {
    p: 1,
    page_size: 20,
    connection_id: props.connection?.id,
  }
  const { data, error, isError, isLoading, isFetching, refetch } = useQuery({
    queryKey:
      props.open && props.app && props.connection
        ? mcpQueryKeys.tunnelAuditLogsList(props.app.id, requestParams)
        : [...mcpQueryKeys.tunnelApps(), 'audit-logs', 'none'],
    queryFn: async () => {
      if (!props.app || !props.connection) return { items: [], total: 0 }
      const result = await listTunnelAuditLogs(props.app.id, requestParams)
      if (!result.success) {
        throw mcpQueryError(result.message, 'Failed to load tunnel audit logs')
      }
      return {
        items: result.data?.items ?? [],
        total: result.data?.total ?? 0,
      }
    },
    enabled: props.open && !!props.app && !!props.connection,
    placeholderData: (previousData) => previousData,
  })

  useEffect(() => {
    if (!isError) return
    toast.error(
      mcpQueryErrorMessage(error, t('Failed to load tunnel audit logs'))
    )
  }, [error, isError, t])

  const logs = data?.items ?? []

  return (
    <>
      <Dialog open={props.open} onOpenChange={props.onOpenChange}>
        <DialogContent className='max-w-[calc(100%-2rem)] sm:max-w-5xl'>
          <DialogHeader>
            <div className='flex items-start justify-between gap-3 pr-8'>
              <div className='min-w-0'>
                <DialogTitle>{t('Tunnel Audit Logs')}</DialogTitle>
                <DialogDescription>
                  {props.connection
                    ? `${props.connection.name} · ${props.connection.key_prefix}`
                    : t('Recent tunnel events for this connection.')}
                </DialogDescription>
              </div>
              <Button
                type='button'
                variant='outline'
                size='sm'
                disabled={isFetching}
                onClick={() => void refetch()}
              >
                <RefreshCw
                  className={cn('size-3.5', isFetching && 'animate-spin')}
                />
                {t('Refresh')}
              </Button>
            </div>
          </DialogHeader>

          <div className='max-h-[62vh] overflow-auto rounded-lg border'>
            <table className='w-full min-w-[900px] text-sm'>
              <thead className='bg-muted/40 sticky top-0 z-10 border-b'>
                <tr className='text-muted-foreground text-left text-xs'>
                  <th className='px-3 py-2 font-medium'>{t('Event')}</th>
                  <th className='px-3 py-2 font-medium'>{t('Trace')}</th>
                  <th className='px-3 py-2 font-medium'>{t('Tool')}</th>
                  <th className='px-3 py-2 font-medium'>{t('Size')}</th>
                  <th className='px-3 py-2 font-medium'>{t('Created At')}</th>
                  <th className='w-12 px-3 py-2 font-medium'>
                    <span className='sr-only'>{t('Actions')}</span>
                  </th>
                </tr>
              </thead>
              <tbody>
                {isLoading ? (
                  Array.from({ length: 4 }).map((_, index) => (
                    <tr key={index} className='border-b last:border-0'>
                      <td className='px-3 py-3' colSpan={6}>
                        <div className='bg-muted h-4 w-full max-w-3xl animate-pulse rounded' />
                      </td>
                    </tr>
                  ))
                ) : logs.length === 0 ? (
                  <tr>
                    <td
                      className='text-muted-foreground px-3 py-8 text-center'
                      colSpan={6}
                    >
                      {t('No Tunnel Audit Logs Found')}
                    </td>
                  </tr>
                ) : (
                  logs.map((log) => (
                    <TunnelAuditLogRow
                      key={log.id}
                      log={log}
                      onOpenDetail={setDetail}
                    />
                  ))
                )}
              </tbody>
            </table>
          </div>
        </DialogContent>
      </Dialog>

      {detail && (
        <JsonDetailDialog
          open={detail != null}
          onOpenChange={(open) => !open && setDetail(null)}
          title={detail.title}
          description={detail.description}
          value={detail.value}
        />
      )}
    </>
  )
}

function TunnelAuditLogRow(props: {
  log: TunnelAuditLog
  onOpenDetail: (detail: DetailState) => void
}) {
  const { t } = useTranslation()
  const log = props.log
  return (
    <tr className='border-b align-top last:border-0'>
      <td className='px-3 py-3'>
        <div className='flex min-w-[150px] flex-col gap-1.5'>
          <div className='flex flex-wrap items-center gap-1.5'>
            <StatusBadge
              label={auditLabel(log.decision, t)}
              variant={tunnelDecisionVariant(log.decision)}
              copyable={false}
            />
            <StatusBadge
              label={auditLabel(log.action, t)}
              autoColor={log.action}
              copyable={false}
            />
          </div>
          <LongTextCell value={log.reason} className='max-w-[220px] text-xs' />
        </div>
      </td>
      <td className='px-3 py-3'>
        <TraceCell
          items={[
            { label: t('Request'), value: log.request_id },
            { label: t('Session'), value: log.session_id },
            { label: t('Key'), value: log.connection_key_prefix },
          ]}
        />
      </td>
      <td className='px-3 py-3'>
        <div className='flex min-w-[160px] flex-col gap-1'>
          <LongTextCell
            value={log.tool_name || log.method}
            className='max-w-[180px] font-mono'
          />
          <LongTextCell
            value={log.path}
            className='text-muted-foreground max-w-[180px] text-xs'
          />
        </div>
      </td>
      <td className='px-3 py-3'>
        <div className='flex min-w-[110px] flex-col gap-1'>
          <SizeCell value={log.bytes_in} />
          <SizeCell value={log.bytes_out} />
          <DurationCell value={log.duration_ms} />
        </div>
      </td>
      <td className='px-3 py-3'>
        <TimestampCell value={log.created_at} />
      </td>
      <td className='px-3 py-2 text-right'>
        <Button
          type='button'
          variant='ghost'
          size='icon-sm'
          onClick={() =>
            props.onOpenDetail({
              title: t('Tunnel Audit Log Detail'),
              description: log.request_id || String(log.id),
              value: log,
            })
          }
          aria-label={t('Details')}
        >
          <FileText className='size-4' />
        </Button>
      </td>
    </tr>
  )
}

function useTunnelConnectionColumns(options: {
  onOpenAgentSetup: (connection: TunnelConnection) => void
  onOpenAudit: (connection: TunnelConnection) => void
  onOpenEdit: (connection: TunnelConnection) => void
  onRevoke: (connection: TunnelConnection) => void
}): ColumnDef<TunnelConnection>[] {
  const { t } = useTranslation()
  return useMemo(
    () => [
      {
        accessorKey: 'id',
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title='ID' />
        ),
        cell: ({ row }) => <IdCell value={row.original.id} />,
        meta: { label: t('ID'), mobileHidden: true },
      },
      {
        accessorKey: 'name',
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('Connection')} />
        ),
        cell: ({ row }) => (
          <div className='flex min-w-[220px] flex-col gap-1'>
            <div className='flex min-w-0 items-center gap-2'>
              <LongText className='max-w-[180px] font-medium'>
                {row.original.name}
              </LongText>
              <StatusBadge
                label={tunnelLabel(getDisplayStatus(row.original), t)}
                variant={connectionStatusVariant(
                  row.original.status,
                  row.original.expires_at
                )}
                copyable={false}
              />
            </div>
            <LongText className='text-muted-foreground max-w-[240px] font-mono text-xs'>
              {row.original.endpoint_path}
            </LongText>
          </div>
        ),
        enableHiding: false,
        meta: { label: t('Connection'), mobileTitle: true },
      },
      {
        accessorKey: 'key_prefix',
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('Key Prefix')} />
        ),
        cell: ({ row }) => (
          <StatusBadge
            label={row.original.key_prefix}
            copyText={row.original.key_prefix}
            autoColor={row.original.key_prefix}
          />
        ),
        meta: { label: t('Key Prefix'), mobileHidden: true },
      },
      {
        accessorKey: 'permission_mode',
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('Permission')} />
        ),
        cell: ({ row }) => (
          <StatusBadge
            label={tunnelLabel(row.original.permission_mode, t)}
            autoColor={row.original.permission_mode}
            copyable={false}
          />
        ),
        meta: { label: t('Permission'), mobileBadge: true },
      },
      {
        accessorKey: 'status',
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('Status')} />
        ),
        cell: ({ row }) => (
          <StatusBadge
            label={tunnelLabel(getDisplayStatus(row.original), t)}
            variant={connectionStatusVariant(
              row.original.status,
              row.original.expires_at
            )}
            copyable={false}
          />
        ),
        filterFn: (row, id, value) =>
          Array.isArray(value) && value.includes(String(row.getValue(id))),
        meta: { label: t('Status'), mobileBadge: true },
      },
      {
        accessorKey: 'endpoint_path',
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('MCP Endpoint')} />
        ),
        cell: ({ row }) => {
          const endpoint = formatFullEndpoint(row.original.endpoint_path)
          return (
            <div className='flex min-w-[240px] items-center gap-1.5'>
              <LongTextCell
                value={endpoint}
                className='max-w-[300px] font-mono'
              />
              <CopyButton
                value={endpoint}
                size='icon'
                tooltip={t('Copy MCP Endpoint')}
              />
            </div>
          )
        },
        meta: { label: t('MCP Endpoint') },
      },
      {
        accessorKey: 'last_request_id',
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('Last Request')} />
        ),
        cell: ({ row }) => (
          <LongTextCell
            value={row.original.last_request_id}
            className='max-w-[180px] font-mono'
          />
        ),
        meta: { label: t('Last Request'), mobileHidden: true },
      },
      {
        accessorKey: 'last_used_at',
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('Last Used At')} />
        ),
        cell: ({ row }) =>
          row.original.last_used_at ? (
            <TimestampCell value={row.original.last_used_at} />
          ) : (
            <span className='text-muted-foreground'>-</span>
          ),
        meta: { label: t('Last Used At'), mobileHidden: true },
      },
      {
        accessorKey: 'expires_at',
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('Expires')} />
        ),
        cell: ({ row }) => {
          const label = formatExpiry(row.original.expires_at, t)
          return label ? (
            <span className='text-muted-foreground'>{label}</span>
          ) : (
            <TimestampCell value={row.original.expires_at} />
          )
        },
        meta: { label: t('Expires'), mobileHidden: true },
      },
      {
        id: 'actions',
        cell: ({ row }) => (
          <div className='flex items-center justify-end gap-1'>
            <Button
              type='button'
              variant='ghost'
              size='icon-sm'
              disabled={row.original.status !== 'active'}
              onClick={() => options.onOpenAgentSetup(row.original)}
              aria-label={t('Agent Setup')}
            >
              <KeyRound className='size-4' />
            </Button>
            <Button
              type='button'
              variant='ghost'
              size='icon-sm'
              onClick={() => options.onOpenEdit(row.original)}
              aria-label={t('Edit')}
            >
              <Pencil className='size-4' />
            </Button>
            <Button
              type='button'
              variant='ghost'
              size='icon-sm'
              onClick={() => options.onOpenAudit(row.original)}
              aria-label={t('View Audit Logs')}
            >
              <History className='size-4' />
            </Button>
            <Button
              type='button'
              variant='ghost'
              size='icon-sm'
              disabled={row.original.status !== 'active'}
              onClick={() => options.onRevoke(row.original)}
              aria-label={t('Revoke')}
            >
              <Trash2 className='size-4' />
            </Button>
          </div>
        ),
        enableSorting: false,
        enableHiding: false,
        meta: { label: t('Actions') },
      },
    ],
    [options, t]
  )
}

export function TunnelConnectionsTable() {
  const { t } = useTranslation()
  const navigate = route.useNavigate()
  const search = route.useSearch()
  const isMobile = useMediaQuery('(max-width: 640px)')
  const [columnVisibility, setColumnVisibility] = useState<VisibilityState>({})
  const [selectedAppId, setSelectedAppId] = useState<number | undefined>()
  const [createOpen, setCreateOpen] = useState(false)
  const [setupTarget, setSetupTarget] = useState<TunnelConnection | null>(null)
  const [auditTarget, setAuditTarget] = useState<TunnelConnection | null>(null)
  const [editTarget, setEditTarget] = useState<TunnelConnection | null>(null)
  const [revokeTarget, setRevokeTarget] = useState<TunnelConnection | null>(
    null
  )

  const {
    globalFilter,
    onGlobalFilterChange,
    columnFilters,
    onColumnFiltersChange,
    pagination,
    onPaginationChange,
    ensurePageInRange,
  } = useTableUrlState({
    search,
    navigate,
    pagination: {
      pageKey: 'tunnelConnectionsPage',
      pageSizeKey: 'tunnelConnectionsPageSize',
      defaultPage: 1,
      defaultPageSize: isMobile ? 10 : 20,
    },
    globalFilter: { enabled: true, key: 'tunnelConnectionFilter' },
    columnFilters: [
      {
        columnId: 'status',
        searchKey: 'tunnelConnectionStatus',
        type: 'array',
      },
    ],
  })

  const appsParams = {
    p: 1,
    page_size: 100,
    status: 'approved',
  }
  const {
    data: appsData,
    error: appsError,
    isError: isAppsError,
    isLoading: isAppsLoading,
    isFetching: isAppsFetching,
    refetch: refetchApps,
  } = useQuery({
    queryKey: mcpQueryKeys.userTunnelAppsList(appsParams),
    queryFn: async () => {
      const result = await listUserTunnelApps(appsParams)
      if (!result.success) {
        throw mcpQueryError(result.message, 'Failed to load tunnel apps')
      }
      return result.data?.items ?? []
    },
    placeholderData: (previousData) => previousData,
  })

  const apps = appsData ?? []
  useEffect(() => {
    const requestedAppId = search.tunnelConnectionAppId
    if (requestedAppId && apps.some((app) => app.id === requestedAppId)) {
      if (selectedAppId !== requestedAppId) setSelectedAppId(requestedAppId)
      return
    }
    if (selectedAppId && apps.some((app) => app.id === selectedAppId)) return
    setSelectedAppId(apps[0]?.id)
  }, [apps, search.tunnelConnectionAppId, selectedAppId])

  const selectedApp =
    apps.find((app) => app.id === selectedAppId) ?? apps[0] ?? null

  const handleSelectApp = (appId: number) => {
    setSelectedAppId(appId)
    void navigate({
      search: (prev) => ({
        ...prev,
        tunnelConnectionAppId: appId,
        tunnelConnectionsPage: undefined,
      }),
    })
  }

  const statusFilter =
    (columnFilters.find((filter) => filter.id === 'status')?.value as
      | string[]
      | undefined) ?? []
  const requestParams = {
    p: pagination.pageIndex + 1,
    page_size: pagination.pageSize,
    keyword: globalFilter,
    status: statusFilter[0],
  }

  const {
    data,
    error: connectionsError,
    isError: isConnectionsError,
    isLoading,
    isFetching,
    refetch,
  } = useQuery({
    queryKey: selectedApp
      ? mcpQueryKeys.tunnelConnectionsList(selectedApp.id, requestParams)
      : [...mcpQueryKeys.tunnelApps(), 'connections', 'none', requestParams],
    queryFn: async () => {
      if (!selectedApp) return { items: [], total: 0 }
      const result = await listTunnelConnections(selectedApp.id, requestParams)
      if (!result.success) {
        throw mcpQueryError(result.message, 'Failed to load tunnel connections')
      }
      return {
        items: result.data?.items ?? [],
        total: result.data?.total ?? 0,
      }
    },
    enabled: !!selectedApp,
    placeholderData: (previousData) => previousData,
  })

  const revokeMutation = useMutation({
    mutationFn: async (connection: TunnelConnection) => {
      if (!selectedApp) throw new Error(t('Please select a tunnel app'))
      return revokeTunnelConnection(selectedApp.id, connection.id)
    },
    onSuccess: (result) => {
      if (!result.success) {
        toast.error(result.message || t('Failed to revoke tunnel connection'))
        return
      }
      toast.success(t('Tunnel connection revoked'))
      setRevokeTarget(null)
      void refetch()
    },
    onError: (error) => {
      toast.error(
        mcpQueryErrorMessage(error, t('Failed to revoke tunnel connection'))
      )
    },
  })

  useEffect(() => {
    if (!isAppsError) return
    toast.error(
      mcpQueryErrorMessage(appsError, t('Failed to load tunnel apps'))
    )
  }, [appsError, isAppsError, t])

  useEffect(() => {
    if (!isConnectionsError) return
    toast.error(
      mcpQueryErrorMessage(
        connectionsError,
        t('Failed to load tunnel connections')
      )
    )
  }, [connectionsError, isConnectionsError, t])

  const columns = useTunnelConnectionColumns({
    onOpenAgentSetup: (connection) => setSetupTarget(connection),
    onOpenAudit: (connection) => setAuditTarget(connection),
    onOpenEdit: (connection) => setEditTarget(connection),
    onRevoke: (connection) => setRevokeTarget(connection),
  })
  const table = useReactTable({
    data: data?.items ?? [],
    columns,
    pageCount: Math.ceil((data?.total ?? 0) / pagination.pageSize),
    state: {
      columnFilters,
      columnVisibility,
      globalFilter,
      pagination,
    },
    onColumnFiltersChange,
    onColumnVisibilityChange: setColumnVisibility,
    onGlobalFilterChange,
    onPaginationChange,
    getCoreRowModel: getCoreRowModel(),
    manualFiltering: true,
    manualPagination: true,
  })

  const pageCount = table.getPageCount()
  useEffect(() => {
    ensurePageInRange(pageCount)
  }, [ensurePageInRange, pageCount])

  const refreshAll = () => {
    void refetchApps()
    void refetch()
  }

  return (
    <>
      <DataTablePage
        table={table}
        columns={columns}
        isLoading={isLoading || isAppsLoading}
        isFetching={isFetching || isAppsFetching || revokeMutation.isPending}
        emptyTitle={t(
          apps.length === 0
            ? 'No Approved Tunnel Apps'
            : 'No Tunnel Connections Found'
        )}
        emptyDescription={t(
          apps.length === 0
            ? 'Approved tunnel apps will appear here.'
            : 'Create a dedicated connection key before connecting a tunnel client.'
        )}
        emptyAction={
          apps.length > 0 ? (
            <Button onClick={() => setCreateOpen(true)}>
              <Plus className='size-4' />
              {t('New Connection')}
            </Button>
          ) : null
        }
        skeletonKeyPrefix='tunnel-connections-skeleton'
        toolbarProps={{
          searchPlaceholder: t('Filter by connection name or key prefix...'),
          additionalSearch: (
            <AppSelector
              apps={apps}
              loading={isAppsLoading}
              selectedAppId={selectedApp?.id}
              onChange={handleSelectApp}
            />
          ),
          filters: [
            {
              columnId: 'status',
              title: t('Status'),
              options: [
                { label: t('Active'), value: 'active' },
                { label: t('Revoked'), value: 'revoked' },
              ],
              singleSelect: true,
            },
          ],
          preActions: (
            <>
              <Button
                type='button'
                variant='outline'
                onClick={refreshAll}
                disabled={isFetching || isAppsFetching}
                className={cn((isFetching || isAppsFetching) && 'opacity-80')}
              >
                <RefreshCw
                  className={cn(
                    'size-4',
                    (isFetching || isAppsFetching) && 'animate-spin'
                  )}
                />
                {t('Refresh')}
              </Button>
              <Button
                type='button'
                onClick={() => setCreateOpen(true)}
                disabled={!selectedApp}
              >
                <Plus className='size-4' />
                {t('New Connection')}
              </Button>
            </>
          ),
        }}
        afterTable={
          selectedApp ? (
            <div className='text-muted-foreground flex flex-wrap items-center gap-2 text-xs'>
              <span>{t('Selected Tunnel App')}</span>
              <StatusBadge
                label={selectedApp.name}
                autoColor={selectedApp.public_slug}
                copyable={false}
              />
              <span className='font-mono'>{selectedApp.public_slug}</span>
              <CopyButton
                value={formatFullEndpoint(
                  `/t/<connection_key>/tunnel/mcp/${selectedApp.public_slug}`
                )}
                variant='ghost'
                size='sm'
                tooltip={t('Copy Endpoint Template')}
                className='h-6 px-2 text-xs'
              >
                {t('Copy Endpoint Template')}
              </CopyButton>
            </div>
          ) : null
        }
      />

      <CreateTunnelConnectionDialog
        app={selectedApp}
        open={createOpen}
        onOpenChange={setCreateOpen}
        onCreated={() => void refetch()}
      />
      <RevokeTunnelConnectionDialog
        connection={revokeTarget}
        open={revokeTarget != null}
        onOpenChange={(open) => !open && setRevokeTarget(null)}
        onConfirm={(connection) => revokeMutation.mutate(connection)}
        pending={revokeMutation.isPending}
      />
      <EditTunnelConnectionDialog
        app={selectedApp}
        connection={editTarget}
        open={editTarget != null}
        onOpenChange={(open) => !open && setEditTarget(null)}
        onUpdated={() => void refetch()}
      />
      <TunnelAgentSetupDialog
        app={selectedApp}
        connection={setupTarget}
        open={setupTarget != null}
        onOpenChange={(open) => !open && setSetupTarget(null)}
        onUpdated={() => void refetch()}
      />
      <TunnelConnectionAuditDialog
        app={selectedApp}
        connection={auditTarget}
        open={auditTarget != null}
        onOpenChange={(open) => !open && setAuditTarget(null)}
      />
    </>
  )
}
