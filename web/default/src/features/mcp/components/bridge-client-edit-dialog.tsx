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
}

function buildInitialForm(client: BridgeClient | null): BridgeClientForm {
  return {
    name: client?.name ?? '',
    version: client?.version ?? '',
    platform: client?.platform ?? '',
    workspace: client?.workspace ?? '',
    capabilities: (client?.capabilities ?? []).join('\n'),
    status: String(client?.status ?? 0),
  }
}

function parseCapabilities(value: string): string[] {
  return value
    .split(/[\n,]/)
    .map((item) => item.trim())
    .filter(Boolean)
}

export function BridgeClientEditDialog(props: BridgeClientEditDialogProps) {
  const { t } = useTranslation()
  const [form, setForm] = useState<BridgeClientForm>(() =>
    buildInitialForm(props.client)
  )

  return (
    <Dialog open={props.open} onOpenChange={props.onOpenChange}>
      <DialogContent className='max-w-[calc(100%-2rem)] sm:max-w-xl'>
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
