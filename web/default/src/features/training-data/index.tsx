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
import { useEffect, useMemo, useState, type FormEvent } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import {
  Check,
  Database,
  Download,
  Eye,
  Filter,
  ListFilter,
  RefreshCw,
  X,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { formatTimestampToDate } from '@/lib/format'
import { cn } from '@/lib/utils'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Empty, EmptyDescription, EmptyTitle } from '@/components/ui/empty'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { NativeSelect } from '@/components/ui/native-select'
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
import { StatusBadge, type StatusVariant } from '@/components/status-badge'
import {
  approveTrainingSample,
  buildTrainingDataset,
  getTrainingDatasetExportUrl,
  getTrainingDatasets,
  getTrainingSamplePreview,
  getTrainingSamples,
  rejectTrainingSample,
} from './api'
import type {
  ApiResponse,
  BuildTrainingDatasetPayload,
  BuildTrainingDatasetResponse,
  TrainingSample,
  TrainingSamplePreview,
} from './types'

const datasetStatusVariant: Record<string, StatusVariant> = {
  built: 'success',
  completed: 'success',
  building: 'warning',
  failed: 'danger',
  expired: 'neutral',
  deleted: 'neutral',
}

const reviewStatusVariant: Record<string, StatusVariant> = {
  approved: 'success',
  rejected: 'danger',
  pending: 'warning',
}

const redactionStatusVariant: Record<string, StatusVariant> = {
  basic: 'success',
  redacted: 'success',
  sanitized: 'success',
  pending: 'warning',
  failed: 'danger',
}

const datasetStatusLabel: Record<string, string> = {
  built: 'Built',
  completed: 'Built',
  building: 'Building',
  deleted: 'Removed',
  expired: 'Expired',
  failed: 'Failed',
  pending: 'Pending',
}

const reviewStatusLabel: Record<string, string> = {
  approved: 'Approved',
  pending: 'Pending',
  rejected: 'Rejected',
}

const redactionStatusLabel: Record<string, string> = {
  basic: 'Redacted',
  failed: 'Failed',
  pending: 'Pending',
  redacted: 'Redacted',
  sanitized: 'Redacted',
}

function displayLabel(
  value: string | undefined,
  labels: Record<string, string>
): string {
  if (!value) return 'Unknown'
  return labels[value] ?? value
}

function formatBytes(value: number | undefined): string {
  if (!value || value <= 0) return '-'
  const units = ['B', 'KiB', 'MiB', 'GiB', 'TiB']
  let size = value
  let unitIndex = 0
  while (size >= 1024 && unitIndex < units.length - 1) {
    size /= 1024
    unitIndex += 1
  }
  return `${size >= 10 ? size.toFixed(0) : size.toFixed(1)} ${units[unitIndex]}`
}

function formatCount(value: number | undefined): string {
  return typeof value === 'number' ? value.toLocaleString() : '-'
}

function safeJson(value: unknown): string {
  try {
    return JSON.stringify(value, null, 2)
  } catch {
    return String(value)
  }
}

function qualityLabel(value: number | undefined): string {
  if (typeof value !== 'number' || Number.isNaN(value)) return '-'
  return value.toFixed(3)
}

function assertApiData<T>(response: ApiResponse<T>, fallback: string): T {
  if (!response.success || !response.data) {
    throw new Error(response.message || fallback)
  }
  return response.data
}

function DatasetSkeleton() {
  return (
    <div className='space-y-2 rounded-lg border p-3'>
      {Array.from({ length: 5 }).map((_, index) => (
        <Skeleton key={index} className='h-9 w-full rounded-md' />
      ))}
    </div>
  )
}

function SectionPanel(props: React.ComponentProps<'section'>) {
  return (
    <section
      {...props}
      className={cn('bg-card rounded-lg border p-3', props.className)}
    />
  )
}

