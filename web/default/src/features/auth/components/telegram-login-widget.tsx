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
import { useEffect, useId, useRef } from 'react'
import { Send } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import type { TelegramAuthPayload } from '../types'

declare global {
  interface Window {
    __newApiTelegramAuth?: Record<
      string,
      (payload: TelegramAuthPayload) => void
    >
  }
}

type TelegramLoginWidgetProps = {
  botName?: string
  authUrl?: string
  disabled?: boolean
  label?: string
  size?: 'small' | 'medium' | 'large'
  radius?: number
  className?: string
  onAuth?: (payload: TelegramAuthPayload) => void | Promise<void>
}

function normalizeTelegramBotName(botName?: string) {
  return botName?.trim().replace(/^@+/, '') ?? ''
}

export function TelegramLoginWidget({
  botName,
  authUrl,
  disabled = false,
  label,
  size = 'large',
  radius = 8,
  className,
  onAuth,
}: TelegramLoginWidgetProps) {
  const { t } = useTranslation()
  const containerRef = useRef<HTMLDivElement | null>(null)
  const onAuthRef = useRef(onAuth)
  const callbackKey = `telegram_auth_${useId()}`
  const normalizedBotName = normalizeTelegramBotName(botName)

  useEffect(() => {
    onAuthRef.current = onAuth
  }, [onAuth])

  useEffect(() => {
    const container = containerRef.current
    if (
      !container ||
      disabled ||
      !normalizedBotName ||
      (!authUrl && !onAuthRef.current)
    ) {
      return
    }

    container.innerHTML = ''

    const script = document.createElement('script')
    script.async = true
    script.src = 'https://telegram.org/js/telegram-widget.js?22'
    script.setAttribute('data-telegram-login', normalizedBotName)
    script.setAttribute('data-size', size)
    script.setAttribute('data-radius', String(radius))

    if (authUrl) {
      script.setAttribute('data-auth-url', authUrl)
    } else if (onAuthRef.current) {
      window.__newApiTelegramAuth = window.__newApiTelegramAuth ?? {}
      window.__newApiTelegramAuth[callbackKey] = (payload) => {
        void onAuthRef.current?.(payload)
      }
      script.setAttribute(
        'data-onauth',
        `window.__newApiTelegramAuth[${JSON.stringify(callbackKey)}](user)`
      )
    }

    container.appendChild(script)

    return () => {
      if (window.__newApiTelegramAuth?.[callbackKey]) {
        delete window.__newApiTelegramAuth[callbackKey]
      }
      container.innerHTML = ''
    }
  }, [authUrl, callbackKey, disabled, normalizedBotName, radius, size])

  if (!normalizedBotName) {
    return (
      <Button
        type='button'
        variant='outline'
        disabled
        className={cn('h-11 w-full justify-center gap-2 rounded-lg', className)}
      >
        <Send className='h-4 w-4' />
        {t('Telegram bot is not configured')}
      </Button>
    )
  }

  if (disabled) {
    return (
      <Button
        type='button'
        variant='outline'
        disabled
        className={cn('h-11 w-full justify-center gap-2 rounded-lg', className)}
      >
        <Send className='h-4 w-4' />
        {label ?? t('Continue with Telegram')}
      </Button>
    )
  }

  return (
    <div
      ref={containerRef}
      className={cn(
        'flex min-h-11 w-full items-center justify-center',
        className
      )}
    />
  )
}
