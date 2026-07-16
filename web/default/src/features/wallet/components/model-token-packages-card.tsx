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
import { useQuery } from '@tanstack/react-query'
import { Package } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Skeleton } from '@/components/ui/skeleton'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import {
  getSelfModelTokenPackageLedger,
  getSelfModelTokenPackages,
  getSelfModelTokenPackageSkus,
  purchaseModelTokenPackageSku,
  type ModelTokenPackage,
  type ModelTokenPackageSku,
} from '../api'

function packageModels(pkg: ModelTokenPackage): string[] {
  if (pkg.models && pkg.models.length > 0) return pkg.models
  if (!pkg.models_json) return []
  try {
    const parsed = JSON.parse(pkg.models_json) as string[]
    return Array.isArray(parsed) ? parsed : []
  } catch {
    return []
  }
}

function formatTokens(value: number | undefined) {
  return new Intl.NumberFormat().format(value ?? 0)
}

function formatDateTime(value: number | undefined) {
  if (!value) return '—'
  return new Intl.DateTimeFormat(undefined, {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  }).format(new Date(value * 1000))
}

function statusLabel(status: string) {
  switch (status) {
    case 'active':
      return 'Active'
    case 'exhausted':
      return 'Exhausted'
    case 'expired':
      return 'Expired'
    case 'disabled':
      return 'Disabled'
    default:
      return status || 'Unknown'
  }
}

function statusClass(status: string) {
  switch (status) {
    case 'active':
      return 'border-emerald-500/40 text-emerald-700'
    case 'exhausted':
      return 'border-amber-500/40 text-amber-700'
    case 'expired':
    case 'disabled':
      return 'text-muted-foreground'
    default:
      return ''
  }
}

