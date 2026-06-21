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
*/
import { type FormEvent, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Route } from '@/routes/_authenticated/quota-requests'
import { Eye, Plus, RefreshCcw, Trash2 } from 'lucide-react'
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
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
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
import { EmptyState } from '@/components/empty-state'
import { ErrorState } from '@/components/error-state'
import { SectionPageLayout } from '@/components/layout'
import {
  getEnterpriseQuotaRequestPolicies,
  getEnterpriseQuotaRequests,
  submitEnterpriseQuotaRequest,
  withdrawEnterpriseQuotaRequest,
} from '@/features/enterprise/api'
import { QuotaRequestDetailSheet } from '@/features/enterprise/components/quota-request-detail-sheet'
import type {
  EnterpriseQuotaRequestPolicy,
  EnterpriseQuotaRequest,
  EnterpriseQuotaRequestStatus,
} from '@/features/enterprise/types'
import { getApiKeyEnterpriseProjects } from '@/features/keys/api'

const PAGE_SIZE = 10
const ALL_VALUE = '__all__'
const NO_PROJECT_VALUE = '0'

function parsePositiveSearchId(value: string) {
  const trimmed = value.trim()
  if (!trimmed) return undefined
  const parsed = Number(trimmed)
  return Number.isFinite(parsed) && parsed > 0 ? parsed : undefined
}

function todayInputValue() {
  return new Date().toISOString().slice(0, 10)
}

function endOfDayUnix(value: string) {
  const date = value ? new Date(`${value}T23:59:59`) : new Date()
  return Math.floor(date.getTime() / 1000)
}

function formatDateTime(value: number | undefined) {
  if (!value) return '-'
  return new Intl.DateTimeFormat(undefined, {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  }).format(new Date(value * 1000))
}

function formatNumber(value: number | undefined) {
  return new Intl.NumberFormat().format(value ?? 0)
}

function statusLabel(status: EnterpriseQuotaRequestStatus) {
  switch (status) {
    case 'approved':
      return 'Approved'
    case 'rejected':
      return 'Rejected'
    case 'withdrawn':
      return 'Withdrawn'
    case 'expired':
      return 'Expired'
    default:
      return 'Pending'
  }
}

function policyMetricLabel(policy: EnterpriseQuotaRequestPolicy) {
  return policy.metric === 'quota' ? 'Quota' : 'Requests'
}

function policyPeriodLabel(policy: EnterpriseQuotaRequestPolicy) {
  return policy.period === 'month' ? 'Monthly' : 'Daily'
}

function policyTargetLabel(policy: EnterpriseQuotaRequestPolicy) {
  return policy.target_name || `${policy.target_type} #${policy.target_id}`
}

function StatusBadge(props: { status: EnterpriseQuotaRequestStatus }) {
  const { t } = useTranslation()
  return <Badge variant='outline'>{t(statusLabel(props.status))}</Badge>
}

function Field(props: { label: string; children: React.ReactNode }) {
  const { t } = useTranslation()
  return (
    <div className='grid gap-1.5'>
      <Label>{t(props.label)}</Label>
      {props.children}
    </div>
  )
}

function TableSkeleton() {
  return (
    <div className='space-y-2 rounded-lg border p-3'>
      {Array.from({ length: 5 }).map((_, index) => (
        <Skeleton key={index} className='h-9 w-full' />
      ))}
    </div>
  )
}

