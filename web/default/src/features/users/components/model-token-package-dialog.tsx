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
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Badge } from '@/components/ui/badge'
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
import { Skeleton } from '@/components/ui/skeleton'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Textarea } from '@/components/ui/textarea'
import {
  disableUserModelTokenPackage,
  grantUserModelTokenPackage,
  listUserModelTokenPackages,
  type ModelTokenPackage,
} from '../api'

type ModelTokenPackageDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  userId: number
  username: string
  onSuccess?: () => void
}

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

export function ModelTokenPackageDialog(props: ModelTokenPackageDialogProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [tab, setTab] = useState('packages')
  const [name, setName] = useState('')
  const [modelsText, setModelsText] = useState('gpt-4o')
  const [totalTokens, setTotalTokens] = useState('1000000')
  const [inputRatio, setInputRatio] = useState('1')
  const [outputRatio, setOutputRatio] = useState('1')
  const [cacheRatio, setCacheRatio] = useState('1')
  const [priority, setPriority] = useState('0')
  const [remark, setRemark] = useState('')

  const packagesQuery = useQuery({
    queryKey: ['admin', 'user-model-token-packages', props.userId],
    queryFn: async () => {
      const response = await listUserModelTokenPackages(props.userId, true)
      if (!response.success) {
        throw new Error(response.message || 'Failed to load packages')
      }
      return response.data ?? []
    },
    enabled: props.open && props.userId > 0,
  })

  const packages = useMemo(
    () => packagesQuery.data ?? [],
    [packagesQuery.data]
  )

  const resetForm = () => {
    setName('')
    setModelsText('gpt-4o')
    setTotalTokens('1000000')
    setInputRatio('1')
    setOutputRatio('1')
    setCacheRatio('1')
    setPriority('0')
    setRemark('')
  }

  const invalidate = () => {
    queryClient.invalidateQueries({
      queryKey: ['admin', 'user-model-token-packages', props.userId],
    })
    props.onSuccess?.()
  }

  const grantMutation = useMutation({
    mutationFn: async () => {
      const models = modelsText
        .split(/[\n,]/)
        .map((item) => item.trim())
        .filter(Boolean)
      const tokens = Number(totalTokens)
      if (models.length === 0) {
        throw new Error(t('Please enter at least one model'))
      }
      if (!Number.isFinite(tokens) || tokens <= 0) {
        throw new Error(t('Please enter a valid token amount'))
      }
      const result = await grantUserModelTokenPackage(props.userId, {
        name: name.trim() || undefined,
        models,
        total_tokens: Math.floor(tokens),
        input_ratio: Number(inputRatio) || 1,
        output_ratio: Number(outputRatio) || 1,
        cache_ratio: Number(cacheRatio) || 0,
        priority: Number(priority) || 0,
        expired_at: 0,
        remark: remark.trim() || undefined,
      })
      if (!result.success) {
        throw new Error(
          result.message || t('Failed to grant model token package')
        )
      }
      return result.data
    },
    onSuccess: () => {
      toast.success(t('Model token package granted'))
      resetForm()
      setTab('packages')
      invalidate()
    },
    onError: (error) => {
      toast.error(
        error instanceof Error
          ? error.message
          : t('Failed to grant model token package')
      )
    },
  })

  const disableMutation = useMutation({
    mutationFn: async (packageId: number) => {
      const result = await disableUserModelTokenPackage(
        props.userId,
        packageId
      )
      if (!result.success) {
        throw new Error(result.message || t('Failed to disable package'))
      }
      return result.data
    },
    onSuccess: () => {
      toast.success(t('Package disabled'))
      invalidate()
    },
    onError: (error) => {
      toast.error(
        error instanceof Error
          ? error.message
          : t('Failed to disable package')
      )
    },
  })

  return (
    <Dialog
      open={props.open}
      onOpenChange={(open) => {
        if (!open) {
          resetForm()
          setTab('packages')
        }
        props.onOpenChange(open)
      }}
    >
      <DialogContent className='max-h-[calc(100vh-2rem)] overflow-y-auto sm:max-w-3xl'>
        <DialogHeader>
          <DialogTitle>{t('Model Token Packages')}</DialogTitle>
          <DialogDescription>
            {t(
              'Manage model token packages for {{username}}. Package tokens are LLM token counts, not wallet balance.',
              { username: props.username }
            )}
          </DialogDescription>
        </DialogHeader>

        <Tabs value={tab} onValueChange={setTab}>
          <TabsList>
            <TabsTrigger value='packages'>{t('Packages')}</TabsTrigger>
            <TabsTrigger value='grant'>{t('Grant')}</TabsTrigger>
          </TabsList>

          <TabsContent value='packages' className='mt-3'>
            {packagesQuery.isLoading ? (
              <div className='space-y-2'>
                <Skeleton className='h-8 w-full' />
                <Skeleton className='h-8 w-full' />
              </div>
            ) : packagesQuery.isError ? (
              <div className='text-destructive text-sm'>
                {packagesQuery.error instanceof Error
                  ? packagesQuery.error.message
                  : t('Failed to load model token packages')}
              </div>
            ) : packages.length === 0 ? (
              <div className='text-muted-foreground rounded-lg border p-6 text-sm'>
                {t('No model token packages yet. Grant one from the Grant tab.')}
              </div>
            ) : (
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>{t('Package')}</TableHead>
                    <TableHead>{t('Models')}</TableHead>
                    <TableHead>{t('Remaining')}</TableHead>
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
                          <div className='min-w-36'>
                            <div className='truncate font-medium'>
                              {pkg.name || `Package #${pkg.id}`}
                            </div>
                            <div className='text-muted-foreground text-xs'>
                              #{pkg.id}
                            </div>
                          </div>
                        </TableCell>
                        <TableCell>
                          <div className='flex max-w-56 flex-wrap gap-1'>
                            {models.slice(0, 3).map((modelName) => (
                              <Badge
                                key={modelName}
                                variant='outline'
                                className='font-mono text-[10px]'
                              >
                                {modelName}
                              </Badge>
                            ))}
                            {models.length > 3 ? (
                              <Badge variant='outline' className='text-[10px]'>
                                +{models.length - 3}
                              </Badge>
                            ) : null}
                          </div>
                        </TableCell>
                        <TableCell className='font-mono text-xs'>
                          {formatTokens(pkg.remaining_tokens)} /{' '}
                          {formatTokens(pkg.total_tokens)}
                        </TableCell>
                        <TableCell>
                          <Badge variant='outline'>
                            {t(statusLabel(pkg.status))}
                          </Badge>
                        </TableCell>
                        <TableCell className='text-right'>
                          <Button
                            type='button'
                            variant='ghost'
                            size='sm'
                            disabled={
                              pkg.status === 'disabled' ||
                              disableMutation.isPending
                            }
                            onClick={() => {
                              if (
                                window.confirm(
                                  t(
                                    'Disable this package? It will stop being used for new requests.'
                                  )
                                )
                              ) {
                                disableMutation.mutate(pkg.id)
                              }
                            }}
                          >
                            {t('Disable')}
                          </Button>
                        </TableCell>
                      </TableRow>
                    )
                  })}
                </TableBody>
              </Table>
            )}
          </TabsContent>

          <TabsContent value='grant' className='mt-3'>
            <div className='grid gap-3'>
              <div className='grid gap-1.5'>
                <Label>{t('Name')}</Label>
                <Input
                  value={name}
                  onChange={(event) => setName(event.target.value)}
                  placeholder={t('Optional package name')}
                />
              </div>
              <div className='grid gap-1.5'>
                <Label>{t('Models')}</Label>
                <Textarea
                  value={modelsText}
                  onChange={(event) => setModelsText(event.target.value)}
                  placeholder={t('One model per line, or comma-separated')}
                  className='min-h-20 font-mono text-xs'
                />
              </div>
              <div className='grid gap-1.5'>
                <Label>{t('Total Tokens')}</Label>
                <Input
                  type='number'
                  min={1}
                  value={totalTokens}
                  onChange={(event) => setTotalTokens(event.target.value)}
                />
              </div>
              <div className='grid grid-cols-3 gap-2'>
                <div className='grid gap-1.5'>
                  <Label>{t('Input Ratio')}</Label>
                  <Input
                    type='number'
                    min={0}
                    max={10}
                    step='0.1'
                    value={inputRatio}
                    onChange={(event) => setInputRatio(event.target.value)}
                  />
                </div>
                <div className='grid gap-1.5'>
                  <Label>{t('Output Ratio')}</Label>
                  <Input
                    type='number'
                    min={0}
                    max={10}
                    step='0.1'
                    value={outputRatio}
                    onChange={(event) => setOutputRatio(event.target.value)}
                  />
                </div>
                <div className='grid gap-1.5'>
                  <Label>{t('Cache Ratio')}</Label>
                  <Input
                    type='number'
                    min={0}
                    max={10}
                    step='0.1'
                    value={cacheRatio}
                    onChange={(event) => setCacheRatio(event.target.value)}
                  />
                </div>
              </div>
              <div className='grid gap-1.5'>
                <Label>{t('Priority')}</Label>
                <Input
                  type='number'
                  value={priority}
                  onChange={(event) => setPriority(event.target.value)}
                />
              </div>
              <div className='grid gap-1.5'>
                <Label>{t('Remark')}</Label>
                <Textarea
                  value={remark}
                  onChange={(event) => setRemark(event.target.value)}
                  className='min-h-16'
                />
              </div>
              <DialogFooter>
                <Button
                  type='button'
                  onClick={() => grantMutation.mutate()}
                  disabled={grantMutation.isPending}
                >
                  {t('Grant')}
                </Button>
              </DialogFooter>
            </div>
          </TabsContent>
        </Tabs>
      </DialogContent>
    </Dialog>
  )
}
