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
import { Link } from '@tanstack/react-router'
import { ArrowRight, Clock3 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'

export function UIV2MCPOperationsPlaceholder() {
  const { t } = useTranslation()

  return (
    <section className='border-border bg-card rounded-xl border'>
      <div className='border-border flex flex-col gap-3 border-b px-4 py-3 sm:flex-row sm:items-center sm:justify-between'>
        <div className='min-w-0'>
          <h2 className='text-foreground text-base font-semibold'>
            {t('MCP operations pilot')}
          </h2>
          <p className='text-muted-foreground mt-1 max-w-2xl text-sm text-pretty'>
            {t(
              'The v2 shell is ready. The next step connects this surface to the existing MCP operations data.'
            )}
          </p>
        </div>
        <div className='text-muted-foreground flex items-center gap-1.5 text-xs'>
          <Clock3 className='size-3.5' />
          <span>{t('Waiting for data surface')}</span>
        </div>
      </div>

      <div className='grid gap-3 p-4 md:grid-cols-3'>
        {[
          {
            label: t('Source APIs'),
            value: t('Existing MCP dashboard queries'),
          },
          {
            label: t('State coverage'),
            value: t('Loading, empty, partial, error, permission'),
          },
          {
            label: t('Current fallback'),
            value: t('Open current MCP dashboard'),
          },
        ].map((item) => (
          <div
            key={item.label}
            className='border-border bg-muted/30 rounded-lg border px-3 py-2'
          >
            <div className='text-muted-foreground text-xs'>{item.label}</div>
            <div className='text-foreground mt-1 text-sm font-medium'>
              {item.value}
            </div>
          </div>
        ))}
      </div>

      <div className='border-border flex justify-end border-t px-4 py-3'>
        <Button
          variant='outline'
          render={<Link to='/mcp/$section' params={{ section: 'overview' }} />}
        >
          {t('Open current MCP')}
          <ArrowRight className='size-4' />
        </Button>
      </div>
    </section>
  )
}