export function TrainingData() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [datasetPage, setDatasetPage] = useState(1)
  const [samplePage, setSamplePage] = useState(1)
  const [datasetName, setDatasetName] = useState('')
  const [datasetStatus, setDatasetStatus] = useState('')
  const [selectedDatasetId, setSelectedDatasetId] = useState<number>()
  const [buildForm, setBuildForm] = useState({
    name: '',
    version: '',
    limit: '200',
    maxDecodedBundleBytes: '',
    includeErrored: false,
  })
  const [sampleFilters, setSampleFilters] = useState({
    requestId: '',
    modelName: '',
    minQualityScore: '',
    reviewStatus: 'pending',
  })
  const [previewSampleId, setPreviewSampleId] = useState<number | null>(null)
  const [reviewComment, setReviewComment] = useState('')

  const datasetsQuery = useQuery({
    queryKey: [
      'training-data',
      'datasets',
      datasetPage,
      datasetName,
      datasetStatus,
    ],
    queryFn: () =>
      getTrainingDatasets({
        p: datasetPage,
        page_size: 10,
        name: datasetName,
        status: datasetStatus,
      }),
  })

  const datasets = datasetsQuery.data?.data?.items ?? []
  const datasetTotal = datasetsQuery.data?.data?.total ?? 0
  const selectedDataset = useMemo(
    () => datasets.find((dataset) => dataset.id === selectedDatasetId),
    [datasets, selectedDatasetId]
  )

  useEffect(() => {
    if (!selectedDatasetId && datasets.length > 0) {
      setSelectedDatasetId(datasets[0].id)
    }
  }, [datasets, selectedDatasetId])

  const samplesQuery = useQuery({
    queryKey: [
      'training-data',
      'samples',
      selectedDatasetId,
      samplePage,
      sampleFilters,
    ],
    queryFn: () =>
      getTrainingSamples({
        p: samplePage,
        page_size: 20,
        dataset_version_id: selectedDatasetId,
        request_id: sampleFilters.requestId,
        model_name: sampleFilters.modelName,
        min_quality_score: sampleFilters.minQualityScore
          ? Number(sampleFilters.minQualityScore)
          : undefined,
        review_status: sampleFilters.reviewStatus,
      }),
    enabled: Boolean(selectedDatasetId),
  })

  const samples = samplesQuery.data?.data?.items ?? []
  const sampleTotal = samplesQuery.data?.data?.total ?? 0

  const previewQuery = useQuery({
    queryKey: ['training-data', 'sample-preview', previewSampleId],
    queryFn: () => getTrainingSamplePreview(previewSampleId as number),
    enabled: previewSampleId !== null,
  })

  const preview = previewQuery.data?.data

  const buildMutation = useMutation({
    mutationFn: async (payload: BuildTrainingDatasetPayload) =>
      assertApiData<BuildTrainingDatasetResponse>(
        await buildTrainingDataset(payload),
        t('Failed to build training dataset')
      ),
    onSuccess: (result) => {
      toast.success(t('Training dataset built'))
      setSelectedDatasetId(result.dataset.id)
      setDatasetPage(1)
      void queryClient.invalidateQueries({
        queryKey: ['training-data', 'datasets'],
      })
      void queryClient.invalidateQueries({
        queryKey: ['training-data', 'samples'],
      })
    },
    onError: (error) => {
      toast.error(
        error instanceof Error
          ? error.message
          : t('Failed to build training dataset')
      )
    },
  })

  const reviewMutation = useMutation({
    mutationFn: async (payload: {
      action: 'approve' | 'reject'
      sampleId: number
      comment: string
    }) => {
      const response =
        payload.action === 'approve'
          ? await approveTrainingSample(payload.sampleId, payload.comment)
          : await rejectTrainingSample(payload.sampleId, payload.comment)
      return assertApiData<{ sample: TrainingSample }>(
        response,
        payload.action === 'approve'
          ? t('Failed to approve sample')
          : t('Failed to reject sample')
      )
    },
    onSuccess: (_result, variables) => {
      toast.success(
        variables.action === 'approve'
          ? t('Sample approved')
          : t('Sample rejected')
      )
      setReviewComment('')
      void queryClient.invalidateQueries({
        queryKey: ['training-data', 'samples'],
      })
      void queryClient.invalidateQueries({
        queryKey: ['training-data', 'sample-preview', previewSampleId],
      })
    },
    onError: (error, variables) => {
      toast.error(
        error instanceof Error
          ? error.message
          : variables.action === 'approve'
            ? t('Failed to approve sample')
            : t('Failed to reject sample')
      )
    },
  })

  const handleBuild = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    const payload: BuildTrainingDatasetPayload = {
      name: buildForm.name || undefined,
      version: buildForm.version || undefined,
      limit: buildForm.limit ? Number(buildForm.limit) : undefined,
      max_decoded_bundle_bytes: buildForm.maxDecodedBundleBytes
        ? Number(buildForm.maxDecodedBundleBytes)
        : undefined,
      include_errored: buildForm.includeErrored,
    }
    buildMutation.mutate(payload)
  }

  const handleOpenPreview = (sample: TrainingSample) => {
    setPreviewSampleId(sample.id)
    setReviewComment(sample.review_comment || '')
  }

  const selectedExportUrl = selectedDatasetId
    ? getTrainingDatasetExportUrl(selectedDatasetId)
    : ''

  return (
    <>
      <SectionPageLayout>
        <SectionPageLayout.Title>{t('Training Data')}</SectionPageLayout.Title>
        <SectionPageLayout.Actions>
          <Button
            variant='outline'
            onClick={() => {
              void queryClient.invalidateQueries({
                queryKey: ['training-data'],
              })
            }}
          >
            <RefreshCw className='size-4' aria-hidden />
            {t('Refresh')}
          </Button>
        </SectionPageLayout.Actions>
        <SectionPageLayout.Content>
          <div className='space-y-4'>
            <Alert>
              <Database className='size-4' aria-hidden />
              <AlertTitle>{t('Private training corpus workflow')}</AlertTitle>
              <AlertDescription>
                {t(
                  'Raw capture bundles stay private. Export only returns approved redacted samples from the selected dataset version.'
                )}
              </AlertDescription>
            </Alert>

            <div className='grid gap-4 xl:grid-cols-[minmax(280px,360px)_1fr]'>
              <SectionPanel>
                <div className='mb-3 space-y-1'>
                  <h3 className='text-sm font-semibold'>
                    {t('Build dataset')}
                  </h3>
                  <p className='text-muted-foreground text-xs'>
                    {t(
                      'Create a versioned shard from captured request bundles.'
                    )}
                  </p>
                </div>

                <form className='space-y-3' onSubmit={handleBuild}>
                  <div className='grid gap-2'>
                    <Label htmlFor='training-dataset-name'>{t('Name')}</Label>
                    <Input
                      id='training-dataset-name'
                      value={buildForm.name}
                      placeholder={t('Default capture dataset')}
                      onChange={(event) =>
                        setBuildForm((current) => ({
                          ...current,
                          name: event.target.value,
                        }))
                      }
                    />
                  </div>
                  <div className='grid gap-2'>
                    <Label htmlFor='training-dataset-version'>
                      {t('Version')}
                    </Label>
                    <Input
                      id='training-dataset-version'
                      value={buildForm.version}
                      placeholder='v1'
                      onChange={(event) =>
                        setBuildForm((current) => ({
                          ...current,
                          version: event.target.value,
                        }))
                      }
                    />
                  </div>
                  <div className='grid grid-cols-2 gap-2'>
                    <div className='grid gap-2'>
                      <Label htmlFor='training-dataset-limit'>
                        {t('Limit')}
                      </Label>
                      <Input
                        id='training-dataset-limit'
                        type='number'
                        min='1'
                        value={buildForm.limit}
                        onChange={(event) =>
                          setBuildForm((current) => ({
                            ...current,
                            limit: event.target.value,
                          }))
                        }
                      />
                    </div>
                    <div className='grid gap-2'>
                      <Label htmlFor='training-dataset-max-bytes'>
                        {t('Max bytes')}
                      </Label>
                      <Input
                        id='training-dataset-max-bytes'
                        type='number'
                        min='1'
                        value={buildForm.maxDecodedBundleBytes}
                        placeholder='1048576'
                        onChange={(event) =>
                          setBuildForm((current) => ({
                            ...current,
                            maxDecodedBundleBytes: event.target.value,
                          }))
                        }
                      />
                    </div>
                  </div>
                  <div className='bg-muted/30 flex items-start gap-2 rounded-lg border p-2'>
                    <Checkbox
                      id='training-dataset-include-errored'
                      checked={buildForm.includeErrored}
                      onCheckedChange={(checked) =>
                        setBuildForm((current) => ({
                          ...current,
                          includeErrored: Boolean(checked),
                        }))
                      }
                    />
                    <div className='space-y-1'>
                      <Label
                        htmlFor='training-dataset-include-errored'
                        className='text-xs font-medium'
                      >
                        {t('Include errored captures')}
                      </Label>
                      <p className='text-muted-foreground text-xs'>
                        {t(
                          'Use only for diagnosis or supervised cleanup datasets.'
                        )}
                      </p>
                    </div>
                  </div>
                  <Button
                    type='submit'
                    className='w-full'
                    disabled={buildMutation.isPending}
                  >
                    {buildMutation.isPending ? (
                      <RefreshCw className='size-4 animate-spin' aria-hidden />
                    ) : (
                      <Database className='size-4' aria-hidden />
                    )}
                    {t('Build dataset')}
                  </Button>
                </form>
              </SectionPanel>

              <SectionPanel className='min-w-0'>
                <div className='mb-3 flex flex-col gap-3 lg:flex-row lg:items-end lg:justify-between'>
                  <div className='space-y-1'>
                    <h3 className='text-sm font-semibold'>
                      {t('Dataset versions')}
                    </h3>
                    <p className='text-muted-foreground text-xs'>
                      {t('{{count}} dataset versions', {
                        count: datasetTotal,
                      })}
                    </p>
                  </div>
                  <div className='flex flex-wrap items-center gap-2'>
                    <Input
                      className='w-48'
                      value={datasetName}
                      placeholder={t('Filter by name')}
                      onChange={(event) => {
                        setDatasetPage(1)
                        setDatasetName(event.target.value)
                      }}
                    />
                    <NativeSelect
                      value={datasetStatus}
                      onChange={(event) => {
                        setDatasetPage(1)
                        setDatasetStatus(event.target.value)
                      }}
                    >
                      <option value=''>{t('All statuses')}</option>
                      <option value='completed'>{t('Built')}</option>
                      <option value='building'>{t('Building')}</option>
                      <option value='failed'>{t('Failed')}</option>
                      <option value='expired'>{t('Expired')}</option>
                    </NativeSelect>
                    {selectedDatasetId ? (
                      <Button
                        variant='outline'
                        render={
                          <a
                            href={selectedExportUrl}
                            target='_blank'
                            rel='noopener noreferrer'
                          />
                        }
                      >
                        <Download className='size-4' aria-hidden />
                        {t('Download approved export')}
                      </Button>
                    ) : null}
                  </div>
                </div>

                {datasetsQuery.isLoading ? (
                  <DatasetSkeleton />
                ) : datasets.length === 0 ? (
                  <Empty className='min-h-[14rem] border'>
                    <EmptyTitle>{t('No training datasets')}</EmptyTitle>
                    <EmptyDescription>
                      {t(
                        'Build a dataset after request capture has stored raw bundles.'
                      )}
                    </EmptyDescription>
                  </Empty>
                ) : (
                  <div className='rounded-lg border'>
                    <Table>
                      <TableHeader>
                        <TableRow>
                          <TableHead>{t('Dataset')}</TableHead>
                          <TableHead>{t('Status')}</TableHead>
                          <TableHead>{t('Samples')}</TableHead>
                          <TableHead>{t('Size')}</TableHead>
                          <TableHead>{t('Built At')}</TableHead>
                          <TableHead className='w-24 text-right'>
                            {t('Action')}
                          </TableHead>
                        </TableRow>
                      </TableHeader>
                      <TableBody>
                        {datasets.map((dataset) => (
                          <TableRow
                            key={dataset.id}
                            data-state={
                              dataset.id === selectedDatasetId
                                ? 'selected'
                                : undefined
                            }
                          >
                            <TableCell>
                              <div className='min-w-[180px]'>
                                <div className='font-medium'>
                                  {dataset.name || '-'}
                                </div>
                                <div className='text-muted-foreground font-mono text-xs'>
                                  {dataset.version || `#${dataset.id}`}
                                </div>
                              </div>
                            </TableCell>
                            <TableCell>
                              <StatusBadge
                                label={t(
                                  displayLabel(
                                    dataset.status,
                                    datasetStatusLabel
                                  )
                                )}
                                variant={
                                  datasetStatusVariant[dataset.status] ??
                                  'neutral'
                                }
                                copyable={false}
                              />
                              {dataset.last_error ? (
                                <div className='text-destructive mt-1 max-w-[240px] truncate text-xs'>
                                  {dataset.last_error}
                                </div>
                              ) : null}
                            </TableCell>
                            <TableCell>
                              {formatCount(dataset.sample_count)}
                            </TableCell>
                            <TableCell>
                              {formatBytes(dataset.size_bytes)}
                            </TableCell>
                            <TableCell>
                              {formatTimestampToDate(dataset.built_at)}
                            </TableCell>
                            <TableCell className='text-right'>
                              <Button
                                size='sm'
                                variant={
                                  dataset.id === selectedDatasetId
                                    ? 'secondary'
                                    : 'outline'
                                }
                                onClick={() => {
                                  setSelectedDatasetId(dataset.id)
                                  setSamplePage(1)
                                }}
                              >
                                {dataset.id === selectedDatasetId
                                  ? t('Selected')
                                  : t('Select')}
                              </Button>
                            </TableCell>
                          </TableRow>
                        ))}
                      </TableBody>
                    </Table>
                    <div className='flex items-center justify-between gap-2 border-t p-2 text-xs'>
                      <span className='text-muted-foreground'>
                        {t('Page {{page}}', { page: datasetPage })}
                      </span>
                      <div className='flex items-center gap-2'>
                        <Button
                          size='sm'
                          variant='outline'
                          disabled={datasetPage <= 1}
                          onClick={() =>
                            setDatasetPage((page) => Math.max(1, page - 1))
                          }
                        >
                          {t('Previous')}
                        </Button>
                        <Button
                          size='sm'
                          variant='outline'
                          disabled={datasetPage * 10 >= datasetTotal}
                          onClick={() => setDatasetPage((page) => page + 1)}
                        >
                          {t('Next')}
                        </Button>
                      </div>
                    </div>
                  </div>
                )}
              </SectionPanel>
            </div>

            <SectionPanel className='min-w-0'>
              <div className='mb-3 flex flex-col gap-3 xl:flex-row xl:items-end xl:justify-between'>
                <div className='space-y-1'>
                  <h3 className='text-sm font-semibold'>
                    {t('Sample review')}
                  </h3>
                  <p className='text-muted-foreground text-xs'>
                    {selectedDataset
                      ? t('Reviewing {{name}} / {{version}}', {
                          name: selectedDataset.name || '-',
                          version:
                            selectedDataset.version || `#${selectedDataset.id}`,
                        })
                      : t('Select a dataset version to review samples.')}
                  </p>
                </div>
                <div className='flex flex-wrap items-center gap-2'>
                  <Filter className='text-muted-foreground size-4' />
                  <Input
                    className='w-56'
                    value={sampleFilters.requestId}
                    placeholder={t('Request ID')}
                    onChange={(event) => {
                      setSamplePage(1)
                      setSampleFilters((current) => ({
                        ...current,
                        requestId: event.target.value,
                      }))
                    }}
                  />
                  <Input
                    className='w-44'
                    value={sampleFilters.modelName}
                    placeholder={t('Model')}
                    onChange={(event) => {
                      setSamplePage(1)
                      setSampleFilters((current) => ({
                        ...current,
                        modelName: event.target.value,
                      }))
                    }}
                  />
                  <Input
                    className='w-32'
                    type='number'
                    min='0'
                    max='100'
                    step='1'
                    value={sampleFilters.minQualityScore}
                    placeholder={t('Min score')}
                    onChange={(event) => {
                      setSamplePage(1)
                      setSampleFilters((current) => ({
                        ...current,
                        minQualityScore: event.target.value,
                      }))
                    }}
                  />
                  <NativeSelect
                    value={sampleFilters.reviewStatus}
                    onChange={(event) => {
                      setSamplePage(1)
                      setSampleFilters((current) => ({
                        ...current,
                        reviewStatus: event.target.value,
                      }))
                    }}
                  >
                    <option value=''>{t('All reviews')}</option>
                    <option value='pending'>{t('Pending')}</option>
                    <option value='approved'>{t('Approved')}</option>
                    <option value='rejected'>{t('Rejected')}</option>
                  </NativeSelect>
                </div>
              </div>

              {!selectedDatasetId ? (
                <Empty className='min-h-[14rem] border'>
                  <EmptyTitle>{t('No dataset selected')}</EmptyTitle>
                  <EmptyDescription>
                    {t(
                      'Select or build a dataset version before reviewing samples.'
                    )}
                  </EmptyDescription>
                </Empty>
              ) : samplesQuery.isLoading ? (
                <DatasetSkeleton />
              ) : samples.length === 0 ? (
                <Empty className='min-h-[14rem] border'>
                  <EmptyTitle>{t('No training samples')}</EmptyTitle>
                  <EmptyDescription>
                    {t(
                      'No samples match the current dataset and review filters.'
                    )}
                  </EmptyDescription>
                </Empty>
              ) : (
                <div className='rounded-lg border'>
                  <Table>
                    <TableHeader>
                      <TableRow>
                        <TableHead>{t('Request ID')}</TableHead>
                        <TableHead>{t('Model')}</TableHead>
                        <TableHead>{t('Quality')}</TableHead>
                        <TableHead>{t('Redaction')}</TableHead>
                        <TableHead>{t('Review')}</TableHead>
                        <TableHead>{t('Created At')}</TableHead>
                        <TableHead className='w-28 text-right'>
                          {t('Action')}
                        </TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {samples.map((sample) => (
                        <TableRow key={sample.id}>
                          <TableCell>
                            {sample.request_id ? (
                              <div className='flex max-w-[300px] items-center gap-1.5'>
                                <span className='min-w-0 flex-1 truncate font-mono text-xs'>
                                  {sample.request_id}
                                </span>
                                <Button
                                  variant='ghost'
                                  size='icon'
                                  className='text-muted-foreground hover:text-foreground size-6 shrink-0'
                                  aria-label={t('Filter by Request ID')}
                                  render={
                                    <Link
                                      to='/usage-logs/$section'
                                      params={{ section: 'common' }}
                                      search={{
                                        requestId: sample.request_id,
                                      }}
                                    />
                                  }
                                >
                                  <ListFilter
                                    className='size-3.5'
                                    aria-hidden
                                  />
                                </Button>
                              </div>
                            ) : (
                              <span className='text-muted-foreground'>-</span>
                            )}
                          </TableCell>
                          <TableCell>
                            <span className='block max-w-[220px] truncate font-mono text-xs'>
                              {sample.model_name || '-'}
                            </span>
                          </TableCell>
                          <TableCell>
                            {qualityLabel(sample.quality_score)}
                          </TableCell>
                          <TableCell>
                            <StatusBadge
                              label={t(
                                displayLabel(
                                  sample.redaction_status,
                                  redactionStatusLabel
                                )
                              )}
                              variant={
                                redactionStatusVariant[
                                  sample.redaction_status
                                ] ?? 'neutral'
                              }
                              copyable={false}
                            />
                          </TableCell>
                          <TableCell>
                            <StatusBadge
                              label={t(
                                displayLabel(
                                  sample.review_status,
                                  reviewStatusLabel
                                )
                              )}
                              variant={
                                reviewStatusVariant[sample.review_status] ??
                                'neutral'
                              }
                              copyable={false}
                            />
                          </TableCell>
                          <TableCell>
                            {formatTimestampToDate(sample.created_at)}
                          </TableCell>
                          <TableCell className='text-right'>
                            <Button
                              size='sm'
                              variant='outline'
                              onClick={() => handleOpenPreview(sample)}
                            >
                              <Eye className='size-4' aria-hidden />
                              {t('Preview')}
                            </Button>
                          </TableCell>
                        </TableRow>
                      ))}
                    </TableBody>
                  </Table>
                  <div className='flex items-center justify-between gap-2 border-t p-2 text-xs'>
                    <span className='text-muted-foreground'>
                      {t('{{count}} samples', { count: sampleTotal })}
                    </span>
                    <div className='flex items-center gap-2'>
                      <Button
                        size='sm'
                        variant='outline'
                        disabled={samplePage <= 1}
                        onClick={() =>
                          setSamplePage((page) => Math.max(1, page - 1))
                        }
                      >
                        {t('Previous')}
                      </Button>
                      <Button
                        size='sm'
                        variant='outline'
                        disabled={samplePage * 20 >= sampleTotal}
                        onClick={() => setSamplePage((page) => page + 1)}
                      >
                        {t('Next')}
                      </Button>
                    </div>
                  </div>
                </div>
              )}
            </SectionPanel>
          </div>
        </SectionPageLayout.Content>
      </SectionPageLayout>

      <SamplePreviewDialog
        open={previewSampleId !== null}
        preview={preview}
        loading={previewQuery.isLoading}
        comment={reviewComment}
        onCommentChange={setReviewComment}
        onOpenChange={(open) => {
          if (!open) {
            setPreviewSampleId(null)
            setReviewComment('')
          }
        }}
        onApprove={() => {
          if (previewSampleId) {
            reviewMutation.mutate({
              action: 'approve',
              sampleId: previewSampleId,
              comment: reviewComment,
            })
          }
        }}
        onReject={() => {
          if (previewSampleId) {
            reviewMutation.mutate({
              action: 'reject',
              sampleId: previewSampleId,
              comment: reviewComment,
            })
          }
        }}
        reviewing={reviewMutation.isPending}
      />
    </>
  )
}

