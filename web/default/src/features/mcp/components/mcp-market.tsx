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
import { useEffect, useMemo, useState, type ReactNode } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { BookOpen, Code2, PlugZap, ReceiptText, Sparkles } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'
import { ScrollArea } from '@/components/ui/scroll-area'
import { StatusBadge } from '@/components/status-badge'
import { listMCPTools, mcpQueryKeys } from '../api'
import {
  MCP_TOOL_STATUS,
  getPriceUnitLabel,
  getToolSourceOptions,
} from '../constants'
import { mcpQueryError, mcpQueryErrorMessage } from '../lib/query-errors'
import type { MCPTool } from '../types'
import { JsonDetailDialog } from './json-detail-dialog'
import { ToolCategoryBadge, ToolSourceBadge } from './table-cells'

type SchemaProperty = {
  type?: string | string[]
  description?: string
  enum?: unknown[]
  default?: unknown
  properties?: Record<string, SchemaProperty>
  items?: SchemaProperty
}

function asRecord(value: unknown): Record<string, unknown> {
  return value && typeof value === 'object'
    ? (value as Record<string, unknown>)
    : {}
}

function schemaProperties(schema: unknown): Record<string, SchemaProperty> {
  const properties = asRecord(schema).properties
  return properties && typeof properties === 'object'
    ? (properties as Record<string, SchemaProperty>)
    : {}
}

function schemaRequired(schema: unknown): string[] {
  const required = asRecord(schema).required
  return Array.isArray(required)
    ? required.filter((item): item is string => typeof item === 'string')
    : []
}

function schemaTypeLabel(
  property: SchemaProperty,
  t: (key: string) => string
) {
  if (Array.isArray(property.type)) {
    return property.type.map((item) => t(item)).join(' | ')
  }
  return t(property.type || (property.enum ? 'enum' : 'value'))
}

function exampleValueForProperty(property: SchemaProperty): unknown {
  if (property.default != null) return property.default
  if (Array.isArray(property.enum) && property.enum.length > 0) {
    return property.enum[0]
  }

  const type = Array.isArray(property.type) ? property.type[0] : property.type
  switch (type) {
    case 'integer':
    case 'number':
      return 0
    case 'boolean':
      return false
    case 'array':
      return []
    case 'object':
      return {}
    case 'string':
    default:
      return '<value>'
  }
}

function buildArgumentsTemplate(schema: unknown): Record<string, unknown> {
  const properties = schemaProperties(schema)
  return Object.fromEntries(
    Object.entries(properties).map(([key, property]) => [
      key,
      exampleValueForProperty(property),
    ])
  )
}

function mockStringValueForProperty(name: string): string {
  const key = name.toLowerCase()
  if (key.includes('file_path')) {
    return '/Users/alice/workspace/data-proxy/README.md'
  }
  if (key === 'path' || key.includes('directory') || key.includes('workdir')) {
    return '/Users/alice/workspace/data-proxy'
  }
  if (key.includes('old_string')) return '旧配置项'
  if (key.includes('new_string')) return '新配置项'
  if (key.includes('content')) return '# Data Proxy\n\n这是用于测试的文件内容。'
  if (key.includes('pattern')) return 'func BuiltinTools'
  if (key.includes('glob')) return '**/*.go'
  if (key.includes('command')) return 'go test ./...'
  if (key.includes('input')) return 'pwd\n'
  if (key.includes('session_id')) return 'shell-demo-001'
  if (key.includes('timezone')) return 'Asia/Shanghai'
  if (key.includes('manager')) return 'bun'
  if (key.includes('package')) return 'zod'
  if (key.includes('json')) return '{"name":"Data Proxy","enabled":true}'
  if (key.includes('url')) return 'https://api.example.com/openapi.json'
  if (key.includes('endpoint')) return 'https://mcp.example.com/message'
  if (key.includes('namespace')) return 'default'
  if (key.includes('token')) return 'sk-data-proxy-demo'
  if (key.includes('request')) return 'req_demo_001'
  if (key.includes('client')) return 'bridge-client-demo'
  if (key.includes('server')) return 'mcp-server-demo'
  if (key.includes('object')) return 'obj_demo_001'
  if (key.includes('label')) return 'Data Proxy 示例'
  if (key.includes('name')) return 'remote_read'
  return 'mock-value'
}

function mockNumberValueForProperty(name: string): number {
  const key = name.toLowerCase()
  if (key.includes('timeout')) return 30000
  if (key.includes('interval')) return 60
  if (key.includes('ttl') || key.includes('expiry')) return 3600
  if (key.includes('offset')) return 1
  if (key.includes('depth')) return 2
  if (key.includes('limit') || key.includes('max')) return 20
  if (key.includes('size') || key.includes('bytes')) return 1024
  if (key.includes('port')) return 443
  return 1
}

