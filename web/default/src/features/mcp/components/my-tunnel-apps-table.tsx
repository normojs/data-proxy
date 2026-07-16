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
import { Cable, KeyRound, Loader2, Plus, RefreshCw } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { cn } from '@/lib/utils'
import { useTableUrlState } from '@/hooks/use-table-url-state'
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
  createTunnelApp,
  ensureBridgeAgentSetup,
  listBridgeClients,
  listUserTunnelApps,
  mcpQueryKeys,
} from '../api'
import { mcpQueryError, mcpQueryErrorMessage } from '../lib/query-errors'
import type {
  BridgeAgentSetupResponse,
  BridgeClient,
  TunnelApp,
} from '../types'
import { IdCell, LongTextCell, TimestampCell } from './table-cells'

const route = getRouteApi('/_authenticated/mcp/$section')

type TunnelAppCreateForm = {
  name: string
  description: string
  appType: 'mcp_code' | 'http_tunnel' | 'tcp_tunnel'
  bridgeClientId: string
  permissionMode: string
  targetHost: string
  targetPort: string
  targetPath: string
  localScheme: 'http' | 'https'
  httpAuthMode: 'private' | 'token' | 'public'
  httpAuthToken: string
  routeHost: string
  routePathPrefix: string
  maxRequestBytes: string
  maxResponseBytes: string
}

type BridgeAgentSetupForm = {
  clientId: string
  clientName: string
  platform: string
  workspace: string
  version: string
  rotate: boolean
}

const permissionOptions = ['read_only', 'write', 'exec_safe', 'exec_trusted']

function tunnelStatusVariant(status: string): StatusBadgeProps['variant'] {
  if (status === 'approved') return 'green'
  if (status === 'pending') return 'yellow'
  if (status === 'rejected') return 'red'
  if (status === 'disabled' || status === 'archived') return 'grey'
  return 'neutral'
}

function bridgeStatusVariant(
  client: BridgeClient
): StatusBadgeProps['variant'] {
  return client.online || client.status === 1 ? 'green' : 'grey'
}

function tunnelLabel(value: string, t: (key: string) => string): string {
  const labels: Record<string, string> = {
    active: t('Active'),
    archived: t('Archived'),
    approved: t('Approved'),
    disabled: t('Disabled'),
    exec_safe: t('Exec Safe'),
    exec_trusted: t('Exec Trusted'),
    http_tunnel: t('HTTP Tunnel'),
    mcp_code: t('MCP Code Tunnel'),
    pending: t('Pending'),
    read_only: t('Read Only'),
    rejected: t('Rejected'),
    tcp_tunnel: t('TCP Tunnel'),
    traffic: t('Traffic'),
    write: t('Write'),
  }
  return labels[value] || value
}

function buildInitialForm(): TunnelAppCreateForm {
  return {
    name: '',
    description: '',
    appType: 'http_tunnel',
    bridgeClientId: '',
    permissionMode: 'traffic',
    targetHost: '127.0.0.1',
    targetPort: '8080',
    targetPath: '/',
    localScheme: 'http',
    httpAuthMode: 'private',
    httpAuthToken: '',
    routeHost: '',
    routePathPrefix: '/',
    maxRequestBytes: '',
    maxResponseBytes: '',
  }
}

function buildInitialBridgeAgentSetupForm(): BridgeAgentSetupForm {
  return {
    clientId: '',
    clientName: 'Desktop Bridge Agent',
    platform: '',
    workspace: '',
    version: '',
    rotate: false,
  }
}

function getLocalScheme(app: TunnelApp): 'http' | 'https' {
  const route = app.route || {}
  const raw =
    (typeof route.local_scheme === 'string' && route.local_scheme) ||
    (typeof route.target_scheme === 'string' && route.target_scheme) ||
    (typeof route.scheme === 'string' && route.scheme) ||
    ''
  return raw.toLowerCase() === 'https' ? 'https' : 'http'
}

function getAppTarget(app: TunnelApp): string {
  if (app.app_type === 'mcp_code') return app.target_path || '/mcp'
  if (app.app_type === 'tcp_tunnel') {
    return `${app.target_host || '127.0.0.1'}:${app.target_port}`
  }
  const path = app.target_path || '/'
  if (path.startsWith('http://') || path.startsWith('https://')) return path
  const scheme = getLocalScheme(app)
  return `${scheme}://${app.target_host || '127.0.0.1'}:${app.target_port}${path.startsWith('/') ? path : `/${path}`}`
}

function formatJSON(value: unknown): string {
  return JSON.stringify(value, null, 2)
}

function clientOptionLabel(client: BridgeClient): string {
  const name = client.name || client.client_id
  const workspace = client.workspace ? ` · ${client.workspace}` : ''
  return `${name}${workspace}`
}

function findClient(clients: BridgeClient[], clientId: string) {
  return clients.find((client) => client.client_id === clientId) ?? null
}

function isOptionalPositiveInteger(value: string) {
  if (!value.trim()) return true
  const number = Number(value)
  return Number.isInteger(number) && number > 0
}