export function ModelTokenPackagesCard() {
  const { t } = useTranslation()
  const [includeInactive, setIncludeInactive] = useState(false)
  const [selectedPackage, setSelectedPackage] =
    useState<ModelTokenPackage | null>(null)
  const [purchasingSkuId, setPurchasingSkuId] = useState<number | null>(null)

  const packagesQuery = useQuery({
    queryKey: ['wallet', 'model-token-packages', includeInactive],
    queryFn: async () => {
      const response = await getSelfModelTokenPackages(includeInactive)
      if (!response.success) {
        throw new Error(response.message || 'Failed to load packages')
      }
      return response.data ?? []
    },
  })

  const skusQuery = useQuery({
    queryKey: ['wallet', 'model-token-package-skus'],
    queryFn: async () => {
      const response = await getSelfModelTokenPackageSkus()
      if (!response.success) {
        throw new Error(response.message || 'Failed to load package SKUs')
      }
      return response.data ?? []
    },
  })

  const packages = useMemo(
    () => packagesQuery.data ?? [],
    [packagesQuery.data]
  )
  const skus = useMemo(() => skusQuery.data ?? [], [skusQuery.data])
  const activeCount = useMemo(
    () => packages.filter((item) => item.status === 'active').length,
    [packages]
  )
  const remainingTotal = useMemo(
    () =>
      packages
        .filter((item) => item.status === 'active')
        .reduce((sum, item) => sum + (item.remaining_tokens || 0), 0),
    [packages]
  )

  async function handlePurchase(sku: ModelTokenPackageSku) {
    if (purchasingSkuId != null) return
    setPurchasingSkuId(sku.id)
    try {
      const response = await purchaseModelTokenPackageSku(sku.id)
      if (!response.success) {
        throw new Error(response.message || 'Purchase failed')
      }
      await Promise.all([packagesQuery.refetch(), skusQuery.refetch()])
    } catch (error) {
      // Keep UI simple: surface the server message via alert for now.
      window.alert(
        error instanceof Error ? error.message : t('Purchase failed')
      )
    } finally {
      setPurchasingSkuId(null)
    }
  }

  return (
    <>
      <div className='overflow-hidden rounded-lg border'>
        <div className='flex flex-wrap items-start justify-between gap-3 border-b px-4 py-3'>
          <div className='min-w-0'>
            <div className='flex items-center gap-2'>
              <Package className='text-muted-foreground size-4' />
              <h3 className='text-sm font-medium'>
                {t('Model Token Packages')}
              </h3>
            </div>
            <p className='text-muted-foreground mt-1 text-xs'>
              {t(
                'LLM token balances for specific models. Used before wallet balance when a package covers the model.'
              )}
            </p>
          </div>
          <div className='flex items-center gap-2'>
            <Button
              type='button'
              variant='outline'
              size='sm'
              onClick={() => setIncludeInactive((value) => !value)}
            >
              {includeInactive
                ? t('Hide inactive')
                : t('Show inactive')}
            </Button>
            <Button
              type='button'
              variant='outline'
              size='sm'
              onClick={() => {
                packagesQuery.refetch()
                skusQuery.refetch()
              }}
              disabled={packagesQuery.isFetching || skusQuery.isFetching}
            >
              {t('Refresh')}
            </Button>
          </div>
        </div>

        <div className='text-muted-foreground grid grid-cols-2 gap-2 border-b px-4 py-2 text-xs sm:grid-cols-3'>
          <div>
            {t('Active packages')}:{' '}
            <span className='text-foreground font-medium'>{activeCount}</span>
          </div>
          <div>
            {t('Remaining tokens')}:{' '}
            <span className='text-foreground font-medium'>
              {formatTokens(remainingTotal)}
            </span>
          </div>
          <div className='col-span-2 sm:col-span-1'>
            {t('Buy with wallet')}:{' '}
            <span className='text-foreground font-medium'>{skus.length}</span>
          </div>
        </div>

        {skus.length > 0 && (
          <div className='border-b px-4 py-3'>
            <div className='mb-2 text-xs font-medium'>{t('Available packages')}</div>
            <div className='grid gap-2 md:grid-cols-2'>
              {skus.map((sku) => {
                const models = packageModels(sku as unknown as ModelTokenPackage)
                return (
                  <div
                    key={sku.id}
                    className='flex items-start justify-between gap-3 rounded-md border px-3 py-2'
                  >
                    <div className='min-w-0'>
                      <div className='truncate text-sm font-medium'>{sku.name}</div>
                      <div className='text-muted-foreground mt-0.5 text-xs'>
                        {formatTokens(sku.total_tokens)} tokens · {t('Price')}:{' '}
                        {formatTokens(sku.price_quota)} {t('quota points')}
                      </div>
                      {models.length > 0 && (
                        <div className='text-muted-foreground mt-1 truncate text-[11px]'>
                          {models.join(', ')}
                        </div>
                      )}
                    </div>
                    <Button
                      type='button'
                      size='sm'
                      disabled={purchasingSkuId != null}
                      onClick={() => handlePurchase(sku)}
                    >
                      {purchasingSkuId === sku.id ? t('Buying…') : t('Buy')}
                    </Button>
                  </div>
                )
              })}
            </div>
          </div>
        )}

        {packagesQuery.isLoading ? (
          <div className='space-y-2 p-4'>
            <Skeleton className='h-8 w-full' />
            <Skeleton className='h-8 w-full' />
            <Skeleton className='h-8 w-full' />
          </div>
        ) : packagesQuery.isError ? (
          <div className='text-destructive p-4 text-sm'>
            {packagesQuery.error instanceof Error
              ? packagesQuery.error.message
              : t('Failed to load model token packages')}
          </div>
        ) : packages.length === 0 ? (
          <div className='text-muted-foreground p-6 text-sm'>
            {t(
              'No model token packages yet. When an admin grants a package, it will appear here.'
            )}
          </div>
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t('Package')}</TableHead>
                <TableHead>{t('Models')}</TableHead>
                <TableHead>{t('Remaining')}</TableHead>
                <TableHead>{t('Ratios')}</TableHead>
                <TableHead>{t('Status')}</TableHead>
                <TableHead className='text-right'>{t('Actions')}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {packages.map((pkg) => {
                const models = packageModels(pkg)
                return (
                  <TableRow key={pkg.id}>
                    <TableCell>
                      <div className='min-w-40'>
                        <div className='truncate font-medium'>
                          {pkg.name || `Package #${pkg.id}`}
                        </div>
                        <div className='text-muted-foreground text-xs'>
                          #{pkg.id}
                          {pkg.priority
                            ? ` · ${t('Priority')} ${pkg.priority}`
                            : ''}
                        </div>
                      </div>
                    </TableCell>
                    <TableCell>
                      <div className='flex max-w-64 flex-wrap gap-1'>
                        {models.length === 0 ? (
                          <span className='text-muted-foreground text-xs'>—</span>
                        ) : (
                          models.slice(0, 4).map((modelName) => (
                            <Badge
                              key={modelName}
                              variant='outline'
                              className='font-mono text-[10px]'
                            >
                              {modelName}
                            </Badge>
                          ))
                        )}
                        {models.length > 4 ? (
                          <Badge variant='outline' className='text-[10px]'>
                            +{models.length - 4}
                          </Badge>
                        ) : null}
                      </div>
                    </TableCell>
                    <TableCell>
                      <div className='font-mono text-xs'>
                        {formatTokens(pkg.remaining_tokens)} /{' '}
                        {formatTokens(pkg.total_tokens)}
                      </div>
                      <div className='text-muted-foreground text-[11px]'>
                        {t('Used')}: {formatTokens(pkg.used_tokens)}
                      </div>
                    </TableCell>
                    <TableCell>
                      <div className='text-muted-foreground font-mono text-[11px]'>
                        in {pkg.input_ratio} · out {pkg.output_ratio} · cache{' '}
                        {pkg.cache_ratio}
                      </div>
                    </TableCell>
                    <TableCell>
                      <Badge
                        variant='outline'
                        className={statusClass(pkg.status)}
                      >
                        {t(statusLabel(pkg.status))}
                      </Badge>
                      {pkg.expired_at > 0 ? (
                        <div className='text-muted-foreground mt-1 text-[11px]'>
                          {t('Expires')}: {formatDateTime(pkg.expired_at)}
                        </div>
                      ) : null}
                    </TableCell>
                    <TableCell className='text-right'>
                      <Button
                        type='button'
                        variant='ghost'
                        size='sm'
                        onClick={() => setSelectedPackage(pkg)}
                      >
                        {t('Ledger')}
                      </Button>
                    </TableCell>
                  </TableRow>
                )
              })}
            </TableBody>
          </Table>
        )}
      </div>

      <ModelTokenPackageLedgerDialog
        open={Boolean(selectedPackage)}
        pkg={selectedPackage}
        onOpenChange={(open) => {
          if (!open) setSelectedPackage(null)
        }}
      />
    </>
  )
}