function SamplePreviewDialog(props: {
  open: boolean
  preview: TrainingSamplePreview | undefined
  loading: boolean
  comment: string
  reviewing: boolean
  onCommentChange: (value: string) => void
  onOpenChange: (open: boolean) => void
  onApprove: () => void
  onReject: () => void
}) {
  const { t } = useTranslation()
  const sample = props.preview?.sample

  return (
    <Dialog open={props.open} onOpenChange={props.onOpenChange}>
      <DialogContent className='max-h-[90vh] overflow-hidden sm:max-w-5xl'>
        <DialogHeader>
          <DialogTitle>{t('Sample preview')}</DialogTitle>
          <DialogDescription>
            {sample
              ? t('Request {{requestId}} from {{model}}', {
                  requestId: sample.request_id || '-',
                  model: sample.model_name || '-',
                })
              : t('Loading sample preview...')}
          </DialogDescription>
        </DialogHeader>

        {props.loading ? (
          <div className='space-y-3'>
            <Skeleton className='h-6 w-64' />
            <Skeleton className='h-72 w-full' />
          </div>
        ) : props.preview ? (
          <div className='grid min-h-0 gap-3 lg:grid-cols-[1fr_280px]'>
            <div className='min-h-0 overflow-hidden rounded-lg border'>
              <div className='bg-muted/40 flex items-center justify-between border-b px-3 py-2'>
                <span className='text-sm font-medium'>
                  {t('Redacted JSONL line')}
                </span>
                <StatusBadge
                  label={t(
                    displayLabel(sample?.review_status, reviewStatusLabel)
                  )}
                  variant={
                    sample
                      ? (reviewStatusVariant[sample.review_status] ?? 'neutral')
                      : 'neutral'
                  }
                  copyable={false}
                />
              </div>
              <pre className='max-h-[56vh] overflow-auto p-3 font-mono text-xs leading-relaxed whitespace-pre-wrap'>
                {safeJson(props.preview.line)}
              </pre>
            </div>

            <div className='space-y-3 rounded-lg border p-3'>
              <div className='space-y-2 text-sm'>
                <DetailRow
                  label={t('Dataset')}
                  value={props.preview.dataset.name}
                />
                <DetailRow
                  label={t('Version')}
                  value={
                    props.preview.dataset.version ||
                    `#${props.preview.dataset.id}`
                  }
                />
                <DetailRow
                  label={t('Quality')}
                  value={qualityLabel(sample?.quality_score)}
                />
                <DetailRow
                  label={t('Source hash')}
                  value={sample?.source_hash || '-'}
                />
                <DetailRow
                  label={t('Created At')}
                  value={formatTimestampToDate(sample?.created_at)}
                />
              </div>
              <div className='grid gap-2'>
                <Label htmlFor='training-sample-review-comment'>
                  {t('Review comment')}
                </Label>
                <Textarea
                  id='training-sample-review-comment'
                  value={props.comment}
                  placeholder={t('Optional reviewer note')}
                  onChange={(event) =>
                    props.onCommentChange(event.target.value)
                  }
                />
              </div>
            </div>
          </div>
        ) : (
          <Empty className='min-h-[16rem] border'>
            <EmptyTitle>{t('Preview unavailable')}</EmptyTitle>
            <EmptyDescription>
              {t('The selected sample could not be loaded.')}
            </EmptyDescription>
          </Empty>
        )}

        <DialogFooter>
          <Button
            variant='outline'
            onClick={() => props.onOpenChange(false)}
            disabled={props.reviewing}
          >
            {t('Close')}
          </Button>
          <Button
            variant='destructive'
            onClick={props.onReject}
            disabled={!sample || props.reviewing}
          >
            <X className='size-4' aria-hidden />
            {t('Reject')}
          </Button>
          <Button
            onClick={props.onApprove}
            disabled={!sample || props.reviewing}
          >
            <Check className='size-4' aria-hidden />
            {t('Approve')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

function DetailRow(props: { label: string; value: string }) {
  return (
    <div className='grid gap-1'>
      <div className='text-muted-foreground text-xs'>{props.label}</div>
      <div className='min-w-0 truncate font-mono text-xs'>{props.value}</div>
    </div>
  )
}