function QuotaRequestDialog(props: {
  open: boolean
  onOpenChange: (open: boolean) => void
}) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [projectId, setProjectId] = useState(NO_PROJECT_VALUE)
  const [policyId, setPolicyId] = useState('')
  const [limitDelta, setLimitDelta] = useState('')
  const [expiresAt, setExpiresAt] = useState(todayInputValue())
  const [reason, setReason] = useState('')

  const projectsQuery = useQuery({
    queryKey: ['quota-request-projects'],
    queryFn: getApiKeyEnterpriseProjects,
    enabled: props.open,
  })
  const projects = projectsQuery.data?.data ?? []
  const selectedProjectId = Number(projectId || 0)

  const policiesQuery = useQuery({
    queryKey: ['quota-request-policies', selectedProjectId],
    queryFn: () =>
      getEnterpriseQuotaRequestPolicies({
        project_id: selectedProjectId > 0 ? selectedProjectId : undefined,
      }),
    enabled: props.open,
  })
  const policies = policiesQuery.data?.data ?? []
  const selectedPolicy = policies.find(
    (policy) => String(policy.id) === policyId
  )

  const mutation = useMutation({
    mutationFn: () =>
      submitEnterpriseQuotaRequest({
        policy_id: Number(policyId || 0),
        project_id: selectedProjectId,
        limit_delta: Number(limitDelta || 0),
        expires_at: endOfDayUnix(expiresAt),
        reason: reason.trim(),
      }),
    onSuccess: (response) => {
      if (!response.success) return
      toast.success(t('Submitted'))
      setProjectId(NO_PROJECT_VALUE)
      setPolicyId('')
      setLimitDelta('')
      setReason('')
      props.onOpenChange(false)
      queryClient.invalidateQueries({ queryKey: ['quota-requests'] })
      queryClient.invalidateQueries({ queryKey: ['quota-request-policies'] })
      queryClient.invalidateQueries({
        queryKey: ['notifications', 'enterprise-quota-requests'],
      })
    },
  })

  function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    mutation.mutate()
  }

  return (
    <Dialog open={props.open} onOpenChange={props.onOpenChange}>
      <DialogContent className='sm:max-w-lg'>
        <DialogHeader>
          <DialogTitle>{t('Submit Quota Request')}</DialogTitle>
          <DialogDescription>
            {t(
              'Request temporary quota for a policy available to your account.'
            )}
          </DialogDescription>
        </DialogHeader>
        <form className='grid gap-3' onSubmit={handleSubmit}>
          <Field label='Project'>
            <Select
              value={projectId}
              onValueChange={(value) => {
                setProjectId(value ?? NO_PROJECT_VALUE)
                setPolicyId('')
              }}
              disabled={projectsQuery.isLoading}
            >
              <SelectTrigger>
                <SelectValue
                  placeholder={
                    projectsQuery.isLoading
                      ? t('Loading projects')
                      : t('No project')
                  }
                />
              </SelectTrigger>
              <SelectContent alignItemWithTrigger={false}>
                <SelectGroup>
                  <SelectItem value={NO_PROJECT_VALUE}>
                    {t('No project')}
                  </SelectItem>
                  {projects.map((project) => (
                    <SelectItem key={project.id} value={String(project.id)}>
                      <div className='grid min-w-0 gap-0.5'>
                        <span className='truncate font-medium'>
                          {project.name}
                        </span>
                        {project.slug || project.description ? (
                          <span className='text-muted-foreground truncate text-xs'>
                            {project.slug || project.description}
                          </span>
                        ) : null}
                      </div>
                    </SelectItem>
                  ))}
                </SelectGroup>
              </SelectContent>
            </Select>
            <p className='text-muted-foreground text-xs'>
              {t(
                'Select a project to request project-specific quota policies.'
              )}
            </p>
          </Field>
          <Field label='Quota Policy'>
            <Select
              value={policyId}
              onValueChange={(value) => setPolicyId(value ?? '')}
              disabled={policiesQuery.isLoading || policies.length === 0}
            >
              <SelectTrigger>
                <SelectValue
                  placeholder={
                    policiesQuery.isLoading
                      ? t('Loading policies')
                      : t('Select a policy')
                  }
                />
              </SelectTrigger>
              <SelectContent alignItemWithTrigger={false}>
                <SelectGroup>
                  {policies.map((policy) => (
                    <SelectItem key={policy.id} value={String(policy.id)}>
                      <div className='grid min-w-0 gap-0.5'>
                        <span className='truncate font-medium'>
                          {policy.name || `#${policy.id}`}
                        </span>
                        <span className='text-muted-foreground truncate text-xs'>
                          {t(policyMetricLabel(policy))} ·{' '}
                          {t(policyPeriodLabel(policy))} ·{' '}
                          {policyTargetLabel(policy)} ·{' '}
                          {formatNumber(policy.used_value)} /{' '}
                          {formatNumber(policy.limit_value)}
                        </span>
                      </div>
                    </SelectItem>
                  ))}
                </SelectGroup>
              </SelectContent>
            </Select>
            {policiesQuery.isError ? (
              <p className='text-destructive text-xs'>
                {t('Failed to load policies')}
              </p>
            ) : policiesQuery.isSuccess && policies.length === 0 ? (
              <p className='text-muted-foreground text-xs'>
                {t('No requestable quota policies are available.')}
              </p>
            ) : selectedPolicy ? (
              <p className='text-muted-foreground text-xs'>
                {t(policyTargetLabel(selectedPolicy))} ·{' '}
                {t(policyMetricLabel(selectedPolicy))} ·{' '}
                {t(policyPeriodLabel(selectedPolicy))}
              </p>
            ) : null}
          </Field>
          <div className='grid gap-3 sm:grid-cols-2'>
            <Field label='Extra Limit'>
              <Input
                type='number'
                value={limitDelta}
                onChange={(event) => setLimitDelta(event.target.value)}
              />
            </Field>
            <Field label='Expires'>
              <Input
                type='date'
                value={expiresAt}
                onChange={(event) => setExpiresAt(event.target.value)}
              />
            </Field>
          </div>
          <Field label='Reason'>
            <Textarea
              value={reason}
              onChange={(event) => setReason(event.target.value)}
            />
          </Field>
          <DialogFooter>
            <Button
              type='submit'
              disabled={
                mutation.isPending || !policyId || policies.length === 0
              }
            >
              {t('Submit')}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

function QuotaRequestsTable(props: {
  requests: EnterpriseQuotaRequest[]
  onWithdraw: (request: EnterpriseQuotaRequest) => void
  onViewDetails: (request: EnterpriseQuotaRequest) => void
}) {
  const { t } = useTranslation()
  return (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>{t('Policy')}</TableHead>
          <TableHead>{t('Target')}</TableHead>
          <TableHead>{t('Extra Limit')}</TableHead>
          <TableHead>{t('Status')}</TableHead>
          <TableHead>{t('Expires')}</TableHead>
          <TableHead className='text-right'>{t('Actions')}</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {props.requests.map((request) => (
          <TableRow key={request.id}>
            <TableCell>
              <div className='min-w-48'>
                <div className='truncate font-medium'>
                  {request.policy_name || `#${request.policy_id}`}
                </div>
                <div className='text-muted-foreground truncate text-xs'>
                  {request.reason || t('No reason provided')}
                </div>
              </div>
            </TableCell>
            <TableCell>
              {request.target_name ||
                `${request.target_type} #${request.target_id}`}
            </TableCell>
            <TableCell>{formatNumber(request.limit_delta)}</TableCell>
            <TableCell>
              <StatusBadge status={request.status} />
            </TableCell>
            <TableCell>{formatDateTime(request.expires_at)}</TableCell>
            <TableCell className='text-right'>
              <div className='flex justify-end gap-1'>
                <Button
                  variant='ghost'
                  size='icon-sm'
                  onClick={() => props.onViewDetails(request)}
                >
                  <Eye className='size-3.5' />
                  <span className='sr-only'>{t('Details')}</span>
                </Button>
                <Button
                  variant='ghost'
                  size='icon-sm'
                  disabled={request.status !== 'pending'}
                  onClick={() => props.onWithdraw(request)}
                >
                  <Trash2 className='size-3.5' />
                  <span className='sr-only'>{t('Withdraw')}</span>
                </Button>
              </div>
            </TableCell>
          </TableRow>
        ))}
      </TableBody>
    </Table>
  )
}

export function QuotaRequests() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const search = Route.useSearch()
  const navigate = Route.useNavigate()
  const [dialogOpen, setDialogOpen] = useState(false)
  const [viewingRequest, setViewingRequest] =
    useState<EnterpriseQuotaRequest | null>(null)
  const [page, setPage] = useState(1)
  const status = search.status ?? ''
  const requestId = search.quota_request_id
    ? String(search.quota_request_id)
    : ''
  const requestIdValue = search.quota_request_id
  const query = useQuery({
    queryKey: ['quota-requests', page, status, requestId, requestIdValue],
    queryFn: () =>
      getEnterpriseQuotaRequests({
        p: page,
        page_size: PAGE_SIZE,
        id: requestIdValue,
        status,
      }),
  })
  const withdrawMutation = useMutation({
    mutationFn: withdrawEnterpriseQuotaRequest,
    onSuccess: (response) => {
      if (!response.success) return
      toast.success(t('Withdrawn'))
      queryClient.invalidateQueries({ queryKey: ['quota-requests'] })
      queryClient.invalidateQueries({
        queryKey: ['notifications', 'enterprise-quota-requests'],
      })
    },
  })
  const pageData = query.data?.data
  const requests = pageData?.items ?? []
  const total = pageData?.total ?? 0
  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE))

  return (
    <>
      <SectionPageLayout>
        <SectionPageLayout.Title>{t('Quota Requests')}</SectionPageLayout.Title>
        <SectionPageLayout.Actions>
          <Button variant='outline' size='sm' onClick={() => query.refetch()}>
            <RefreshCcw className='size-3.5' />
            {t('Refresh')}
          </Button>
          <Button size='sm' onClick={() => setDialogOpen(true)}>
            <Plus className='size-3.5' />
            {t('Quota Request')}
          </Button>
        </SectionPageLayout.Actions>
        <SectionPageLayout.Content>
          <div className='space-y-3'>
            <div className='flex flex-wrap items-center gap-2'>
              <Select
                value={status || ALL_VALUE}
                onValueChange={(value) => {
                  void navigate({
                    search: (prev) => ({
                      ...prev,
                      status: value === ALL_VALUE ? '' : (value ?? ''),
                    }),
                  })
                  setPage(1)
                }}
              >
                <SelectTrigger className='w-40'>
                  <SelectValue placeholder={t('Status')} />
                </SelectTrigger>
                <SelectContent alignItemWithTrigger={false}>
                  <SelectGroup>
                    <SelectItem value={ALL_VALUE}>
                      {t('All Statuses')}
                    </SelectItem>
                    <SelectItem value='pending'>{t('Pending')}</SelectItem>
                    <SelectItem value='approved'>{t('Approved')}</SelectItem>
                    <SelectItem value='rejected'>{t('Rejected')}</SelectItem>
                    <SelectItem value='withdrawn'>{t('Withdrawn')}</SelectItem>
                    <SelectItem value='expired'>{t('Expired')}</SelectItem>
                  </SelectGroup>
                </SelectContent>
              </Select>
              <Input
                type='number'
                value={requestId}
                onChange={(event) => {
                  const next = parsePositiveSearchId(event.target.value)
                  void navigate({
                    search: (prev) => ({
                      ...prev,
                      quota_request_id: next,
                    }),
                  })
                  setPage(1)
                }}
                placeholder={t('Request ID')}
                className='w-36'
              />
            </div>
            {query.isLoading ? (
              <TableSkeleton />
            ) : query.isError ? (
              <ErrorState
                title='Failed to load quota requests'
                description={query.error?.message || 'Request failed'}
                onRetry={query.refetch}
              />
            ) : requests.length === 0 ? (
              <EmptyState
                title='No quota requests'
                description='Temporary quota requests you submit will appear here.'
              />
            ) : (
              <div className='rounded-lg border'>
                <QuotaRequestsTable
                  requests={requests}
                  onWithdraw={(request) => withdrawMutation.mutate(request.id)}
                  onViewDetails={setViewingRequest}
                />
                <div className='flex flex-wrap items-center justify-between gap-2 border-t px-3 py-2 text-xs'>
                  <span className='text-muted-foreground'>
                    {t('Total')} {formatNumber(total)}
                  </span>
                  <div className='flex items-center gap-2'>
                    <Button
                      variant='outline'
                      size='xs'
                      disabled={page <= 1}
                      onClick={() => setPage(page - 1)}
                    >
                      {t('Previous')}
                    </Button>
                    <span className='text-muted-foreground tabular-nums'>
                      {page} / {totalPages}
                    </span>
                    <Button
                      variant='outline'
                      size='xs'
                      disabled={page >= totalPages}
                      onClick={() => setPage(page + 1)}
                    >
                      {t('Next')}
                    </Button>
                  </div>
                </div>
              </div>
            )}
          </div>
        </SectionPageLayout.Content>
      </SectionPageLayout>
      <QuotaRequestDialog open={dialogOpen} onOpenChange={setDialogOpen} />
      <QuotaRequestDetailSheet
        open={Boolean(viewingRequest)}
        request={viewingRequest}
        onOpenChange={(open) => {
          if (!open) setViewingRequest(null)
        }}
      />
    </>
  )
}