function ModelTokenPackageLedgerDialog(props: {
  open: boolean
  pkg: ModelTokenPackage | null
  onOpenChange: (open: boolean) => void
}) {
  const { t } = useTranslation()
  const packageId = props.pkg?.id ?? 0

  const ledgerQuery = useQuery({
    queryKey: ['wallet', 'model-token-package-ledger', packageId],
    queryFn: async () => {
      const response = await getSelfModelTokenPackageLedger(packageId, {
        p: 1,
        page_size: 50,
      })
      if (!response.success) {
        throw new Error(response.message || 'Failed to load ledger')
      }
      return response.data
    },
    enabled: props.open && packageId > 0,
  })

  const items = ledgerQuery.data?.items ?? []

  return (
    <Dialog open={props.open} onOpenChange={props.onOpenChange}>
      <DialogContent className='max-h-[calc(100vh-2rem)] overflow-y-auto sm:max-w-3xl'>
        <DialogHeader>
          <DialogTitle>{t('Package Ledger')}</DialogTitle>
          <DialogDescription>
            {props.pkg
              ? `${props.pkg.name || `Package #${props.pkg.id}`} · ${t('Remaining')}: ${formatTokens(props.pkg.remaining_tokens)}`
              : '—'}
          </DialogDescription>
        </DialogHeader>
        {ledgerQuery.isLoading ? (
          <div className='space-y-2'>
            <Skeleton className='h-8 w-full' />
            <Skeleton className='h-8 w-full' />
          </div>
        ) : ledgerQuery.isError ? (
          <div className='text-destructive text-sm'>
            {ledgerQuery.error instanceof Error
              ? ledgerQuery.error.message
              : t('Failed to load ledger')}
          </div>
        ) : items.length === 0 ? (
          <div className='text-muted-foreground text-sm'>
            {t('No ledger entries yet')}
          </div>
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t('Time')}</TableHead>
                <TableHead>{t('Reason')}</TableHead>
                <TableHead>{t('Model')}</TableHead>
                <TableHead className='text-right'>{t('Delta')}</TableHead>
                <TableHead>{t('Request ID')}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {items.map((row) => (
                <TableRow key={row.id}>
                  <TableCell className='text-xs'>
                    {formatDateTime(row.created_at)}
                  </TableCell>
                  <TableCell className='font-mono text-xs'>
                    {row.reason}
                  </TableCell>
                  <TableCell className='font-mono text-xs'>
                    {row.model || '—'}
                  </TableCell>
                  <TableCell className='text-right font-mono text-xs'>
                    {row.delta_tokens > 0 ? '+' : ''}
                    {formatTokens(row.delta_tokens)}
                  </TableCell>
                  <TableCell className='text-muted-foreground max-w-48 truncate font-mono text-[11px]'>
                    {row.request_id || '—'}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        )}
      </DialogContent>
    </Dialog>
  )
}
