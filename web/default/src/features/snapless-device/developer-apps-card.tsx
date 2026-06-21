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
import { BellRing, Code2, RefreshCw } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from '@/components/ui/empty'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Skeleton } from '@/components/ui/skeleton'
import { TitledCard } from '@/components/ui/titled-card'
import { StatusBadge } from '@/components/status-badge'
import { ConnectedAppNotificationsSection } from '@/features/system-settings/operations/connected-app-notifications-section'
import {
  type ConnectedAppRequest,
  listSelfConnectedAppRequests,
} from '@/features/system-settings/operations/connected-apps-api'

const CONNECTED_APP_DEVELOPER_REQUESTS_QUERY_KEY = [
  'connected-app-developer',
  'requests',
]

export function ConnectedAppDeveloperAppsCard() {
  const { t } = useTranslation()
  const [selectedSlug, setSelectedSlug] = useState('')
  const requestsQuery = useQuery({
    queryKey: CONNECTED_APP_DEVELOPER_REQUESTS_QUERY_KEY,
    queryFn: () =>
      listSelfConnectedAppRequests({ status: 'approved', page_size: 50 }),
    retry: false,
  })
  const approvedApps = useMemo(
    () =>
      (requestsQuery.data?.items ?? []).filter(
        (request) => request.status === 'approved' && request.app_id > 0
      ),
    [requestsQuery.data?.items]
  )
  const selectedApp =
    approvedApps.find((request) => request.slug === selectedSlug) ??
    approvedApps[0] ??
    null

  return (
    <TitledCard
      title={t('Connected App Developer')}
      description={t('Manage app-level notification delivery')}
      icon={<Code2 className='h-4 w-4' />}
      action={
        <Button
          type='button'
          variant='outline'
          size='sm'
          onClick={() => requestsQuery.refetch()}
          disabled={requestsQuery.isFetching}
        >
          <RefreshCw
            className={
              requestsQuery.isFetching
                ? 'h-3.5 w-3.5 animate-spin'
                : 'h-3.5 w-3.5'
            }
          />
          {t('Refresh')}
        </Button>
      }
    >
      {requestsQuery.isLoading ? (
        <ConnectedAppDeveloperSkeleton />
      ) : requestsQuery.isError ? (
        <Empty className='border-0 py-6'>
          <EmptyHeader>
            <EmptyMedia variant='icon'>
              <BellRing />
            </EmptyMedia>
            <EmptyTitle>{t('Unable to load developer apps')}</EmptyTitle>
            <EmptyDescription>
              {requestsQuery.error instanceof Error
                ? requestsQuery.error.message
                : t('Request failed')}
            </EmptyDescription>
          </EmptyHeader>
        </Empty>
      ) : approvedApps.length === 0 ? (
        <Empty className='border-0 py-6'>
          <EmptyHeader>
            <EmptyMedia variant='icon'>
              <Code2 />
            </EmptyMedia>
            <EmptyTitle>{t('No approved developer apps')}</EmptyTitle>
            <EmptyDescription>
              {t('Approved connected app requests will appear here.')}
            </EmptyDescription>
          </EmptyHeader>
        </Empty>
      ) : selectedApp ? (
        <div className='space-y-4'>
          <div className='flex flex-col gap-3 rounded-xl border p-3 sm:flex-row sm:items-center sm:justify-between'>
            <div className='min-w-0 space-y-1'>
              <div className='flex min-w-0 flex-wrap items-center gap-2'>
                <span className='truncate text-sm font-medium'>
                  {selectedApp.name}
                </span>
                <StatusBadge
                  copyable={false}
                  label={selectedApp.slug}
                  variant='neutral'
                />
              </div>
              <p className='text-muted-foreground line-clamp-2 text-xs'>
                {selectedApp.description || t('Device code connected app')}
              </p>
            </div>
            <Select
              value={selectedApp.slug}
              onValueChange={(value) => setSelectedSlug(value ?? '')}
            >
              <SelectTrigger className='w-full sm:w-72'>
                <SelectValue />
              </SelectTrigger>
              <SelectContent alignItemWithTrigger={false}>
                <SelectGroup>
                  {approvedApps.map((app) => (
                    <SelectItem key={app.id} value={app.slug}>
                      {app.name}
                    </SelectItem>
                  ))}
                </SelectGroup>
              </SelectContent>
            </Select>
          </div>

          <ConnectedAppDeveloperAppSummary app={selectedApp} />
          <ConnectedAppNotificationsSection appSlug={selectedApp.slug} />
        </div>
      ) : null}
    </TitledCard>
  )
}

function ConnectedAppDeveloperAppSummary({
  app,
}: {
  app: ConnectedAppRequest
}) {
  const { t } = useTranslation()

  return (
    <div className='grid gap-3 sm:grid-cols-3'>
      <SummaryCell
        label={t('Scopes')}
        value={app.requested_scopes.join(', ') || '-'}
      />
      <SummaryCell
        label={t('Default scopes')}
        value={app.default_scopes.join(', ') || '-'}
      />
      <SummaryCell label={t('Homepage')} value={app.homepage_url || '-'} />
    </div>
  )
}

function SummaryCell({ label, value }: { label: string; value: string }) {
  return (
    <div className='min-w-0 rounded-xl border px-3 py-2.5'>
      <div className='text-muted-foreground text-xs'>{label}</div>
      <div className='mt-1 truncate text-sm font-medium' title={value}>
        {value}
      </div>
    </div>
  )
}

function ConnectedAppDeveloperSkeleton() {
  return (
    <div className='space-y-3'>
      <Skeleton className='h-16 w-full' />
      <div className='grid gap-3 sm:grid-cols-3'>
        {Array.from({ length: 3 }).map((_, index) => (
          <Skeleton key={index} className='h-16 w-full' />
        ))}
      </div>
      <Skeleton className='h-48 w-full' />
    </div>
  )
}