function buildTunnelAppCreatePayload(form: TunnelAppCreateForm) {
  if (form.appType === 'tcp_tunnel') {
    return {
      name: form.name.trim(),
      description: form.description.trim(),
      app_type: 'tcp_tunnel',
      permission_mode: 'traffic',
      bridge_client_id: form.bridgeClientId,
      target_host: form.targetHost.trim() || '127.0.0.1',
      target_port: Number(form.targetPort),
      target_path: '',
      policy: {},
      route: {},
    }
  }
  if (form.appType === 'http_tunnel') {
    const route: Record<string, unknown> = {
      auth_mode: form.httpAuthMode,
      path_prefix: form.routePathPrefix.trim() || '/',
      local_scheme: form.localScheme === 'https' ? 'https' : 'http',
    }
    if (form.routeHost.trim()) route.host = form.routeHost.trim()
    if (form.maxRequestBytes.trim()) {
      route.max_request_bytes = Number(form.maxRequestBytes)
    }
    if (form.maxResponseBytes.trim()) {
      route.max_response_bytes = Number(form.maxResponseBytes)
    }
    if (form.httpAuthMode === 'token') {
      route.auth_token = form.httpAuthToken.trim()
    }
    return {
      name: form.name.trim(),
      description: form.description.trim(),
      app_type: 'http_tunnel',
      permission_mode: 'traffic',
      bridge_client_id: form.bridgeClientId,
      target_host: form.targetHost.trim() || '127.0.0.1',
      target_port: Number(form.targetPort),
      target_path: form.targetPath.trim() || '/',
      policy: {},
      route,
    }
  }
  return {
    name: form.name.trim(),
    description: form.description.trim(),
    app_type: 'mcp_code',
    permission_mode: form.permissionMode,
    bridge_client_id: form.bridgeClientId,
    target_path: form.targetPath.trim() || '/mcp',
    policy: {},
  }
}

