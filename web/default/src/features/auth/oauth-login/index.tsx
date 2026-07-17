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
import { Link, useSearch } from '@tanstack/react-router'
import { Loader2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { useStatus } from '@/hooks/use-status'
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
import { wechatLoginByCode } from '@/features/auth/api'
import { AuthLayout } from '@/features/auth/auth-layout'
import { LegalConsent } from '@/features/auth/components/legal-consent'
import { OAuthProviders } from '@/features/auth/components/oauth-providers'
import { TermsFooter } from '@/features/auth/components/terms-footer'
import { useAuthRedirect } from '@/features/auth/hooks/use-auth-redirect'

function hasAnyOAuthProvider(status: ReturnType<typeof useStatus>['status']) {
  if (!status) return false
  return Boolean(
    status.github_oauth ||
      status.discord_oauth ||
      status.oidc_enabled ||
      status.linuxdo_oauth ||
      status.hstation_oauth ||
      status.telegram_oauth ||
      status.wechat_login ||
      (status.custom_oauth_providers?.length ?? 0) > 0
  )
}

export function OAuthLoginPage() {
  const { t } = useTranslation()
  const { redirect } = useSearch({ from: '/(auth)/oauth-login' })
  const { status, loading } = useStatus()
  const { handleLoginSuccess } = useAuthRedirect()

  const [agreedToLegal, setAgreedToLegal] = useState(false)
  const [wechatCode, setWeChatCode] = useState('')
  const [isWeChatDialogOpen, setIsWeChatDialogOpen] = useState(false)
  const [isWeChatSubmitting, setIsWeChatSubmitting] = useState(false)

  const hasUserAgreement = Boolean(status?.user_agreement_enabled)
  const hasPrivacyPolicy = Boolean(status?.privacy_policy_enabled)
  const requiresLegalConsent = hasUserAgreement || hasPrivacyPolicy
  const hasOAuth = hasAnyOAuthProvider(status)
  const hasWeChatLogin = Boolean(status?.wechat_login)
  const passwordLoginEnabled =
    (status?.password_login_enabled ??
      status?.data?.password_login_enabled ??
      true) !== false

  const wechatQrCodeUrl = useMemo(() => {
    return (
      status?.wechat_qrcode ||
      status?.wechat_qr_code ||
      status?.wechat_qrcode_image_url ||
      status?.wechat_qr_code_image_url ||
      status?.wechat_account_qrcode_image_url ||
      status?.WeChatAccountQRCodeImageURL ||
      status?.data?.wechat_qrcode ||
      status?.data?.WeChatAccountQRCodeImageURL ||
      ''
    )
  }, [status])

  useEffect(() => {
    let cancelled = false
    queueMicrotask(() => {
      if (!cancelled) {
        setAgreedToLegal(!requiresLegalConsent)
      }
    })
    return () => {
      cancelled = true
    }
  }, [requiresLegalConsent])

  const legalConsentErrorMessage = t('Please agree to the legal terms first')

  const handleOpenWeChatDialog = () => {
    if (requiresLegalConsent && !agreedToLegal) {
      toast.error(legalConsentErrorMessage)
      return
    }
    setIsWeChatDialogOpen(true)
  }

  const handleWeChatDialogChange = (open: boolean) => {
    setIsWeChatDialogOpen(open)
    if (!open) {
      setWeChatCode('')
      setIsWeChatSubmitting(false)
    }
  }

  async function handleWeChatLogin() {
    if (!wechatCode.trim()) {
      toast.error(t('Please enter the verification code'))
      return
    }
    setIsWeChatSubmitting(true)
    try {
      const res = await wechatLoginByCode(wechatCode)
      if (res?.success) {
        await handleLoginSuccess(res.data as { id?: number } | null, redirect)
        toast.success(t('Signed in via WeChat'))
        handleWeChatDialogChange(false)
      } else {
        toast.error(res?.message || t('Login failed'))
      }
    } catch {
      toast.error(t('Login failed'))
    } finally {
      setIsWeChatSubmitting(false)
    }
  }

  return (
    <AuthLayout>
      <div className='w-full space-y-8'>
        <div className='space-y-2'>
          <h2 className='text-center text-2xl font-semibold tracking-tight sm:text-left'>
            {t('Sign in with a third-party account')}
          </h2>
          <p className='text-muted-foreground text-left text-sm sm:text-base'>
            {t(
              'Choose a provider below. You will be redirected to authorize, then returned here.'
            )}
          </p>
        </div>

        {loading && !status ? (
          <div className='text-muted-foreground flex items-center justify-center gap-2 py-10 text-sm'>
            <Loader2 className='size-4 animate-spin' />
            {t('Loading sign-in options…')}
          </div>
        ) : hasOAuth ? (
          <div className='space-y-4'>
            <OAuthProviders
              status={status}
              disabled={requiresLegalConsent && !agreedToLegal}
              redirectTo={redirect}
              hideDivider
              onWeChatLogin={hasWeChatLogin ? handleOpenWeChatDialog : undefined}
              isWeChatLoading={isWeChatSubmitting}
            />
            <LegalConsent
              status={status}
              checked={agreedToLegal}
              onCheckedChange={setAgreedToLegal}
            />
            {status?.oauth_register_enabled === false ? (
              <p className='text-muted-foreground text-xs'>
                {t(
                  'New account registration via third-party login is currently disabled. Existing bound accounts can still sign in.'
                )}
              </p>
            ) : null}
          </div>
        ) : (
          <div className='bg-muted/40 space-y-3 rounded-lg border p-4 text-sm'>
            <p className='font-medium'>
              {t('No third-party sign-in methods are enabled')}
            </p>
            <p className='text-muted-foreground'>
              {t(
                'Ask an administrator to enable GitHub, Discord, WeChat, OIDC, LinuxDO, Telegram, or a custom OAuth provider in System Settings.'
              )}
            </p>
          </div>
        )}

        <div className='text-muted-foreground space-y-2 text-center text-sm'>
          {passwordLoginEnabled ? (
            <p>
              {t('Prefer username and password?')}{' '}
              <Link
                to='/sign-in'
                search={redirect ? { redirect } : undefined}
                className='hover:text-primary font-medium underline underline-offset-4'
              >
                {t('Sign in with password')}
              </Link>
            </p>
          ) : null}
          {!status?.self_use_mode_enabled &&
          status?.register_enabled !== false ? (
            <p>
              {t("Don't have an account?")}{' '}
              <Link
                to='/sign-up'
                className='hover:text-primary font-medium underline underline-offset-4'
              >
                {t('Sign up')}
              </Link>
            </p>
          ) : null}
        </div>

        <TermsFooter
          variant='sign-in'
          status={status}
          className='text-center'
        />
      </div>

      {hasWeChatLogin ? (
        <Dialog open={isWeChatDialogOpen} onOpenChange={handleWeChatDialogChange}>
          <DialogContent className='max-w-sm'>
            <DialogHeader className='text-left'>
              <DialogTitle>{t('WeChat sign in')}</DialogTitle>
              <DialogDescription>
                {t(
                  'Scan the QR code to follow the official account and reply with “验证码” to receive your verification code.'
                )}
              </DialogDescription>
            </DialogHeader>
            {wechatQrCodeUrl ? (
              <div className='flex justify-center'>
                <img
                  src={wechatQrCodeUrl}
                  alt={t('WeChat login QR code')}
                  className='h-40 w-40 rounded-md border object-contain'
                />
              </div>
            ) : (
              <p className='text-muted-foreground text-sm'>
                {t('WeChat QR code is not configured')}
              </p>
            )}
            <div className='space-y-2'>
              <Label htmlFor='oauth-login-wechat-code'>{t('Verification code')}</Label>
              <Input
                id='oauth-login-wechat-code'
                value={wechatCode}
                onChange={(event) => setWeChatCode(event.target.value)}
                placeholder={t('Enter the code from WeChat')}
                autoComplete='one-time-code'
              />
            </div>
            <DialogFooter>
              <Button
                type='button'
                disabled={isWeChatSubmitting}
                onClick={handleWeChatLogin}
                className='w-full'
              >
                {isWeChatSubmitting ? (
                  <Loader2 className='size-4 animate-spin' />
                ) : null}
                {t('Continue with WeChat')}
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      ) : null}
    </AuthLayout>
  )
}
