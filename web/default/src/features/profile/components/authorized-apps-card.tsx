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
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Loader2, ShieldCheck, Trash2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { api } from '@/lib/api'
import { formatTimestampToDate } from '@/lib/format'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { TitledCard } from '@/components/ui/titled-card'
import { StatusBadge } from '@/components/status-badge'

type ApiEnvelope<T> = {
  success: boolean
  message?: string
  data?: T
}

export type UserConnectedAppGrant = {
  id: number
  app_id: number
  app_slug: string
  app_name: string
  app_trusted: boolean
  scopes: string[]
  status: string
  authorized_at: number
  last_used_at: number
  revoked_at: number
  created_at: number
  updated_at: number
}

const QUERY_KEY = ['self-connected-app-grants']

async function listSelfConnectedAppGrants(): Promise<UserConnectedAppGrant[]> {
  const res = await api.get<ApiEnvelope<{ items: UserConnectedAppGrant[] }>>(
    '/api/user/connected-app-grants',
    { skipBusinessError: true }
  )
  if (!res.data?.success) {
    throw new Error(res.data?.message || 'Failed to load authorized apps')
  }
  return res.data.data?.items ?? []
}

async function revokeSelfConnectedAppGrant(appId: number): Promise<void> {
  const res = await api.delete<ApiEnvelope<unknown>>(
    `/api/user/connected-app-grants/${appId}`,
    { skipBusinessError: true }
  )
  if (!res.data?.success) {
    throw new Error(res.data?.message || 'Failed to revoke authorization')
  }
}

export function AuthorizedAppsCard() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [revokeTarget, setRevokeTarget] = useState<UserConnectedAppGrant | null>(
    null
  )

  const grantsQuery = useQuery({
    queryKey: QUERY_KEY,
    queryFn: listSelfConnectedAppGrants,
  })

  const revokeMutation = useMutation({
    mutationFn: (appId: number) => revokeSelfConnectedAppGrant(appId),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: QUERY_KEY })
      toast.success(t('Authorization revoked'))
      setRevokeTarget(null)
    },
    onError: (error: Error) => {
      toast.error(error.message)
    },
  })

  const items = grantsQuery.data ?? []

  return (
    <>
      <TitledCard
        title={t('Authorized applications')}
        description={t(
          'Third-party apps that can access your Data Proxy account'
        )}
        icon={<ShieldCheck className='size-4' />}
      >
        {grantsQuery.isLoading ? (
          <div className='space-y-3 p-4'>
            <Skeleton className='h-12 w-full' />
            <Skeleton className='h-12 w-full' />
          </div>
        ) : grantsQuery.isError ? (
          <div className='text-destructive p-4 text-sm'>
            {(grantsQuery.error as Error).message}
          </div>
        ) : items.length === 0 ? (
          <div className='text-muted-foreground p-4 text-sm'>
            {t('No authorized applications yet.')}
          </div>
        ) : (
          <div className='divide-y'>
            {items.map((grant) => (
              <div
                key={grant.id}
                className='flex flex-col gap-3 px-4 py-3 sm:flex-row sm:items-center sm:justify-between'
              >
                <div className='min-w-0 space-y-1'>
                  <div className='flex min-w-0 flex-wrap items-center gap-2'>
                    <span className='truncate font-medium'>
                      {grant.app_name || grant.app_slug || `#${grant.app_id}`}
                    </span>
                    {grant.app_slug ? (
                      <Badge variant='outline' className='font-mono'>
                        {grant.app_slug}
                      </Badge>
                    ) : null}
                    {grant.app_trusted ? (
                      <StatusBadge
                        copyable={false}
                        label={t('Trusted')}
                        variant='success'
                      />
                    ) : null}
                  </div>
                  <div className='text-muted-foreground flex flex-wrap gap-x-3 gap-y-1 text-xs'>
                    <span>
                      {t('Authorized')}:{' '}
                      {grant.authorized_at
                        ? formatTimestampToDate(grant.authorized_at)
                        : '-'}
                    </span>
                    <span>
                      {t('Last used')}:{' '}
                      {grant.last_used_at
                        ? formatTimestampToDate(grant.last_used_at)
                        : '-'}
                    </span>
                  </div>
                  {grant.scopes?.length ? (
                    <div className='flex flex-wrap gap-1 pt-1'>
                      {grant.scopes.slice(0, 6).map((scope) => (
                        <Badge
                          key={scope}
                          variant='secondary'
                          className='font-mono text-[10px]'
                        >
                          {scope}
                        </Badge>
                      ))}
                      {grant.scopes.length > 6 ? (
                        <Badge variant='secondary' className='text-[10px]'>
                          +{grant.scopes.length - 6}
                        </Badge>
                      ) : null}
                    </div>
                  ) : null}
                </div>
                <Button
                  type='button'
                  size='sm'
                  variant='outline'
                  className='shrink-0'
                  onClick={() => setRevokeTarget(grant)}
                  disabled={revokeMutation.isPending}
                >
                  <Trash2 data-icon='inline-start' />
                  {t('Revoke')}
                </Button>
              </div>
            ))}
          </div>
        )}
      </TitledCard>

      <AlertDialog
        open={Boolean(revokeTarget)}
        onOpenChange={(open) => {
          if (!open) setRevokeTarget(null)
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('Revoke application access?')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t(
                'This app will lose access to your account. Issued API keys for this app will be disabled.'
              )}
              {revokeTarget?.app_name ? (
                <span className='mt-2 block font-medium text-foreground'>
                  {revokeTarget.app_name}
                </span>
              ) : null}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={revokeMutation.isPending}>
              {t('Cancel')}
            </AlertDialogCancel>
            <AlertDialogAction
              disabled={revokeMutation.isPending || !revokeTarget}
              onClick={(event) => {
                event.preventDefault()
                if (!revokeTarget) return
                revokeMutation.mutate(revokeTarget.app_id)
              }}
            >
              {revokeMutation.isPending ? (
                <Loader2 className='animate-spin' />
              ) : null}
              {t('Revoke')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  )
}