function BridgeClientSelect(props: {
  clients: BridgeClient[]
  value: string
  loading: boolean
  onChange: (value: string) => void
}) {
  const { t } = useTranslation()
  return (
    <NativeSelect
      id='mcp-my-tunnel-app-bridge-client'
      value={props.value}
      disabled={props.loading || props.clients.length === 0}
      onChange={(event) => props.onChange(event.target.value)}
      className='w-full'
    >
      <NativeSelectOption value=''>
        {props.loading ? t('Loading...') : t('Select Bridge Client')}
      </NativeSelectOption>
      {props.clients.map((client) => (
        <NativeSelectOption key={client.client_id} value={client.client_id}>
          {clientOptionLabel(client)}
        </NativeSelectOption>
      ))}
    </NativeSelect>
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

function BridgeAgentSetupDialog(props: {
  open: boolean
  bridgeClients: BridgeClient[]
  bridgeClientsLoading: boolean
  onOpenChange: (open: boolean) => void
  onUpdated: () => void
}) {
  const { t } = useTranslation()
  const [form, setForm] = useState<BridgeAgentSetupForm>(
    buildInitialBridgeAgentSetupForm
  )
  const [setup, setSetup] = useState<BridgeAgentSetupResponse | null>(null)

  useEffect(() => {
    if (props.open) return
    let cancelled = false
    queueMicrotask(() => {
      if (cancelled) return
      setForm(buildInitialBridgeAgentSetupForm())
      setSetup(null)
    })
    return () => {
      cancelled = true
    }
  }, [props.open])

  const selectedClient = findClient(props.bridgeClients, form.clientId)

  const mutation = useMutation({
    mutationFn: () =>
      ensureBridgeAgentSetup({
        client_id: form.clientId || undefined,
        rotate: form.clientId ? form.rotate : false,
        client_name: form.clientName.trim(),
        platform: form.platform.trim(),
        workspace: form.workspace.trim(),
        version: form.version.trim(),
      }),
    onSuccess: (result) => {
      if (!result.success || !result.data) {
        toast.error(result.message || t('Failed to prepare bridge agent setup'))
        return
      }
      setSetup(result.data)
      props.onUpdated()
      toast.success(
        result.data.rotated
          ? t('Bridge agent key rotated')
          : t('Bridge agent setup ready')
      )
    },
    onError: (error) => {
      toast.error(
        mcpQueryErrorMessage(error, t('Failed to prepare bridge agent setup'))
      )
    },
  })

  const envSnippet = setup
    ? Object.entries(setup.environment)
        .map(([key, value]) => `${key}=${JSON.stringify(value)}`)
        .join('\n')
    : ''
  const registerSnippet = setup ? formatJSON(setup.register) : ''
  const configSnippet = setup ? formatJSON(setup.config) : ''

  return (
    <Dialog open={props.open} onOpenChange={props.onOpenChange}>
      <DialogContent className='max-w-[calc(100%-2rem)] sm:max-w-3xl'>
        <DialogHeader>
          <DialogTitle>{t('Local Agent Setup')}</DialogTitle>
          <DialogDescription>
            {setup
              ? t(
                  'Copy the API key now if it is shown. Full keys are only returned after creation or rotation.'
                )
              : t(
                  'Generate a dedicated local Bridge Agent configuration before requesting a tunnel app.'
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
              <SecretField
                label={t('Bridge Client ID')}
                value={setup.client_id}
              />
              <SecretField
                label={setup.api_key ? t('API Key') : t('Masked API Key')}
                value={setup.api_key || setup.token_masked_key}
              />
              <SecretField
                label={t('Bridge Client')}
                value={setup.client.name || setup.client.client_id}
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
              <div className='space-y-1.5 sm:col-span-2'>
                <Label htmlFor='mcp-bridge-agent-client'>
                  {t('Bridge Client')}
                </Label>
                <NativeSelect
                  id='mcp-bridge-agent-client'
                  value={form.clientId}
                  disabled={props.bridgeClientsLoading}
                  onChange={(event) => {
                    const client = findClient(
                      props.bridgeClients,
                      event.target.value
                    )
                    setForm((current) => ({
                      ...current,
                      clientId: event.target.value,
                      clientName:
                        client?.name ||
                        current.clientName ||
                        event.target.value,
                      platform: client?.platform || current.platform,
                      workspace: client?.workspace || current.workspace,
                      version: client?.version || current.version,
                      rotate: false,
                    }))
                  }}
                  className='w-full'
                >
                  <NativeSelectOption value=''>
                    {t('New Bridge Client')}
                  </NativeSelectOption>
                  {props.bridgeClients.map((client) => (
                    <NativeSelectOption
                      key={client.client_id}
                      value={client.client_id}
                    >
                      {clientOptionLabel(client)}
                    </NativeSelectOption>
                  ))}
                </NativeSelect>
                {selectedClient ? (
                  <div className='text-muted-foreground flex min-w-0 flex-wrap items-center gap-2 text-xs'>
                    <StatusBadge
                      label={t(selectedClient.online ? 'Online' : 'Offline')}
                      variant={bridgeStatusVariant(selectedClient)}
                      copyable={false}
                    />
                    <LongText className='max-w-[360px] font-mono'>
                      {selectedClient.client_id}
                    </LongText>
                  </div>
                ) : (
                  <p className='text-muted-foreground text-xs'>
                    {t(
                      'Create a reserved offline Bridge client and copy its local agent credentials.'
                    )}
                  </p>
                )}
              </div>
              <div className='space-y-1.5'>
                <Label htmlFor='mcp-bridge-agent-name'>{t('Agent Name')}</Label>
                <Input
                  id='mcp-bridge-agent-name'
                  value={form.clientName}
                  onChange={(event) =>
                    setForm((current) => ({
                      ...current,
                      clientName: event.target.value,
                    }))
                  }
                  placeholder={t('Desktop Bridge Agent')}
                />
              </div>
              <div className='space-y-1.5'>
                <Label htmlFor='mcp-bridge-agent-platform'>
                  {t('Platform')}
                </Label>
                <Input
                  id='mcp-bridge-agent-platform'
                  value={form.platform}
                  onChange={(event) =>
                    setForm((current) => ({
                      ...current,
                      platform: event.target.value,
                    }))
                  }
                  placeholder='darwin'
                />
              </div>
              <div className='space-y-1.5 sm:col-span-2'>
                <Label htmlFor='mcp-bridge-agent-workspace'>
                  {t('Workspace')}
                </Label>
                <Input
                  id='mcp-bridge-agent-workspace'
                  value={form.workspace}
                  onChange={(event) =>
                    setForm((current) => ({
                      ...current,
                      workspace: event.target.value,
                    }))
                  }
                  placeholder='/workspace/project'
                />
              </div>
              <div className='space-y-1.5'>
                <Label htmlFor='mcp-bridge-agent-version'>{t('Version')}</Label>
                <Input
                  id='mcp-bridge-agent-version'
                  value={form.version}
                  onChange={(event) =>
                    setForm((current) => ({
                      ...current,
                      version: event.target.value,
                    }))
                  }
                  placeholder='1.0.0'
                />
              </div>
            </div>
            <label className='flex items-start gap-2 rounded-lg border p-3 text-sm'>
              <input
                type='checkbox'
                checked={form.rotate}
                disabled={!form.clientId}
                onChange={(event) =>
                  setForm((current) => ({
                    ...current,
                    rotate: event.target.checked,
                  }))
                }
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
                disabled={props.bridgeClientsLoading || mutation.isPending}
              >
                {mutation.isPending ? (
                  <Loader2 className='size-4 animate-spin' />
                ) : (
                  <Cable className='size-4' />
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

function CreateTunnelAppDialog(props: {
  open: boolean
  bridgeClients: BridgeClient[]
  bridgeClientsLoading: boolean
  onOpenChange: (open: boolean) => void
  onCreated: () => void
}) {
  const { t } = useTranslation()
  const [form, setForm] = useState<TunnelAppCreateForm>(buildInitialForm)

  useEffect(() => {
    if (!props.open) return
    let cancelled = false
    queueMicrotask(() => {
      if (cancelled) return
      setForm((current) => {
        if (current.bridgeClientId) return current
        return {
          ...current,
          bridgeClientId: props.bridgeClients[0]?.client_id ?? '',
          name:
            current.name ||
            props.bridgeClients[0]?.name ||
            props.bridgeClients[0]?.client_id ||
            '',
        }
      })
    })
    return () => {
      cancelled = true
    }
  }, [props.bridgeClients, props.open])

  useEffect(() => {
    if (props.open) return
    let cancelled = false
    queueMicrotask(() => {
      if (!cancelled) {
        setForm(buildInitialForm())
      }
    })
    return () => {
      cancelled = true
    }
  }, [props.open])

  const selectedClient = findClient(props.bridgeClients, form.bridgeClientId)

  const mutation = useMutation({
    mutationFn: () => createTunnelApp(buildTunnelAppCreatePayload(form)),
    onSuccess: (result) => {
      if (!result.success) {
        toast.error(result.message || t('Failed to create tunnel app request'))
        return
      }
      toast.success(t('Tunnel app request submitted'))
      props.onOpenChange(false)
      props.onCreated()
    },
    onError: (error) => {
      toast.error(
        mcpQueryErrorMessage(error, t('Failed to create tunnel app request'))
      )
    },
  })

  const targetPort = Number(form.targetPort)
  const isTrafficApp =
    form.appType === 'http_tunnel' || form.appType === 'tcp_tunnel'
  const hasValidTargetPort =
    Number.isInteger(targetPort) && targetPort > 0 && targetPort <= 65535
  const hasValidRouteLimits =
    isOptionalPositiveInteger(form.maxRequestBytes) &&
    isOptionalPositiveInteger(form.maxResponseBytes)
  const canSubmit =
    form.name.trim().length > 0 &&
    form.bridgeClientId.trim().length > 0 &&
    (!isTrafficApp || hasValidTargetPort) &&
    (form.appType !== 'http_tunnel' ||
      (hasValidRouteLimits &&
        (form.httpAuthMode !== 'token' ||
          form.httpAuthToken.trim().length > 0))) &&
    !mutation.isPending

  return (
    <Dialog open={props.open} onOpenChange={props.onOpenChange}>
      <DialogContent className='max-w-[calc(100%-2rem)] sm:max-w-3xl'>
        <DialogHeader>
          <DialogTitle>{t('New Tunnel App Request')}</DialogTitle>
          <DialogDescription>
            {t(
              'Public access links always use the site HTTPS URL. Choose the local service protocol separately (HTTP by default, HTTPS optional). An administrator must approve the app before connection keys can be created.'
            )}
          </DialogDescription>
        </DialogHeader>

        <div className='grid gap-3 sm:grid-cols-2'>
          <div className='space-y-1.5'>
            <Label htmlFor='mcp-my-tunnel-app-name'>{t('Name')}</Label>
            <Input
              id='mcp-my-tunnel-app-name'
              value={form.name}
              onChange={(event) =>
                setForm((current) => ({
                  ...current,
                  name: event.target.value,
                }))
              }
              placeholder={t('Local code workspace')}
            />
          </div>
          <div className='space-y-1.5'>
            <Label htmlFor='mcp-my-tunnel-app-type'>{t('Tunnel Type')}</Label>
            <NativeSelect
              id='mcp-my-tunnel-app-type'
              value={form.appType}
              onChange={(event) => {
                const appType = event.target
                  .value as TunnelAppCreateForm['appType']
                setForm((current) => ({
                  ...current,
                  appType,
                  permissionMode:
                    appType === 'http_tunnel' || appType === 'tcp_tunnel'
                      ? 'traffic'
                      : current.permissionMode === 'traffic'
                        ? 'read_only'
                        : current.permissionMode,
                  targetPort:
                    appType === 'tcp_tunnel' &&
                    (!current.targetPort || current.targetPort === '8080')
                      ? '22'
                      : appType === 'http_tunnel' &&
                          (!current.targetPort || current.targetPort === '22')
                        ? '8080'
                        : current.targetPort,
                  targetPath:
                    appType === 'tcp_tunnel'
                      ? ''
                      : appType === 'http_tunnel'
                        ? current.targetPath === '/mcp'
                          ? '/'
                          : current.targetPath || '/'
                        : current.targetPath === '/' ||
                            current.targetPath === ''
                          ? '/mcp'
                          : current.targetPath || '/mcp',
                  localScheme:
                    appType === 'http_tunnel' ? current.localScheme : 'http',
                }))
              }}
              className='w-full'
            >
              <NativeSelectOption value='http_tunnel'>
                {t('HTTP Tunnel')}
              </NativeSelectOption>
              <NativeSelectOption value='mcp_code'>
                {t('MCP Code Tunnel')}
              </NativeSelectOption>
              <NativeSelectOption value='tcp_tunnel'>
                {t('TCP Tunnel')}
              </NativeSelectOption>
            </NativeSelect>
            <p className='text-muted-foreground text-xs'>
              {form.appType === 'http_tunnel'
                ? t(
                    'Expose a local web service. Public URL is HTTPS; local service can be HTTP or HTTPS.'
                  )
                : form.appType === 'mcp_code'
                  ? t(
                      'Expose local MCP/code tools to cloud agents with permission controls.'
                    )
                  : t(
                      'Expose a local TCP service through a WebSocket tunnel (advanced).'
                    )}
            </p>
          </div>
          {form.appType === 'mcp_code' ? (
            <div className='space-y-1.5'>
              <Label htmlFor='mcp-my-tunnel-app-permission'>
                {t('Local Permission')}
              </Label>
              <NativeSelect
                id='mcp-my-tunnel-app-permission'
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
              <p className='text-muted-foreground text-xs'>
                {t(
                  'What the cloud agent may do on this machine. Prefer Read Only unless write/exec is required.'
                )}
              </p>
            </div>
          ) : form.appType === 'http_tunnel' ? (
            <div className='space-y-1.5'>
              <Label htmlFor='mcp-my-tunnel-http-auth'>
                {t('Who Can Access')}
              </Label>
              <NativeSelect
                id='mcp-my-tunnel-http-auth'
                value={form.httpAuthMode}
                onChange={(event) =>
                  setForm((current) => ({
                    ...current,
                    httpAuthMode: event.target
                      .value as TunnelAppCreateForm['httpAuthMode'],
                  }))
                }
                className='w-full'
              >
                <NativeSelectOption value='private'>
                  {t('Private link (connection key required)')}
                </NativeSelectOption>
                <NativeSelectOption value='token'>
                  {t('Extra bearer token')}
                </NativeSelectOption>
                <NativeSelectOption value='public'>
                  {t('Public (not recommended)')}
                </NativeSelectOption>
              </NativeSelect>
              <p className='text-muted-foreground text-xs'>
                {t(
                  'Private is safest: only people with the connection key URL can reach the tunnel.'
                )}
              </p>
            </div>
          ) : (
            <div className='space-y-1.5'>
              <Label htmlFor='mcp-my-tunnel-tcp-permission'>
                {t('Permission')}
              </Label>
              <Input
                id='mcp-my-tunnel-tcp-permission'
                value={t('Traffic')}
                readOnly
              />
            </div>
          )}
          <div className='space-y-1.5 sm:col-span-2'>
            <Label htmlFor='mcp-my-tunnel-app-bridge-client'>
              {t('Local Device (Bridge Client)')}
            </Label>
            <BridgeClientSelect
              clients={props.bridgeClients}
              loading={props.bridgeClientsLoading}
              value={form.bridgeClientId}
              onChange={(value) => {
                const nextClient = findClient(props.bridgeClients, value)
                setForm((current) => ({
                  ...current,
                  bridgeClientId: value,
                  name: current.name || nextClient?.name || value,
                }))
              }}
            />
            {selectedClient ? (
              <div className='text-muted-foreground flex min-w-0 flex-wrap items-center gap-2 text-xs'>
                <StatusBadge
                  label={t(selectedClient.online ? 'Online' : 'Offline')}
                  variant={bridgeStatusVariant(selectedClient)}
                  copyable={false}
                />
                <LongText className='max-w-[360px]'>
                  {selectedClient.workspace || selectedClient.client_id}
                </LongText>
              </div>
            ) : (
              <p className='text-muted-foreground text-xs'>
                {t(
                  'Install and enroll dpa first, then pick the online device here.'
                )}
              </p>
            )}
          </div>
          {isTrafficApp ? (
            <>
              {form.appType === 'http_tunnel' ? (
                <div className='space-y-1.5'>
                  <Label htmlFor='mcp-my-tunnel-local-scheme'>
                    {t('Local Service Protocol')}
                  </Label>
                  <NativeSelect
                    id='mcp-my-tunnel-local-scheme'
                    value={form.localScheme}
                    onChange={(event) =>
                      setForm((current) => ({
                        ...current,
                        localScheme: event.target
                          .value as TunnelAppCreateForm['localScheme'],
                      }))
                    }
                    className='w-full'
                  >
                    <NativeSelectOption value='http'>
                      {t('HTTP (default)')}
                    </NativeSelectOption>
                    <NativeSelectOption value='https'>
                      {t('HTTPS (local TLS service)')}
                    </NativeSelectOption>
                  </NativeSelect>
                  <p className='text-muted-foreground text-xs'>
                    {t(
                      'Protocol used by dpa when calling your machine. Public site access remains HTTPS automatically.'
                    )}
                  </p>
                </div>
              ) : null}
              <div className='space-y-1.5'>
                <Label htmlFor='mcp-my-tunnel-target-host'>
                  {t('Local Host')}
                </Label>
                <Input
                  id='mcp-my-tunnel-target-host'
                  value={form.targetHost}
                  onChange={(event) =>
                    setForm((current) => ({
                      ...current,
                      targetHost: event.target.value,
                    }))
                  }
                  placeholder='127.0.0.1'
                />
              </div>
              <div className='space-y-1.5'>
                <Label htmlFor='mcp-my-tunnel-target-port'>
                  {t('Local Port')}
                </Label>
                <Input
                  id='mcp-my-tunnel-target-port'
                  type='number'
                  min={1}
                  max={65535}
                  value={form.targetPort}
                  onChange={(event) =>
                    setForm((current) => ({
                      ...current,
                      targetPort: event.target.value,
                    }))
                  }
                  placeholder={form.appType === 'tcp_tunnel' ? '22' : '8080'}
                />
              </div>
            </>
          ) : null}
          {form.appType !== 'tcp_tunnel' ? (
            <div className='space-y-1.5'>
              <Label htmlFor='mcp-my-tunnel-app-target-path'>
                {form.appType === 'http_tunnel'
                  ? t('Local Path Prefix')
                  : t('Target Path')}
              </Label>
              <Input
                id='mcp-my-tunnel-app-target-path'
                value={form.targetPath}
                onChange={(event) =>
                  setForm((current) => ({
                    ...current,
                    targetPath: event.target.value,
                  }))
                }
                placeholder={form.appType === 'http_tunnel' ? '/' : '/mcp'}
              />
            </div>
          ) : null}
          {form.appType === 'http_tunnel' ? (
            <>
              <div className='space-y-1.5 sm:col-span-2 rounded-md border border-dashed px-3 py-2'>
                <div className='text-xs font-medium'>{t('Link preview')}</div>
                <p className='text-muted-foreground text-xs'>
                  {t('Public access link')}:{' '}
                  <span className='text-foreground font-mono'>
                    https://&lt;this-site&gt;/t/&lt;connection_key&gt;/tunnel/http/&lt;slug&gt;/
                  </span>
                </p>
                <p className='text-muted-foreground text-xs'>
                  {t('Local service')}:{' '}
                  <span className='text-foreground font-mono'>
                    {form.localScheme}://
                    {form.targetHost.trim() || '127.0.0.1'}:
                    {form.targetPort || '8080'}
                    {form.targetPath.trim() || '/'}
                  </span>
                </p>
              </div>
              <div className='space-y-1.5'>
                <Label htmlFor='mcp-my-tunnel-http-route-path'>
                  {t('Allowed Public Path Prefix')}
                </Label>
                <Input
                  id='mcp-my-tunnel-http-route-path'
                  value={form.routePathPrefix}
                  onChange={(event) =>
                    setForm((current) => ({
                      ...current,
                      routePathPrefix: event.target.value,
                    }))
                  }
                  placeholder='/'
                />
              </div>
              <div className='space-y-1.5'>
                <Label htmlFor='mcp-my-tunnel-http-route-host'>
                  {t('Required Host Header (optional)')}
                </Label>
                <Input
                  id='mcp-my-tunnel-http-route-host'
                  value={form.routeHost}
                  onChange={(event) =>
                    setForm((current) => ({
                      ...current,
                      routeHost: event.target.value,
                    }))
                  }
                  placeholder='dp.example.com'
                />
              </div>
              <div className='space-y-1.5'>
                <Label htmlFor='mcp-my-tunnel-http-max-request-bytes'>
                  {t('Max Request Bytes')}
                </Label>
                <Input
                  id='mcp-my-tunnel-http-max-request-bytes'
                  type='number'
                  min={1}
                  value={form.maxRequestBytes}
                  onChange={(event) =>
                    setForm((current) => ({
                      ...current,
                      maxRequestBytes: event.target.value,
                    }))
                  }
                  placeholder='8388608'
                />
              </div>
              <div className='space-y-1.5'>
                <Label htmlFor='mcp-my-tunnel-http-max-response-bytes'>
                  {t('Max Response Bytes')}
                </Label>
                <Input
                  id='mcp-my-tunnel-http-max-response-bytes'
                  type='number'
                  min={1}
                  value={form.maxResponseBytes}
                  onChange={(event) =>
                    setForm((current) => ({
                      ...current,
                      maxResponseBytes: event.target.value,
                    }))
                  }
                  placeholder='2097152'
                />
              </div>
              {form.httpAuthMode === 'token' ? (
                <div className='space-y-1.5'>
                  <Label htmlFor='mcp-my-tunnel-http-auth-token'>
                    {t('Auth Token')}
                  </Label>
                  <Input
                    id='mcp-my-tunnel-http-auth-token'
                    value={form.httpAuthToken}
                    onChange={(event) =>
                      setForm((current) => ({
                        ...current,
                        httpAuthToken: event.target.value,
                      }))
                    }
                    placeholder={t('Required for token mode')}
                  />
                </div>
              ) : null}
            </>
          ) : null}
          <div className='space-y-1.5 sm:col-span-2'>
            <Label htmlFor='mcp-my-tunnel-app-description'>
              {t('Description')}
            </Label>
            <Textarea
              id='mcp-my-tunnel-app-description'
              value={form.description}
              onChange={(event) =>
                setForm((current) => ({
                  ...current,
                  description: event.target.value,
                }))
              }
              className='min-h-20 resize-none'
              placeholder={t('Optional note for administrator review')}
            />
          </div>
        </div>

        <DialogFooter>
          <Button variant='outline' onClick={() => props.onOpenChange(false)}>
            {t('Cancel')}
          </Button>
          <Button onClick={() => mutation.mutate()} disabled={!canSubmit}>
            {mutation.isPending ? (
              <Loader2 className='size-4 animate-spin' />
            ) : (
              <Plus className='size-4' />
            )}
            {t('Submit Request')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

function useMyTunnelAppColumns(options: {
  onOpenConnections: (app: TunnelApp) => void
}): ColumnDef<TunnelApp>[] {
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
          <DataTableColumnHeader column={column} title={t('App')} />
        ),
        cell: ({ row }) => {
          const app = row.original
          return (
            <div className='flex min-w-[220px] flex-col gap-1'>
              <div className='flex min-w-0 items-center gap-2'>
                <LongText className='max-w-[180px] font-medium'>
                  {app.name}
                </LongText>
                <StatusBadge
                  label={tunnelLabel(app.status, t)}
                  variant={tunnelStatusVariant(app.status)}
                  copyable={false}
                />
              </div>
              <div className='text-muted-foreground flex min-w-0 items-center gap-1.5'>
                <LongText className='max-w-[220px] font-mono text-xs'>
                  {app.public_slug}
                </LongText>
                <CopyButton
                  value={app.public_slug}
                  size='icon'
                  tooltip={t('Copy Slug')}
                  className='size-6'
                />
              </div>
            </div>
          )
        },
        enableHiding: false,
        meta: { label: t('App'), mobileTitle: true },
      },
      {
        accessorKey: 'status',
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('Status')} />
        ),
        cell: ({ row }) => (
          <StatusBadge
            label={tunnelLabel(row.original.status, t)}
            variant={tunnelStatusVariant(row.original.status)}
            copyable={false}
          />
        ),
        filterFn: (row, id, value) =>
          Array.isArray(value) && value.includes(String(row.getValue(id))),
        meta: { label: t('Status'), mobileBadge: true },
      },
      {
        accessorKey: 'app_type',
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('Type')} />
        ),
        cell: ({ row }) => (
          <StatusBadge
            label={tunnelLabel(row.original.app_type, t)}
            autoColor={row.original.app_type}
            copyable={false}
          />
        ),
        filterFn: (row, id, value) =>
          Array.isArray(value) && value.includes(String(row.getValue(id))),
        meta: { label: t('Type'), mobileHidden: true },
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
        accessorKey: 'bridge_client_id',
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('Bridge Client')} />
        ),
        cell: ({ row }) => (
          <LongTextCell
            value={row.original.bridge_client_id}
            className='max-w-[220px] font-mono'
          />
        ),
        meta: { label: t('Bridge Client') },
      },
      {
        accessorKey: 'target_path',
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('Target')} />
        ),
        cell: ({ row }) => (
          <LongTextCell
            value={getAppTarget(row.original)}
            className='max-w-[220px] font-mono'
          />
        ),
        meta: { label: t('Target'), mobileHidden: true },
      },
      {
        accessorKey: 'review_note',
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('Review Note')} />
        ),
        cell: ({ row }) => (
          <LongTextCell
            value={row.original.review_note}
            className='max-w-[260px]'
          />
        ),
        meta: { label: t('Review Note'), mobileHidden: true },
      },
      {
        accessorKey: 'approved_at',
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('Approved At')} />
        ),
        cell: ({ row }) =>
          row.original.approved_at ? (
            <TimestampCell value={row.original.approved_at} />
          ) : (
            <span className='text-muted-foreground'>-</span>
          ),
        meta: { label: t('Approved At'), mobileHidden: true },
      },
      {
        accessorKey: 'created_at',
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('Submitted')} />
        ),
        cell: ({ row }) => <TimestampCell value={row.original.created_at} />,
        meta: { label: t('Submitted'), mobileHidden: true },
      },
      {
        id: 'actions',
        cell: ({ row }) => (
          <Button
            variant='ghost'
            size='icon-sm'
            disabled={row.original.status !== 'approved'}
            onClick={() => options.onOpenConnections(row.original)}
            aria-label={t('Open Connections')}
          >
            <KeyRound className='size-4' />
          </Button>
        ),
        enableSorting: false,
        enableHiding: false,
        meta: { label: t('Actions') },
      },
    ],
    [options, t]
  )
}

