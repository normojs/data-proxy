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
import { Download, ExternalLink } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { useStatus } from '@/hooks/use-status'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { PublicLayout } from '@/components/layout'
import { DownloadIcon, parseDownloadItems } from './lib'

function DownloadListSkeleton() {
  return (
    <div className='grid gap-3 md:grid-cols-2 xl:grid-cols-3'>
      {Array.from({ length: 6 }).map((_, index) => (
        <div key={index} className='rounded-lg border p-4'>
          <div className='flex items-start gap-3'>
            <Skeleton className='size-10 rounded-lg' />
            <div className='min-w-0 flex-1 space-y-2'>
              <Skeleton className='h-4 w-32' />
              <Skeleton className='h-3 w-full' />
              <Skeleton className='h-3 w-2/3' />
            </div>
          </div>
        </div>
      ))}
    </div>
  )
}

export function Downloads() {
  const { t } = useTranslation()
  const { status, loading } = useStatus()
  const enabled = status ? status.downloads_enabled !== false : false
  const downloads = enabled ? parseDownloadItems(status?.downloads) : []

  return (
    <PublicLayout>
      <div className='mx-auto flex max-w-6xl flex-col gap-6 py-4 md:py-8'>
        <div className='flex flex-col gap-3 border-b pb-5 md:flex-row md:items-end md:justify-between'>
          <div className='space-y-2'>
            <div className='bg-muted text-muted-foreground flex size-10 items-center justify-center rounded-lg'>
              <Download className='size-5' aria-hidden />
            </div>
            <div>
              <h1 className='text-2xl font-semibold tracking-normal'>
                {t('Downloads')}
              </h1>
              <p className='text-muted-foreground max-w-2xl text-sm'>
                {t('Download companion clients and developer tools.')}
              </p>
            </div>
          </div>
          {!loading && downloads.length > 0 && (
            <div className='text-muted-foreground text-sm'>
              {t('{{count}} software links available', {
                count: downloads.length,
              })}
            </div>
          )}
        </div>

        {loading ? (
          <DownloadListSkeleton />
        ) : downloads.length === 0 ? (
          <div className='flex min-h-[18rem] items-center justify-center rounded-lg border border-dashed p-8'>
            <div className='max-w-md space-y-2 text-center'>
              <h2 className='text-base font-medium'>
                {t('No downloads configured')}
              </h2>
              <p className='text-muted-foreground text-sm'>
                {t(
                  'The administrator has not published companion software links yet.'
                )}
              </p>
            </div>
          </div>
        ) : (
          <div className='grid gap-3 md:grid-cols-2 xl:grid-cols-3'>
            {downloads.map((item) => (
              <article
                key={item.id}
                className='bg-card text-card-foreground flex min-h-[11rem] flex-col justify-between rounded-lg border p-4'
              >
                <div className='flex items-start gap-3'>
                  <div className='bg-muted text-muted-foreground flex size-10 shrink-0 items-center justify-center rounded-lg'>
                    <DownloadIcon item={item} />
                  </div>
                  <div className='min-w-0 space-y-1'>
                    <h2 className='truncate text-base font-semibold'>
                      {item.name}
                    </h2>
                    {item.description ? (
                      <p className='text-muted-foreground line-clamp-3 text-sm'>
                        {item.description}
                      </p>
                    ) : null}
                  </div>
                </div>

                <Button
                  size='sm'
                  className='mt-4 w-full justify-between'
                  variant='outline'
                  render={
                    <a
                      href={item.url}
                      target={item.openInNewTab ? '_blank' : undefined}
                      rel={
                        item.openInNewTab ? 'noopener noreferrer' : undefined
                      }
                    />
                  }
                >
                  <span>{t('Open download')}</span>
                  <ExternalLink className='size-4' aria-hidden />
                </Button>
              </article>
            ))}
          </div>
        )}
      </div>
    </PublicLayout>
  )
}
