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
import { AlertCircle, AlertTriangle, KeyRound, Settings, Wallet } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Link } from '@tanstack/react-router'
import { useAuthStore } from '@/stores/auth-store'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { MESSAGE_STATUS } from '../constants'
import type { Message } from '../types'

interface MessageErrorProps {
  message: Message
  className?: string
}

type FundingErrorGuide = {
  title: string
  href: string
  cta: string
  icon: typeof Wallet
}

function fundingErrorGuide(
  errorCode: string | undefined,
  t: (key: string) => string
): FundingErrorGuide | null {
  switch (errorCode) {
    case 'insufficient_user_quota':
      return {
        title: t('Wallet or subscription quota insufficient'),
        href: '/wallet',
        cta: t('Open wallet'),
        icon: Wallet,
      }
    case 'insufficient_model_token_package':
      return {
        title: t('Model token package insufficient'),
        href: '/wallet#model-token-packages',
        cta: t('View packages'),
        icon: Wallet,
      }
    case 'pre_consume_token_quota_failed':
      return {
        title: t('API key quota limit reached'),
        href: '/keys',
        cta: t('Manage keys'),
        icon: KeyRound,
      }
    default:
      return null
  }
}

/**
 * Display error messages using Alert component
 * Following ai-elements pattern for error handling
 */
export function MessageError({ message, className = '' }: MessageErrorProps) {
  const { t } = useTranslation()
  const user = useAuthStore((s) => s.auth.user)
  const isAdmin = user?.role != null && user.role >= 10

  if (message.status !== MESSAGE_STATUS.ERROR) {
    return null
  }

  const errorContent =
    message.versions[0]?.content || 'An unknown error occurred'

  if (message.errorCode === 'model_price_error') {
    return (
      <Alert variant='default' className={className}>
        <AlertTriangle className='text-orange-500' />
        <AlertTitle>{t('Model Price Not Configured')}</AlertTitle>
        <AlertDescription className='space-y-2'>
          <p>{errorContent}</p>
          {isAdmin && (
            <Button
              variant='outline'
              size='sm'
              onClick={() =>
                window.open('/system-settings/billing/model-pricing', '_blank')
              }
            >
              <Settings className='mr-1 h-3.5 w-3.5' />
              {t('Go to Settings')}
            </Button>
          )}
        </AlertDescription>
      </Alert>
    )
  }

  const guide = fundingErrorGuide(message.errorCode, t)
  if (guide) {
    const Icon = guide.icon
    return (
      <Alert variant='destructive' className={className}>
        <AlertCircle />
        <AlertTitle>{guide.title}</AlertTitle>
        <AlertDescription className='space-y-2'>
          <p>{errorContent}</p>
          {message.errorCode ? (
            <p className='text-muted-foreground font-mono text-xs'>
              {message.errorCode}
            </p>
          ) : null}
          <Button
            variant='outline'
            size='sm'
            render={<Link to={guide.href} />}
          >
            <Icon className='mr-1 h-3.5 w-3.5' />
            {guide.cta}
          </Button>
        </AlertDescription>
      </Alert>
    )
  }

  return (
    <Alert variant='destructive' className={className}>
      <AlertCircle />
      <AlertTitle>{t('Error')}</AlertTitle>
      <AlertDescription className='space-y-1'>
        <p>{errorContent}</p>
        {message.errorCode ? (
          <p className='text-muted-foreground font-mono text-xs'>
            {message.errorCode}
          </p>
        ) : null}
      </AlertDescription>
    </Alert>
  )
}
