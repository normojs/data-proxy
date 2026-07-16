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
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Package, Pencil, Plus, RefreshCw } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import { Skeleton } from '@/components/ui/skeleton'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { Textarea } from '@/components/ui/textarea'
import { SectionPageLayout } from '@/components/layout'
import {
  createAdminModelTokenPackageSku,
  listAdminModelTokenPackageSkus,
  updateAdminModelTokenPackageSku,
} from './api'
import type { ModelTokenPackageSku, ModelTokenPackageSkuPayload } from './types'

type FormState = {
  name: string
  description: string
  modelsText: string
  totalTokens: string
  inputRatio: string
  outputRatio: string
  cacheRatio: string
  priority: string
  durationDays: string
  priceQuota: string
  status: 'enabled' | 'disabled'
  sortOrder: string
}

const EMPTY_FORM: FormState = {
  name: '',
  description: '',
  modelsText: 'gpt-4o-mini',
  totalTokens: '1000000',
  inputRatio: '1',
  outputRatio: '1',
  cacheRatio: '1',
  priority: '0',
  durationDays: '0',
  priceQuota: '1000',
  status: 'enabled',
  sortOrder: '0',
}

function skuModels(sku: ModelTokenPackageSku): string[] {
  if (sku.models && sku.models.length > 0) return sku.models
  if (!sku.models_json) return []
  try {
    const parsed = JSON.parse(sku.models_json) as string[]
    return Array.isArray(parsed) ? parsed : []
  } catch {
    return []
  }
}

function formatTokens(value: number | undefined) {
  return new Intl.NumberFormat().format(value ?? 0)
}

function formFromSku(sku: ModelTokenPackageSku): FormState {
  return {
    name: sku.name || '',
    description: sku.description || '',
    modelsText: skuModels(sku).join(', '),
    totalTokens: String(sku.total_tokens ?? 0),
    inputRatio: String(sku.input_ratio ?? 1),
    outputRatio: String(sku.output_ratio ?? 1),
    cacheRatio: String(sku.cache_ratio ?? 1),
    priority: String(sku.priority ?? 0),
    durationDays:
      sku.duration_seconds && sku.duration_seconds > 0
        ? String(Math.round(sku.duration_seconds / 86400))
        : '0',
    priceQuota: String(sku.price_quota ?? 0),
    status: sku.status === 'disabled' ? 'disabled' : 'enabled',
    sortOrder: String(sku.sort_order ?? 0),
  }
}

function payloadFromForm(
  form: FormState,
  t: (key: string) => string
): ModelTokenPackageSkuPayload {
  const models = form.modelsText
    .split(/[\n,]/)
    .map((item) => item.trim())
    .filter(Boolean)
  const totalTokens = Math.floor(Number(form.totalTokens))
  const priceQuota = Math.floor(Number(form.priceQuota))
  const durationDays = Math.floor(Number(form.durationDays))
  if (!form.name.trim()) {
    throw new Error(t('Name is required'))
  }
  if (models.length === 0) {
    throw new Error(t('Please enter at least one model'))
  }
  if (!Number.isFinite(totalTokens) || totalTokens <= 0) {
    throw new Error(t('Please enter a valid token amount'))
  }
  if (!Number.isFinite(priceQuota) || priceQuota < 0) {
    throw new Error(t('Please enter a valid price in quota points'))
  }
  if (!Number.isFinite(durationDays) || durationDays < 0) {
    throw new Error(t('Please enter a valid duration in days'))
  }
  return {
    name: form.name.trim(),
    description: form.description.trim() || undefined,
    models,
    total_tokens: totalTokens,
    input_ratio: Number(form.inputRatio) || 1,
    output_ratio: Number(form.outputRatio) || 1,
    cache_ratio: Number(form.cacheRatio) || 1,
    priority: Math.floor(Number(form.priority) || 0),
    duration_seconds: durationDays > 0 ? durationDays * 86400 : 0,
    price_quota: priceQuota,
    status: form.status,
    sort_order: Math.floor(Number(form.sortOrder) || 0),
  }
}

