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
import { Loader2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
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
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
import { getBridgeClientStatusOptions } from '../constants'
import type { BridgeClient, BridgeClientUpdatePayload } from '../types'

type BridgeClientEditDialogProps = {
  client: BridgeClient | null
  isSaving?: boolean
  open: boolean
  onOpenChange: (open: boolean) => void
  onSubmit: (payload: BridgeClientUpdatePayload) => void
}

type BridgeClientForm = {
  name: string
  version: string
  platform: string
  workspace: string
  capabilities: string
  status: string
  allowedTools: string
  allowWrite: boolean
  maxResultBytes: string
  maxScanFileBytes: string
  maxResults: string
  treeDepth: string
  walkDepth: string
  mcpAllowedTargets: string
}

function buildInitialForm(client: BridgeClient | null): BridgeClientForm {
  return {
    name: client?.name ?? '',
    version: client?.version ?? '',
    platform: client?.platform ?? '',
    workspace: client?.workspace ?? '',
    capabilities: (client?.capabilities ?? []).join('\n'),
    status: String(client?.status ?? 0),
    allowedTools: (client?.policy?.allowed_tools ?? []).join('\n'),
    allowWrite: client?.policy?.allow_write ?? false,
    maxResultBytes: String(client?.policy?.max_result_bytes ?? ''),
    maxScanFileBytes: String(client?.policy?.max_scan_file_bytes ?? ''),
    maxResults: String(client?.policy?.max_results ?? ''),
    treeDepth: String(client?.policy?.tree_depth ?? ''),
    walkDepth: String(client?.policy?.walk_depth ?? ''),
    mcpAllowedTargets: (client?.policy?.mcp_allowed_targets ?? []).join('\n'),
  }
}

function parseCapabilities(value: string): string[] {
  return value
    .split(/[\n,]/)
    .map((item) => item.trim())
    .filter(Boolean)
}

function parseOptionalNumber(value: string): number | undefined {
  const parsed = Number(value)
  if (!Number.isFinite(parsed) || parsed <= 0) return undefined
  return Math.floor(parsed)
}

export function BridgeClientEditDialog(props: BridgeClientEditDialogProps) {
  const { t } = useTranslation()
  const [form, setForm] = useState<BridgeClientForm>(() =>
    buildInitialForm(props.client)
  )

  return (
    <Dialog open={props.open} onOpenChange={props.onOpenChange}>
      <DialogContent className='max-w-[calc(100%-2rem)] sm:max-w-2xl'>
        <DialogHeader>
          <DialogTitle>{t('Edit Bridge Client')}</DialogTitle>
        </DialogHeader>

        <div className='grid gap-3 sm:grid-cols-2'>
          <div className='space-y-1.5'>
            <Label htmlFor='bridge-client-name'>{t('Name')}</Label>
            <Input
              id='bridge-client-name'
              value={form.name}
              onChange={(event) =>
                setForm((current) => ({ ...current, name: event.target.value }))
              }
            />
          </div>
          <div className='space-y-1.5'>
            <Label htmlFor='bridge-client-version'>{t('Version')}</Label>
            <Input
              id='bridge-client-version'
              value={form.version}
              onChange={(event) =>
                setForm((current) => ({
                  ...current,
                  version: event.target.value,
                }))
              }
            />
          </div>
          <div className='space-y-1.5'>
            <Label htmlFor='bridge-client-platform'>{t('Platform')}</Label>
            <Input
              id='bridge-client-platform'
              value={form.platform}
              onChange={(event) =>
                setForm((current) => ({
                  ...current,
                  platform: event.target.value,
                }))
              }
            />
          </div>
          <div className='space-y-1.5'>
            <Label htmlFor='bridge-client-status'>{t('Status')}</Label>
            <NativeSelect
              id='bridge-client-status'
              value={form.status}
              onChange={(event) =>
                setForm((current) => ({
                  ...current,
                  status: event.target.value,
                }))
              }
              className='w-full'
            >
              {getBridgeClientStatusOptions(t).map((option) => (
                <NativeSelectOption key={option.value} value={option.value}>
                  {option.label}
                </NativeSelectOption>
              ))}
            </NativeSelect>
          </div>
          <div className='space-y-1.5 sm:col-span-2'>
            <Label htmlFor='bridge-client-workspace'>{t('Workspace')}</Label>
            <Input
              id='bridge-client-workspace'
              value={form.workspace}
              onChange={(event) =>
                setForm((current) => ({
                  ...current,
                  workspace: event.target.value,
                }))
              }
            />
          </div>
          <div className='space-y-1.5 sm:col-span-2'>
            <Label htmlFor='bridge-client-capabilities'>
              {t('Capabilities')}
            </Label>
            <Textarea
              id='bridge-client-capabilities'
              value={form.capabilities}
              rows={4}
              onChange={(event) =>
                setForm((current) => ({
                  ...current,
                  capabilities: event.target.value,
                }))
              }
            />
          </div>
          <div className='space-y-1.5 sm:col-span-2'>
            <Label htmlFor='bridge-client-policy-tools'>
              {t('Allowed tools')}
            </Label>
            <Textarea
              id='bridge-client-policy-tools'
              value={form.allowedTools}
              rows={3}
              onChange={(event) =>
                setForm((current) => ({
                  ...current,
                  allowedTools: event.target.value,
                }))
              }
            />
          </div>
          <div className='flex items-center gap-2 sm:col-span-2'>
            <Switch
              id='bridge-client-policy-write'
              size='sm'
              checked={form.allowWrite}
              onCheckedChange={(checked) =>
                setForm((current) => ({ ...current, allowWrite: checked }))
              }
            />
            <Label htmlFor='bridge-client-policy-write'>
              {t('Allow write')}
            </Label>
          </div>
          <div className='space-y-1.5'>
            <Label htmlFor='bridge-client-policy-max-result-bytes'>
              {t('Max result bytes')}
            </Label>
            <Input
              id='bridge-client-policy-max-result-bytes'
              inputMode='numeric'
              value={form.maxResultBytes}
              onChange={(event) =>
                setForm((current) => ({
                  ...current,
                  maxResultBytes: event.target.value,
                }))
              }
            />
          </div>
          <div className='space-y-1.5'>
            <Label htmlFor='bridge-client-policy-max-scan-file-bytes'>
              {t('Max scan file bytes')}
            </Label>
            <Input
              id='bridge-client-policy-max-scan-file-bytes'
              inputMode='numeric'
              value={form.maxScanFileBytes}
              onChange={(event) =>
                setForm((current) => ({
                  ...current,
                  maxScanFileBytes: event.target.value,
                }))
              }
            />
          </div>
          <div className='space-y-1.5'>
            <Label htmlFor='bridge-client-policy-max-results'>
              {t('Max results')}
            </Label>
            <Input
              id='bridge-client-policy-max-results'
              inputMode='numeric'
              value={form.maxResults}
              onChange={(event) =>
                setForm((current) => ({
                  ...current,
                  maxResults: event.target.value,
                }))
              }
            />
          </div>
          <div className='space-y-1.5'>
            <Label htmlFor='bridge-client-policy-tree-depth'>
              {t('Tree depth')}
            </Label>
            <Input
              id='bridge-client-policy-tree-depth'
              inputMode='numeric'
              value={form.treeDepth}
              onChange={(event) =>
                setForm((current) => ({
                  ...current,
                  treeDepth: event.target.value,
                }))
              }
            />
          </div>
          <div className='space-y-1.5'>
            <Label htmlFor='bridge-client-policy-walk-depth'>
              {t('Walk depth')}
            </Label>
            <Input
              id='bridge-client-policy-walk-depth'
              inputMode='numeric'
              value={form.walkDepth}
              onChange={(event) =>
                setForm((current) => ({
                  ...current,
                  walkDepth: event.target.value,
                }))
              }
            />
          </div>
          <div className='space-y-1.5 sm:col-span-2'>
            <Label htmlFor='bridge-client-policy-targets'>
              {t('MCP target allowlist')}
            </Label>
            <Textarea
              id='bridge-client-policy-targets'
              value={form.mcpAllowedTargets}
              rows={3}
              onChange={(event) =>
                setForm((current) => ({
                  ...current,
                  mcpAllowedTargets: event.target.value,
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
            disabled={props.isSaving}
          >
            {t('Cancel')}
          </Button>
          <Button
            type='button'
            disabled={props.isSaving || !props.client}
            onClick={() =>
              props.onSubmit({
                name: form.name.trim(),
                version: form.version.trim(),
                platform: form.platform.trim(),
                workspace: form.workspace.trim(),
                capabilities: parseCapabilities(form.capabilities),
                status: Number(form.status),
                policy: {
                  allowed_tools: parseCapabilities(form.allowedTools),
                  allow_write: form.allowWrite,
                  max_result_bytes: parseOptionalNumber(form.maxResultBytes),
                  max_scan_file_bytes: parseOptionalNumber(
                    form.maxScanFileBytes
                  ),
                  max_results: parseOptionalNumber(form.maxResults),
                  tree_depth: parseOptionalNumber(form.treeDepth),
                  walk_depth: parseOptionalNumber(form.walkDepth),
                  mcp_allowed_targets: parseCapabilities(
                    form.mcpAllowedTargets
                  ),
                },
              })
            }
          >
            {props.isSaving && <Loader2 className='size-4 animate-spin' />}
            {t('Save')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