export function MyTunnelAppsTable() {
  const { t } = useTranslation()
  const navigate = route.useNavigate()
  const search = route.useSearch()
  const isMobile = useMediaQuery('(max-width: 640px)')
  const [columnVisibility, setColumnVisibility] = useState<VisibilityState>({})
  const [createOpen, setCreateOpen] = useState(false)
  const [agentSetupOpen, setAgentSetupOpen] = useState(false)

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
      pageKey: 'myTunnelAppsPage',
      pageSizeKey: 'myTunnelAppsPageSize',
      defaultPage: 1,
      defaultPageSize: isMobile ? 10 : 20,
    },
    globalFilter: { enabled: true, key: 'myTunnelFilter' },
    columnFilters: [
      { columnId: 'status', searchKey: 'myTunnelStatus', type: 'array' },
      { columnId: 'app_type', searchKey: 'myTunnelType', type: 'array' },
    ],
  })

  const statusFilter =
    (columnFilters.find((filter) => filter.id === 'status')?.value as
      | string[]
      | undefined) ?? []
  const typeFilter =
    (columnFilters.find((filter) => filter.id === 'app_type')?.value as
      | string[]
      | undefined) ?? []

  const requestParams = {
    p: pagination.pageIndex + 1,
    page_size: pagination.pageSize,
    app_type: typeFilter[0],
    keyword: globalFilter,
    status: statusFilter[0],
  }

  const {
    data,
    error: tunnelAppsError,
    isError: isTunnelAppsError,
    isLoading,
    isFetching,
    refetch,
  } = useQuery({
    queryKey: mcpQueryKeys.userTunnelAppsList(requestParams),
    queryFn: async () => {
      const result = await listUserTunnelApps(requestParams)
      if (!result.success) {
        throw mcpQueryError(result.message, 'Failed to load tunnel apps')
      }
      return {
        items: result.data?.items ?? [],
        total: result.data?.total ?? 0,
      }
    },
    placeholderData: (previousData) => previousData,
  })

  const bridgeClientParams = { p: 1, page_size: 100 }
  const {
    data: bridgeClientsData,
    error: bridgeClientsError,
    isError: isBridgeClientsError,
    isLoading: isBridgeClientsLoading,
    isFetching: isBridgeClientsFetching,
    refetch: refetchBridgeClients,
  } = useQuery({
    queryKey: mcpQueryKeys.bridgeClientsList(bridgeClientParams),
    queryFn: async () => {
      const result = await listBridgeClients(bridgeClientParams)
      if (!result.success) {
        throw mcpQueryError(result.message, 'Failed to load bridge clients')
      }
      return result.data?.items ?? []
    },
    placeholderData: (previousData) => previousData,
  })

  useEffect(() => {
    if (!isTunnelAppsError) return
    toast.error(
      mcpQueryErrorMessage(tunnelAppsError, t('Failed to load tunnel apps'))
    )
  }, [isTunnelAppsError, t, tunnelAppsError])

  useEffect(() => {
    if (!isBridgeClientsError) return
    toast.error(
      mcpQueryErrorMessage(
        bridgeClientsError,
        t('Failed to load bridge clients')
      )
    )
  }, [bridgeClientsError, isBridgeClientsError, t])

  const columns = useMyTunnelAppColumns({
    onOpenConnections: (app) => {
      void navigate({
        to: '/mcp/$section',
        params: { section: 'tunnel-connections' },
        search: (prev) => ({
          ...prev,
          tunnelConnectionAppId: app.id,
          tunnelConnectionsPage: undefined,
          tunnelConnectionFilter: '',
          tunnelConnectionStatus: undefined,
        }),
      })
    },
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
    void refetch()
    void refetchBridgeClients()
  }

  const bridgeClients = bridgeClientsData ?? []
  const loadingBridgeClients = isBridgeClientsLoading || isBridgeClientsFetching

  return (
    <>
      <DataTablePage
        table={table}
        columns={columns}
        isLoading={isLoading}
        isFetching={isFetching || loadingBridgeClients}
        emptyTitle={t('No Tunnel App Requests')}
        emptyDescription={t(
          'Request an MCP code tunnel, HTTP tunnel, or TCP tunnel for a local Bridge client before creating connection keys.'
        )}
        emptyAction={
          <Button onClick={() => setCreateOpen(true)}>
            <Plus className='size-4' />
            {t('New Tunnel App')}
          </Button>
        }
        skeletonKeyPrefix='my-tunnel-apps-skeleton'
        toolbarProps={{
          searchPlaceholder: t(
            'Filter by tunnel name, slug or bridge client...'
          ),
          filters: [
            {
              columnId: 'status',
              title: t('Status'),
              options: [
                { label: t('Pending'), value: 'pending' },
                { label: t('Approved'), value: 'approved' },
                { label: t('Rejected'), value: 'rejected' },
                { label: t('Disabled'), value: 'disabled' },
                { label: t('Archived'), value: 'archived' },
              ],
              singleSelect: true,
            },
            {
              columnId: 'app_type',
              title: t('Type'),
              options: [
                { label: t('MCP Code Tunnel'), value: 'mcp_code' },
                { label: t('HTTP Tunnel'), value: 'http_tunnel' },
                { label: t('TCP Tunnel'), value: 'tcp_tunnel' },
              ],
              singleSelect: true,
            },
          ],
          preActions: (
            <>
              <Button
                type='button'
                variant='outline'
                onClick={() => setAgentSetupOpen(true)}
                disabled={loadingBridgeClients}
              >
                <Cable className='size-4' />
                {t('Local Agent Setup')}
              </Button>
              <Button
                type='button'
                variant='outline'
                onClick={refreshAll}
                disabled={isFetching || loadingBridgeClients}
                className={cn(
                  (isFetching || loadingBridgeClients) && 'opacity-80'
                )}
              >
                <RefreshCw
                  className={cn(
                    'size-4',
                    (isFetching || loadingBridgeClients) && 'animate-spin'
                  )}
                />
                {t('Refresh')}
              </Button>
              <Button type='button' onClick={() => setCreateOpen(true)}>
                <Plus className='size-4' />
                {t('New Tunnel App')}
              </Button>
            </>
          ),
        }}
        afterTable={
          <p className='text-muted-foreground max-w-3xl text-xs leading-5'>
            {t(
              'Approved requests appear in Tunnel Connections, where you can create per-device connection keys.'
            )}
          </p>
        }
      />

      <CreateTunnelAppDialog
        open={createOpen}
        bridgeClients={bridgeClients}
        bridgeClientsLoading={loadingBridgeClients}
        onOpenChange={setCreateOpen}
        onCreated={() => void refetch()}
      />
      <BridgeAgentSetupDialog
        open={agentSetupOpen}
        bridgeClients={bridgeClients}
        bridgeClientsLoading={loadingBridgeClients}
        onOpenChange={setAgentSetupOpen}
        onUpdated={() => void refetchBridgeClients()}
      />
    </>
  )
}
