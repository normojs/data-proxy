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
import { Link, Outlet, useLocation } from '@tanstack/react-router'
import { Activity, Database, ShieldCheck } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { cn } from '@/lib/utils'
import { Badge } from '@/components/ui/badge'
import {
  Breadcrumb,
  BreadcrumbItem,
  BreadcrumbList,
  BreadcrumbPage,
  BreadcrumbSeparator,
} from '@/components/ui/breadcrumb'
import { UI_V2_NAV_ITEMS } from './constants'

function getCurrentNavItem(pathname: string) {
  return UI_V2_NAV_ITEMS.find((item) => pathname.startsWith(item.to))
}

export function UIV2Shell() {
  const { t } = useTranslation()
  const location = useLocation()
  const currentNavItem = getCurrentNavItem(location.pathname)

  return (
    <main id='content' className='bg-background min-h-full overflow-auto'>
      <div className='bg-background/95 border-border supports-[backdrop-filter]:bg-background/90 sticky top-0 z-10 border-b backdrop-blur'>
        <div className='mx-auto flex w-full max-w-7xl flex-col gap-3 px-4 py-3 md:px-6'>
          <Breadcrumb>
            <BreadcrumbList className='text-xs'>
              <BreadcrumbItem>
                <span>{t('Console')}</span>
              </BreadcrumbItem>
              <BreadcrumbSeparator />
              <BreadcrumbItem>
                <span>{t('UI v2 pilot')}</span>
              </BreadcrumbItem>
              {currentNavItem ? (
                <>
                  <BreadcrumbSeparator />
                  <BreadcrumbItem>
                    <BreadcrumbPage>
                      {t(currentNavItem.titleKey)}
                    </BreadcrumbPage>
                  </BreadcrumbItem>
                </>
              ) : null}
            </BreadcrumbList>
          </Breadcrumb>

          <div className='flex flex-col gap-3 lg:flex-row lg:items-end lg:justify-between'>
            <div className='min-w-0 space-y-1'>
              <div className='flex flex-wrap items-center gap-2'>
                <h1 className='text-foreground text-lg leading-tight font-semibold text-wrap'>
                  {currentNavItem
                    ? t(currentNavItem.titleKey)
                    : t('UI v2 pilot')}
                </h1>
                <Badge variant='outline'>{t('Pilot')}</Badge>
              </div>
              <p className='text-muted-foreground max-w-3xl text-sm text-pretty'>
                {currentNavItem
                  ? t(currentNavItem.descriptionKey)
                  : t('Dense operations shell for the next New API UI.')}
              </p>
            </div>

            <div className='grid grid-cols-3 gap-2 text-xs sm:min-w-[360px]'>
              <div className='border-border bg-muted/40 flex min-w-0 items-center gap-2 rounded-lg border px-2 py-1.5'>
                <Activity className='text-muted-foreground size-3.5 shrink-0' />
                <span className='truncate'>{t('Compact density')}</span>
              </div>
              <div className='border-border bg-muted/40 flex min-w-0 items-center gap-2 rounded-lg border px-2 py-1.5'>
                <Database className='text-muted-foreground size-3.5 shrink-0' />
                <span className='truncate'>{t('Shared data hooks')}</span>
              </div>
              <div className='border-border bg-muted/40 flex min-w-0 items-center gap-2 rounded-lg border px-2 py-1.5'>
                <ShieldCheck className='text-muted-foreground size-3.5 shrink-0' />
                <span className='truncate'>{t('Admin guarded')}</span>
              </div>
            </div>
          </div>

          <nav
            className='flex gap-1 overflow-x-auto'
            aria-label={t('UI v2 pilot navigation')}
          >
            {UI_V2_NAV_ITEMS.map((item) => {
              const active = location.pathname.startsWith(item.to)
              return (
                <Link
                  key={item.id}
                  to={item.to}
                  className={cn(
                    'focus-visible:border-ring focus-visible:ring-ring/50 flex min-w-[172px] flex-col gap-0.5 rounded-lg border px-3 py-2 text-left transition-colors outline-none focus-visible:ring-3',
                    active
                      ? 'border-foreground bg-foreground text-background'
                      : 'border-border bg-background hover:bg-muted'
                  )}
                >
                  <span className='text-sm leading-tight font-medium'>
                    {t(item.titleKey)}
                  </span>
                  <span
                    className={cn(
                      'truncate text-xs',
                      active ? 'text-background/70' : 'text-muted-foreground'
                    )}
                  >
                    {t(item.descriptionKey)}
                  </span>
                </Link>
              )
            })}
          </nav>
        </div>
      </div>

      <div className='mx-auto w-full max-w-7xl px-4 py-4 md:px-6'>
        <Outlet />
      </div>
    </main>
  )
}