function mockArrayValueForProperty(
  name: string,
  property: SchemaProperty
): unknown[] {
  const key = name.toLowerCase()
  if (key.includes('tool')) return ['remote_read', 'server_time']
  if (key.includes('group')) return ['default']
  if (key.includes('target')) return ['workspace:default']
  if (property.items) return [mockValueForProperty(`${name}_item`, property.items)]
  return ['mock-value']
}

function mockValueForProperty(
  name: string,
  property: SchemaProperty
): unknown {
  if (property.default != null) return property.default
  if (Array.isArray(property.enum) && property.enum.length > 0) {
    return property.enum[0]
  }

  const type = Array.isArray(property.type) ? property.type[0] : property.type
  switch (type) {
    case 'integer':
    case 'number':
      return mockNumberValueForProperty(name)
    case 'boolean':
      return !name.toLowerCase().includes('dry')
    case 'array':
      return mockArrayValueForProperty(name, property)
    case 'object': {
      const properties = property.properties ?? {}
      if (Object.keys(properties).length === 0) {
        return { trace_id: 'trace_demo_001', source: 'mock' }
      }
      return Object.fromEntries(
        Object.entries(properties).map(([key, child]) => [
          key,
          mockValueForProperty(key, child),
        ])
      )
    }
    case 'string':
    default:
      return mockStringValueForProperty(name)
  }
}

function buildMockArguments(schema: unknown): Record<string, unknown> {
  const properties = schemaProperties(schema)
  return Object.fromEntries(
    Object.entries(properties).map(([key, property]) => [
      key,
      mockValueForProperty(key, property),
    ])
  )
}

function buildToolCallExample(tool: MCPTool | null): Record<string, unknown> {
  return {
    jsonrpc: '2.0',
    id: 'request-1',
    method: 'tools/call',
    params: {
      name: tool?.name ?? '',
      arguments: buildArgumentsTemplate(tool?.input_schema),
    },
  }
}

function buildToolCallMockExample(tool: MCPTool | null): Record<string, unknown> {
  return {
    jsonrpc: '2.0',
    id: 'request-mock-1',
    method: 'tools/call',
    params: {
      name: tool?.name ?? '',
      arguments: buildMockArguments(tool?.input_schema),
    },
  }
}

function getCapabilityBadges(tool: MCPTool) {
  const badges = []
  if (tool.source === 'mcp_proxy') {
    badges.push({ labelKey: 'Remote MCP Proxy', value: 'mcp_proxy' })
  } else if (tool.source === 'openapi') {
    badges.push({ labelKey: 'OpenAPI Import', value: 'openapi' })
  } else if (tool.is_remote) {
    badges.push({ labelKey: 'QidianBrowser Bridge', value: 'bridge' })
  } else if (tool.source === 'builtin') {
    badges.push({ labelKey: 'Server-side Tool', value: 'server' })
  } else if (tool.source === 'custom') {
    badges.push({ labelKey: 'Custom Tool', value: 'custom' })
  }

  if (tool.is_remote) {
    badges.push({ labelKey: 'Local Workspace', value: 'local_workspace' })
  } else if (tool.source !== 'mcp_proxy') {
    badges.push({ labelKey: 'No Local Client', value: 'hosted' })
  }

  return badges
}

function MarketToolRow(props: {
  active: boolean
  tool: MCPTool
  onSelect: (tool: MCPTool) => void
}) {
  const { t } = useTranslation()
  const title = props.tool.display_name || props.tool.name

  return (
    <button
      type='button'
      className={cn(
        'border-input hover:bg-muted/40 flex w-full min-w-0 flex-col gap-2 rounded-lg border px-3 py-2.5 text-left transition-colors',
        props.active && 'border-primary bg-primary/5'
      )}
      onClick={() => props.onSelect(props.tool)}
    >
      <span className='flex min-w-0 items-start justify-between gap-3'>
        <span className='min-w-0'>
          <span className='block truncate font-medium'>{title}</span>
          <span className='text-muted-foreground block truncate font-mono text-xs'>
            {props.tool.name}
          </span>
        </span>
        <ToolSourceBadge source={props.tool.source} />
      </span>
      <span className='flex flex-wrap items-center gap-2 text-xs'>
        <ToolCategoryBadge category={props.tool.category} />
        <span className='text-muted-foreground tabular-nums'>
          {props.tool.price_per_call.toFixed(4)}{' '}
          {t(getPriceUnitLabel(props.tool.price_unit))}
        </span>
        {props.tool.free_quota > 0 && (
          <span className='text-muted-foreground tabular-nums'>
            {t('Daily Free Quota')}: {props.tool.free_quota.toLocaleString()}
          </span>
        )}
      </span>
    </button>
  )
}

