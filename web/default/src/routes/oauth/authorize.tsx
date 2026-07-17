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
import { z } from 'zod'
import { useMutation, useQuery } from '@tanstack/react-query'
import {
  createFileRoute,
  Link,
  redirect,
  useNavigate,
} from '@tanstack/react-router'
import { Check, Loader2, ShieldCheck, X } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { useAuthStore } from '@/stores/auth-store'
import { api } from '@/lib/api'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { PublicLayout } from '@/components/layout'
import { LoadingState } from '@/components/loading-state'
import { ErrorState } from '@/components/error-state'

const searchSchema = z.object({
  client_id: z.string().optional(),
  redirect_uri: z.string().optional(),
  response_type: z.string().optional(),
  scope: z.string().optional(),
  state: z.string().optional(),
  nonce: z.string().optional(),
  code_challenge: z.string().optional(),
  code_challenge_method: z.string().optional(),
})

type AuthorizePreview = {
  client_id: string
  app: {
    id: number
    slug: string
    name: string
    description?: string
    trusted?: boolean
  }
  redirect_uri: string
  scope: string
  scopes: string[]
  state?: string
  nonce?: string
  code_challenge: string
  code_challenge_method: string
}

export const Route = createFileRoute('/oauth/authorize')({
  validateSearch: searchSchema,
  beforeLoad: ({ location }) => {
    const { auth } = useAuthStore.getState()
    if (!auth.user) {
      throw redirect({
        to: '/sign-in',
        search: { redirect: location.href },
      })
    }
  },
  component: OAuthAuthorizePage,
})

function OAuthAuthorizePage() {
  const { t } = useTranslation()
  const search = Route.useSearch()
  const navigate = useNavigate()
  const [error, setError] = useState<string | null>(null)

  const previewQuery = useQuery({
    queryKey: ['oauth-authorize-preview', search],
    queryFn: async () => {
      const res = await api.get('/api/oauth/authorize/validate', {
        params: {
          client_id: search.client_id,
          redirect_uri: search.redirect_uri,
          response_type: search.response_type || 'code',
          scope: search.scope,
          state: search.state,
          nonce: search.nonce,
          code_challenge: search.code_challenge,
          code_challenge_method: search.code_challenge_method || 'S256',
        },
        skipBusinessError: true,
      })
      if (!res.data?.success) {
        throw new Error(res.data?.message || 'Invalid authorization request')
      }
      return res.data.data as AuthorizePreview
    },
    retry: false,
  })

  const consentMutation = useMutation({
    mutationFn: async (approve: boolean) => {
      const res = await api.post(
        '/api/oauth/consent',
        {
          client_id: search.client_id,
          redirect_uri: search.redirect_uri,
          scope: search.scope,
          state: search.state,
          nonce: search.nonce,
          code_challenge: search.code_challenge,
          code_challenge_method: search.code_challenge_method || 'S256',
          approve,
        },
        { skipBusinessError: true }
      )
      if (!res.data?.success) {
        throw new Error(res.data?.message || 'Consent failed')
      }
      return res.data.data as { redirect_to: string }
    },
    onSuccess: (data) => {
      if (data.redirect_to) {
        window.location.assign(data.redirect_to)
      }
    },
    onError: (err: Error) => {
      setError(err.message)
      toast.error(err.message)
    },
  })

  const scopes = useMemo(
    () => previewQuery.data?.scopes ?? [],
    [previewQuery.data]
  )

  useEffect(() => {
    if (!search.client_id || !search.redirect_uri || !search.code_challenge) {
      setError(
        t(
          'Missing required OAuth parameters (client_id, redirect_uri, code_challenge).'
        )
      )
    }
  }, [search, t])

  return (
    <PublicLayout showAuthButtons={false} showNotifications showThemeSwitch>
      <div className='mx-auto flex max-w-lg flex-col gap-4 py-10'>
        <Card>
          <CardHeader>
            <CardTitle className='flex items-center gap-2'>
              <ShieldCheck className='size-5' />
              {t('Authorize application')}
            </CardTitle>
            <CardDescription>
              {t(
                'This website is requesting access to your Data Proxy account.'
              )}
            </CardDescription>
          </CardHeader>
          <CardContent className='space-y-4'>
            {previewQuery.isLoading ? (
              <LoadingState inline message={t('Loading...')} />
            ) : previewQuery.isError || error ? (
              <ErrorState
                title={t('Cannot continue')}
                description={
                  error || (previewQuery.error as Error | null)?.message
                }
              />
            ) : (
              <>
                <div>
                  <div className='text-lg font-semibold'>
                    {previewQuery.data?.app.name}
                  </div>
                  {previewQuery.data?.app.description ? (
                    <p className='text-muted-foreground text-sm'>
                      {previewQuery.data.app.description}
                    </p>
                  ) : null}
                  {previewQuery.data?.app.trusted ? (
                    <p className='text-muted-foreground mt-1 text-xs'>
                      {t('Trusted application')}
                    </p>
                  ) : null}
                </div>
                <div className='space-y-2'>
                  <div className='text-muted-foreground text-xs font-medium'>
                    {t('Permissions requested')}
                  </div>
                  <div className='flex flex-wrap gap-1.5'>
                    {scopes.map((scope) => (
                      <span
                        key={scope}
                        className='bg-muted rounded-md px-2 py-0.5 font-mono text-[11px]'
                      >
                        {scope}
                      </span>
                    ))}
                  </div>
                </div>
                <p className='text-muted-foreground text-xs'>
                  {t('Redirects to')}:{' '}
                  <span className='font-mono'>
                    {previewQuery.data?.redirect_uri}
                  </span>
                </p>
              </>
            )}
          </CardContent>
          <CardFooter className='flex flex-col gap-2 sm:flex-row sm:justify-between'>
            <Button
              variant='outline'
              className='w-full sm:w-auto'
              onClick={() => navigate({ to: '/developers' })}
            >
              {t('Learn more')}
            </Button>
            <div className='flex w-full gap-2 sm:w-auto'>
              <Button
                variant='outline'
                disabled={
                  previewQuery.isLoading ||
                  !!previewQuery.isError ||
                  consentMutation.isPending
                }
                onClick={() => consentMutation.mutate(false)}
                className='flex-1'
              >
                {consentMutation.isPending ? (
                  <Loader2 className='animate-spin' />
                ) : (
                  <X />
                )}
                {t('Deny')}
              </Button>
              <Button
                disabled={
                  previewQuery.isLoading ||
                  !!previewQuery.isError ||
                  consentMutation.isPending
                }
                onClick={() => consentMutation.mutate(true)}
                className='flex-1'
              >
                {consentMutation.isPending ? (
                  <Loader2 className='animate-spin' />
                ) : (
                  <Check />
                )}
                {t('Approve')}
              </Button>
            </div>
          </CardFooter>
        </Card>
        <p className='text-muted-foreground text-center text-xs'>
          <Link to='/profile' className='underline underline-offset-4'>
            {t('Manage connected apps in Profile')}
          </Link>
        </p>
      </div>
    </PublicLayout>
  )
}