export function PackageSkusPage() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [sheetOpen, setSheetOpen] = useState(false)
  const [editing, setEditing] = useState<ModelTokenPackageSku | null>(null)
  const [form, setForm] = useState<FormState>(EMPTY_FORM)

  const skusQuery = useQuery({
    queryKey: ['admin', 'model-token-package-skus'],
    queryFn: async () => {
      const response = await listAdminModelTokenPackageSkus(false)
      if (!response.success) {
        throw new Error(response.message || t('Failed to load package SKUs'))
      }
      return response.data ?? []
    },
  })

  const skus = useMemo(() => skusQuery.data ?? [], [skusQuery.data])

  useEffect(() => {
    if (!sheetOpen) return
    setForm(editing ? formFromSku(editing) : EMPTY_FORM)
  }, [sheetOpen, editing])

  const saveMutation = useMutation({
    mutationFn: async () => {
      const payload = payloadFromForm(form, t)
      if (editing) {
        const result = await updateAdminModelTokenPackageSku(editing.id, payload)
        if (!result.success) {
          throw new Error(result.message || t('Failed to update package SKU'))
        }
        return result.data
      }
      const result = await createAdminModelTokenPackageSku(payload)
      if (!result.success) {
        throw new Error(result.message || t('Failed to create package SKU'))
      }
      return result.data
    },
    onSuccess: () => {
      toast.success(
        editing ? t('Package SKU updated') : t('Package SKU created')
      )
      setSheetOpen(false)
      setEditing(null)
      queryClient.invalidateQueries({
        queryKey: ['admin', 'model-token-package-skus'],
      })
      queryClient.invalidateQueries({
        queryKey: ['wallet', 'model-token-package-skus'],
      })
    },
    onError: (error) => {
      toast.error(
        error instanceof Error
          ? error.message
          : t('Failed to save package SKU')
      )
    },
  })

  const toggleMutation = useMutation({
    mutationFn: async (sku: ModelTokenPackageSku) => {
      const nextStatus = sku.status === 'enabled' ? 'disabled' : 'enabled'
      const payload = payloadFromForm(formFromSku(sku), t)
      payload.status = nextStatus
      const result = await updateAdminModelTokenPackageSku(sku.id, payload)
      if (!result.success) {
        throw new Error(result.message || t('Failed to update package SKU'))
      }
      return result.data
    },
    onSuccess: () => {
      toast.success(t('Package SKU updated'))
      queryClient.invalidateQueries({
        queryKey: ['admin', 'model-token-package-skus'],
      })
      queryClient.invalidateQueries({
        queryKey: ['wallet', 'model-token-package-skus'],
      })
    },
    onError: (error) => {
      toast.error(
        error instanceof Error
          ? error.message
          : t('Failed to update package SKU')
      )
    },
  })

  return (
    <>
      <SectionPageLayout>
        <SectionPageLayout.Title>
          {t('Package SKUs')}
        </SectionPageLayout.Title>
        <SectionPageLayout.Actions>
          <div className='flex gap-2'>
            <Button
              type='button'
              size='sm'
              variant='outline'
              onClick={() => skusQuery.refetch()}
              disabled={skusQuery.isFetching}
            >
              <RefreshCw className='h-4 w-4' />
              {t('Refresh')}
            </Button>
            <Button
              type='button'
              size='sm'
              onClick={() => {
                setEditing(null)
                setSheetOpen(true)
              }}
            >
              <Plus className='h-4 w-4' />
              {t('Create SKU')}
            </Button>
          </div>
        </SectionPageLayout.Actions>
        <SectionPageLayout.Content>
          <div className='mb-3 flex items-start gap-2 text-sm text-muted-foreground'>
            <Package className='mt-0.5 size-4 shrink-0' />
            <p>
              {t(
                'Sellable model token packages. Users buy them with wallet quota points from the Wallet page.'
              )}
            </p>
          </div>

          {skusQuery.isLoading ? (
            <div className='space-y-2'>
              <Skeleton className='h-10 w-full' />
              <Skeleton className='h-10 w-full' />
              <Skeleton className='h-10 w-full' />
            </div>
          ) : skusQuery.isError ? (
            <div className='text-destructive text-sm'>
              {skusQuery.error instanceof Error
                ? skusQuery.error.message
                : t('Failed to load package SKUs')}
            </div>
          ) : skus.length === 0 ? (
            <div className='rounded-lg border border-dashed p-8 text-center text-sm text-muted-foreground'>
              {t('No package SKUs yet. Create one to let users buy token packs.')}
            </div>
          ) : (
            <div className='overflow-hidden rounded-lg border'>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>{t('Name')}</TableHead>
                    <TableHead>{t('Models')}</TableHead>
                    <TableHead className='text-right'>{t('Tokens')}</TableHead>
                    <TableHead className='text-right'>{t('Price')}</TableHead>
                    <TableHead>{t('Duration')}</TableHead>
                    <TableHead>{t('Status')}</TableHead>
                    <TableHead className='text-right'>{t('Actions')}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {skus.map((sku) => {
                    const models = skuModels(sku)
                    const days =
                      sku.duration_seconds && sku.duration_seconds > 0
                        ? Math.round(sku.duration_seconds / 86400)
                        : 0
                    return (
                      <TableRow key={sku.id}>
                        <TableCell>
                          <div className='font-medium'>{sku.name}</div>
                          {sku.description ? (
                            <div className='text-muted-foreground line-clamp-1 text-xs'>
                              {sku.description}
                            </div>
                          ) : null}
                        </TableCell>
                        <TableCell className='max-w-[16rem] truncate text-xs'>
                          {models.join(', ') || '—'}
                        </TableCell>
                        <TableCell className='text-right font-mono text-xs'>
                          {formatTokens(sku.total_tokens)}
                        </TableCell>
                        <TableCell className='text-right font-mono text-xs'>
                          {formatTokens(sku.price_quota)}
                        </TableCell>
                        <TableCell className='text-xs'>
                          {days > 0
                            ? t('{{count}} days', { count: days })
                            : t('Never expires')}
                        </TableCell>
                        <TableCell>
                          <Badge
                            variant='outline'
                            className={
                              sku.status === 'enabled'
                                ? 'border-emerald-500/40 text-emerald-700'
                                : 'text-muted-foreground'
                            }
                          >
                            {sku.status === 'enabled'
                              ? t('Enabled')
                              : t('Disabled')}
                          </Badge>
                        </TableCell>
                        <TableCell className='text-right'>
                          <div className='flex justify-end gap-1'>
                            <Button
                              type='button'
                              size='sm'
                              variant='ghost'
                              onClick={() => {
                                setEditing(sku)
                                setSheetOpen(true)
                              }}
                            >
                              <Pencil className='h-3.5 w-3.5' />
                              {t('Edit')}
                            </Button>
                            <Button
                              type='button'
                              size='sm'
                              variant='outline'
                              disabled={toggleMutation.isPending}
                              onClick={() => toggleMutation.mutate(sku)}
                            >
                              {sku.status === 'enabled'
                                ? t('Disable')
                                : t('Enable')}
                            </Button>
                          </div>
                        </TableCell>
                      </TableRow>
                    )
                  })}
                </TableBody>
              </Table>
            </div>
          )}
        </SectionPageLayout.Content>
      </SectionPageLayout>

      <Sheet
        open={sheetOpen}
        onOpenChange={(open) => {
          setSheetOpen(open)
          if (!open) setEditing(null)
        }}
      >
        <SheetContent className='flex w-full flex-col sm:max-w-lg'>
          <SheetHeader>
            <SheetTitle>
              {editing ? t('Edit package SKU') : t('Create package SKU')}
            </SheetTitle>
            <SheetDescription>
              {t(
                'Users pay with wallet quota points. Ratios default to 1/1/1.'
              )}
            </SheetDescription>
          </SheetHeader>

          <div className='flex-1 space-y-4 overflow-y-auto px-1 py-2'>
            <div className='space-y-2'>
              <Label htmlFor='sku-name'>{t('Name')}</Label>
              <Input
                id='sku-name'
                value={form.name}
                onChange={(e) => setForm((f) => ({ ...f, name: e.target.value }))}
                placeholder={t('e.g. GPT Mini 1M Pack')}
              />
            </div>
            <div className='space-y-2'>
              <Label htmlFor='sku-desc'>{t('Description')}</Label>
              <Textarea
                id='sku-desc'
                value={form.description}
                onChange={(e) =>
                  setForm((f) => ({ ...f, description: e.target.value }))
                }
                rows={2}
              />
            </div>
            <div className='space-y-2'>
              <Label htmlFor='sku-models'>{t('Models')}</Label>
              <Textarea
                id='sku-models'
                value={form.modelsText}
                onChange={(e) =>
                  setForm((f) => ({ ...f, modelsText: e.target.value }))
                }
                placeholder='gpt-4o-mini, claude-sonnet-4'
                rows={2}
              />
              <p className='text-muted-foreground text-xs'>
                {t('Comma or newline separated model names.')}
              </p>
            </div>
            <div className='grid grid-cols-2 gap-3'>
              <div className='space-y-2'>
                <Label htmlFor='sku-tokens'>{t('Total tokens')}</Label>
                <Input
                  id='sku-tokens'
                  inputMode='numeric'
                  value={form.totalTokens}
                  onChange={(e) =>
                    setForm((f) => ({ ...f, totalTokens: e.target.value }))
                  }
                />
              </div>
              <div className='space-y-2'>
                <Label htmlFor='sku-price'>{t('Price (quota points)')}</Label>
                <Input
                  id='sku-price'
                  inputMode='numeric'
                  value={form.priceQuota}
                  onChange={(e) =>
                    setForm((f) => ({ ...f, priceQuota: e.target.value }))
                  }
                />
              </div>
              <div className='space-y-2'>
                <Label htmlFor='sku-days'>{t('Valid days (0 = never)')}</Label>
                <Input
                  id='sku-days'
                  inputMode='numeric'
                  value={form.durationDays}
                  onChange={(e) =>
                    setForm((f) => ({ ...f, durationDays: e.target.value }))
                  }
                />
              </div>
              <div className='space-y-2'>
                <Label htmlFor='sku-status'>{t('Status')}</Label>
                <Select
                  value={form.status}
                  onValueChange={(value) =>
                    setForm((f) => ({
                      ...f,
                      status: value === 'disabled' ? 'disabled' : 'enabled',
                    }))
                  }
                >
                  <SelectTrigger id='sku-status'>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value='enabled'>{t('Enabled')}</SelectItem>
                    <SelectItem value='disabled'>{t('Disabled')}</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <div className='space-y-2'>
                <Label htmlFor='sku-in'>{t('Input ratio')}</Label>
                <Input
                  id='sku-in'
                  value={form.inputRatio}
                  onChange={(e) =>
                    setForm((f) => ({ ...f, inputRatio: e.target.value }))
                  }
                />
              </div>
              <div className='space-y-2'>
                <Label htmlFor='sku-out'>{t('Output ratio')}</Label>
                <Input
                  id='sku-out'
                  value={form.outputRatio}
                  onChange={(e) =>
                    setForm((f) => ({ ...f, outputRatio: e.target.value }))
                  }
                />
              </div>
              <div className='space-y-2'>
                <Label htmlFor='sku-cache'>{t('Cache ratio')}</Label>
                <Input
                  id='sku-cache'
                  value={form.cacheRatio}
                  onChange={(e) =>
                    setForm((f) => ({ ...f, cacheRatio: e.target.value }))
                  }
                />
              </div>
              <div className='space-y-2'>
                <Label htmlFor='sku-priority'>{t('Priority')}</Label>
                <Input
                  id='sku-priority'
                  value={form.priority}
                  onChange={(e) =>
                    setForm((f) => ({ ...f, priority: e.target.value }))
                  }
                />
              </div>
              <div className='space-y-2'>
                <Label htmlFor='sku-sort'>{t('Sort order')}</Label>
                <Input
                  id='sku-sort'
                  value={form.sortOrder}
                  onChange={(e) =>
                    setForm((f) => ({ ...f, sortOrder: e.target.value }))
                  }
                />
              </div>
            </div>
          </div>

          <SheetFooter className='gap-2 sm:gap-0'>
            <Button
              type='button'
              variant='outline'
              onClick={() => setSheetOpen(false)}
            >
              {t('Cancel')}
            </Button>
            <Button
              type='button'
              disabled={saveMutation.isPending}
              onClick={() => saveMutation.mutate()}
            >
              {saveMutation.isPending ? t('Saving…') : t('Save')}
            </Button>
          </SheetFooter>
        </SheetContent>
      </Sheet>
    </>
  )
}