function MarketToolDetail(props: {
  tool: MCPTool | null
  onOpenLedger: (tool: MCPTool) => void
  onViewSchema: (tool: MCPTool) => void
  onViewExample: (tool: MCPTool) => void
  onViewMockExample: (tool: MCPTool) => void
}) {
  const { t } = useTranslation()
  const tool = props.tool
  if (!tool) {
    return (
      <div className='border-input text-muted-foreground flex min-h-60 items-center justify-center rounded-lg border text-sm'>
        {t('No MCP tool selected')}
      </div>
    )
  }

  return (
    <div className='border-input rounded-lg border'>
      <div className='border-b px-4 py-3'>
        <div className='flex min-w-0 flex-wrap items-center gap-2'>
          <h2 className='truncate text-base font-semibold'>
            {tool.display_name || tool.name}
          </h2>
          <ToolSourceBadge source={tool.source} />
        </div>
        <div className='text-muted-foreground mt-1 truncate font-mono text-xs'>
          {tool.name}
        </div>
      </div>

      <div className='grid gap-4 p-4'>
        <div className='flex flex-wrap gap-1.5'>
          {getCapabilityBadges(tool).map((badge) => (
            <StatusBadge
              key={badge.value}
              label={t(badge.labelKey)}
              autoColor={badge.value}
              copyable={false}
            />
          ))}
        </div>

        <div className='grid gap-3 sm:grid-cols-2'>
          <DetailField
            label={t('Category')}
            value={<ToolCategoryBadge category={tool.category} />}
          />
          <DetailField
            label={t('Source')}
            value={t(getCapabilityBadges(tool)[0]?.labelKey || tool.source)}
          />
          <DetailField
            label={t('Price')}
            value={`${tool.price_per_call.toFixed(4)} ${t(
              getPriceUnitLabel(tool.price_unit)
            )}`}
          />
          <DetailField
            label={t('Daily Free Quota')}
            value={tool.free_quota.toLocaleString()}
          />
          <DetailField
            label={t('Updated At')}
            value={String(tool.updated_at)}
          />
        </div>

        {tool.description && (
          <div className='space-y-1.5'>
            <div className='text-muted-foreground text-xs font-medium'>
              {t('Tool Introduction')}
            </div>
            <p className='text-sm leading-6'>{tool.description}</p>
          </div>
        )}

        <SchemaFieldList schema={tool.input_schema} />

        <div className='flex flex-wrap gap-2'>
          <Button
            type='button'
            variant='outline'
            onClick={() => props.onViewSchema(tool)}
          >
            <Code2 className='size-4' />
            {t('Input Schema')}
          </Button>
          <Button
            type='button'
            variant='outline'
            onClick={() => props.onViewExample(tool)}
          >
            <PlugZap className='size-4' />
            {t('Parameter Template')}
          </Button>
          <Button
            type='button'
            variant='outline'
            onClick={() => props.onViewMockExample(tool)}
          >
            <Sparkles className='size-4' />
            {t('Mock Example')}
          </Button>
          <Button
            type='button'
            variant='outline'
            onClick={() => props.onOpenLedger(tool)}
          >
            <ReceiptText className='size-4' />
            {t('Open Ledger')}
          </Button>
        </div>
      </div>
    </div>
  )
}

function DetailField(props: { label: string; value: ReactNode }) {
  return (
    <div className='min-w-0'>
      <div className='text-muted-foreground text-xs'>{props.label}</div>
      <div className='min-w-0 truncate text-sm font-medium'>{props.value}</div>
    </div>
  )
}

function SchemaFieldList(props: { schema: unknown }) {
  const { t } = useTranslation()
  const properties = schemaProperties(props.schema)
  const required = new Set(schemaRequired(props.schema))
  const entries = Object.entries(properties).slice(0, 8)

  if (entries.length === 0) return null

  return (
    <div className='space-y-2'>
      <div className='text-muted-foreground flex items-center gap-1.5 text-xs font-medium'>
        <BookOpen className='size-3.5' />
        {t('Parameters')}
      </div>
      <div className='grid gap-2 sm:grid-cols-2'>
        {entries.map(([name, property]) => (
          <div
            key={name}
            className='border-input min-w-0 rounded-md border p-2'
          >
            <div className='flex min-w-0 items-center gap-1.5'>
              <code className='truncate font-mono text-xs font-medium'>
                {name}
              </code>
              <StatusBadge
                label={schemaTypeLabel(property, t)}
                variant='neutral'
                copyable={false}
              />
              {required.has(name) && (
                <StatusBadge
                  label={t('Required')}
                  variant='warning'
                  copyable={false}
                />
              )}
            </div>
            {property.description && (
              <p className='text-muted-foreground mt-1 line-clamp-2 text-xs leading-5'>
                {property.description}
              </p>
            )}
          </div>
        ))}
      </div>
    </div>
  )
}

