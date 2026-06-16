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
import { ArrowRight, BookOpen, ExternalLink } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { useStatus } from '@/hooks/use-status'
import { Button } from '@/components/ui/button'
import { AnimateInView } from '@/components/animate-in-view'

interface CTAProps {
  className?: string
  isAuthenticated?: boolean
}

export function CTA(props: CTAProps) {
  const { t } = useTranslation()
  const { status } = useStatus()
  const docsUrl =
    (status?.docs_link as string | undefined) || 'https://docs.newapi.pro'
  const isExternalDocs = docsUrl.startsWith('http')

  if (props.isAuthenticated) {
    return null
  }

  return (
    <section className='relative z-10 px-6 py-20 md:py-24'>
      <AnimateInView
        className='border-border bg-muted/20 mx-auto grid max-w-6xl gap-6 rounded-xl border p-6 md:grid-cols-[1fr_auto] md:items-center md:p-8'
        animation='fade-up'
      >
        <div className='max-w-2xl'>
          <p className='text-sm font-medium'>
            {t('Ready for a governed gateway?')}
          </p>
          <h2 className='mt-3 text-2xl font-semibold tracking-tight text-balance md:text-3xl'>
            {t(
              'Bring model APIs, MCP tools, and local Bridge access under one operational contract.'
            )}
          </h2>
          <p className='text-muted-foreground mt-4 text-sm leading-6 text-pretty'>
            {t(
              'Start with keys and routing, then add billing, binary object storage, audit trails, and operator review as your deployment grows.'
            )}
          </p>
        </div>
        <div className='flex flex-wrap gap-3 md:justify-end'>
          <Button
            className='h-10 rounded-lg px-4'
            render={<Link to='/sign-up' />}
          >
            {t('Get Started')}
            <ArrowRight className='size-4' />
          </Button>
          {isExternalDocs ? (
            <Button
              variant='outline'
              className='h-10 rounded-lg px-4'
              render={
                <a href={docsUrl} target='_blank' rel='noopener noreferrer' />
              }
            >
              <BookOpen className='size-4' />
              {t('Docs')}
              <ExternalLink className='size-3.5' />
            </Button>
          ) : (
            <Button
              variant='outline'
              className='h-10 rounded-lg px-4'
              render={<Link to={docsUrl} />}
            >
              <BookOpen className='size-4' />
              {t('Docs')}
            </Button>
          )}
        </div>
      </AnimateInView>
    </section>
  )
}
