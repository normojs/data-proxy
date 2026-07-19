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
import { useEffect } from 'react'
import { Link, createFileRoute } from '@tanstack/react-router'
import { Copy, Gift, Loader2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { SectionPageLayout } from '@/components/layout'
import { useAffiliate } from '@/features/wallet/hooks'

export const Route = createFileRoute('/_authenticated/invitation/')({
  component: InvitationPage,
})

function InvitationPage() {
  const { t } = useTranslation()
  const {
    affiliateCode,
    affiliateLink,
    loading,
    copyAffiliateLink,
    refetch,
  } = useAffiliate()

  useEffect(() => {
    void refetch()
  }, [refetch])

  const handleCopy = () => {
    copyAffiliateLink()
    toast.success(t('Copied'))
  }

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>{t('Invite friends')}</SectionPageLayout.Title>
      <SectionPageLayout.Content>
        <div className='mx-auto flex w-full max-w-2xl flex-col gap-6'>
          <Card>
            <CardHeader>
              <CardTitle className='flex items-center gap-2'>
                <Gift className='size-5' />
                {t('Your invitation link')}
              </CardTitle>
              <CardDescription>
                {t(
                  'Friends open this link to register. Rewards follow system invitation settings.'
                )}
              </CardDescription>
            </CardHeader>
            <CardContent className='space-y-4'>
              {loading ? (
                <div className='text-muted-foreground flex items-center gap-2 text-sm'>
                  <Loader2 className='size-4 animate-spin' />
                  {t('Loading…')}
                </div>
              ) : (
                <>
                  <div className='flex flex-col gap-2 sm:flex-row'>
                    <Input
                      readOnly
                      value={affiliateLink || ''}
                      placeholder={t('Invitation link unavailable')}
                      className='font-mono text-sm'
                    />
                    <Button
                      type='button'
                      variant='secondary'
                      onClick={handleCopy}
                      disabled={!affiliateLink}
                    >
                      <Copy className='size-4' />
                      {t('Copy link')}
                    </Button>
                  </div>
                  {affiliateCode ? (
                    <p className='text-muted-foreground text-sm'>
                      {t('Referral code')}:{' '}
                      <span className='font-mono'>{affiliateCode}</span>
                    </p>
                  ) : null}
                </>
              )}
              <div className='flex flex-wrap gap-3 text-sm'>
                <Link
                  to='/wallet'
                  className='text-primary font-medium underline underline-offset-4'
                >
                  {t('Open wallet & rewards')}
                </Link>
                <Link
                  to='/profile'
                  className='text-muted-foreground underline underline-offset-4'
                >
                  {t('Profile')}
                </Link>
              </div>
            </CardContent>
          </Card>
        </div>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