export function MCPMarket() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const [keyword, setKeyword] = useState('')
  const [source, setSource] = useState('')
  const [selectedToolId, setSelectedToolId] = useState<number | null>(null)
  const [schemaTool, setSchemaTool] = useState<MCPTool | null>(null)
  const [exampleTool, setExampleTool] = useState<MCPTool | null>(null)
  const [mockExampleTool, setMockExampleTool] = useState<MCPTool | null>(null)

  const requestParams = {
    p: 1,
    page_size: 100,
    keyword,
    source,
    status: MCP_TOOL_STATUS.ENABLED,
  }

  const {
    data,
    error: toolsError,
    isError,
    isLoading,
  } = useQuery({
    queryKey: mcpQueryKeys.toolsList(requestParams),
    queryFn: async () => {
      const result = await listMCPTools(requestParams)
      if (!result.success) {
        throw mcpQueryError(result.message, 'Failed to load MCP tools')
      }
      return result.data?.items ?? []
    },
  })

  useEffect(() => {
    if (!isError) return
    toast.error(mcpQueryErrorMessage(toolsError, t('Failed to load MCP tools')))
  }, [isError, t, toolsError])

  const tools = useMemo(() => data ?? [], [data])
  const selectedTool = useMemo(() => {
    if (tools.length === 0) return null
    return tools.find((tool) => tool.id === selectedToolId) ?? tools[0]
  }, [selectedToolId, tools])

  return (
    <>
      <div className='space-y-4'>
        <div className='flex flex-col gap-2 sm:flex-row'>
          <Input
            value={keyword}
            onChange={(event) => setKeyword(event.target.value)}
            placeholder={t('Filter by tool name or description...')}
            className='sm:max-w-sm'
          />
          <NativeSelect
            value={source}
            onChange={(event) => setSource(event.target.value)}
            className='sm:w-52'
          >
            <NativeSelectOption value=''>{t('All Sources')}</NativeSelectOption>
            {getToolSourceOptions(t).map((option) => (
              <NativeSelectOption key={option.value} value={option.value}>
                {option.label}
              </NativeSelectOption>
            ))}
          </NativeSelect>
        </div>

        <div className='grid gap-4 lg:grid-cols-[minmax(260px,380px)_1fr]'>
          <ScrollArea className='max-h-[70vh]'>
            <div className='grid gap-2 pr-2'>
              {isLoading ? (
                Array.from({ length: 6 }).map((_, index) => (
                  <div
                    key={index}
                    className='bg-muted/40 h-20 animate-pulse rounded-lg'
                  />
                ))
              ) : tools.length === 0 ? (
                <div className='border-input text-muted-foreground rounded-lg border px-3 py-8 text-center text-sm'>
                  {t(
                    'No MCP tools available. Seed built-in tools or adjust your filters.'
                  )}
                </div>
              ) : (
                tools.map((tool) => (
                  <MarketToolRow
                    key={tool.id}
                    tool={tool}
                    active={selectedTool?.id === tool.id}
                    onSelect={(item) => setSelectedToolId(item.id)}
                  />
                ))
              )}
            </div>
          </ScrollArea>

          <MarketToolDetail
            tool={selectedTool}
            onOpenLedger={(tool) => {
              void navigate({
                to: '/wallet',
                search: (prev) => ({
                  ...prev,
                  ledgerPage: undefined,
                  ledgerFilter: tool.name,
                  ledgerSourceKind: ['mcp_tool_call'],
                }),
              })
            }}
            onViewSchema={setSchemaTool}
            onViewExample={setExampleTool}
            onViewMockExample={setMockExampleTool}
          />
        </div>
      </div>

      <JsonDetailDialog
        open={schemaTool != null}
        title={t('Input Schema')}
        description={schemaTool?.name}
        value={schemaTool?.input_schema}
        onOpenChange={(open) => !open && setSchemaTool(null)}
      />
      <JsonDetailDialog
        open={exampleTool != null}
        title={t('Parameter Template')}
        description={exampleTool?.name}
        value={buildToolCallExample(exampleTool)}
        onOpenChange={(open) => !open && setExampleTool(null)}
      />
      <JsonDetailDialog
        open={mockExampleTool != null}
        title={t('Mock Example')}
        description={t('Realistic mock data based on this tool schema.')}
        value={buildToolCallMockExample(mockExampleTool)}
        onOpenChange={(open) => !open && setMockExampleTool(null)}
      />
    </>
  )
}
