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
import { useEffect, useState } from 'react'
import { Loader2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { formatQuota, formatCompactNumber } from '@/lib/format'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Label } from '@/components/ui/label'
import { getUserInfo } from '../../api'
import type { UserInfo } from '../../types'

interface UserInfoDialogProps {
  userId: number | null
  open: boolean
  onOpenChange: (open: boolean) => void
}

function InfoItem({
  label,
  value,
}: {
  label: string
  value: string | number
}) {
  return (
    <div className='space-y-1.5'>
      <Label className='text-muted-foreground text-xs'>{label}</Label>
      <div className='text-sm font-semibold'>{value}</div>
    </div>
  )
}

export function UserInfoDialog({
  userId,
  open,
  onOpenChange,
}: UserInfoDialogProps) {
  const { t } = useTranslation()
  const [userInfo, setUserInfo] = useState<UserInfo | null>(null)
  const [loadingUserId, setLoadingUserId] = useState<number | null>(null)

  useEffect(() => {
    if (!open || !userId) return

    let cancelled = false

    const fetchUserInfo = async () => {
      await Promise.resolve()
      if (cancelled) return

      setLoadingUserId(userId)
      setUserInfo(null)
      try {
        const result = await getUserInfo(userId)
        if (cancelled) return

        if (result.success) {
          setUserInfo(result.data || null)
        } else {
          setUserInfo(null)
          toast.error(result.message || t('Failed to fetch user information'))
        }
      } catch (error) {
        if (cancelled) return

        setUserInfo(null)
        // eslint-disable-next-line no-console
        console.error('Failed to fetch user info:', error)
        toast.error(t('Failed to fetch user information'))
      } finally {
        if (!cancelled) {
          setLoadingUserId(null)
        }
      }
    }

    void fetchUserInfo()

    return () => {
      cancelled = true
    }
  }, [open, t, userId])

  const isLoading = open && !!userId && loadingUserId === userId
  const displayedUserInfo = userInfo?.id === userId ? userInfo : null

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='sm:max-w-lg'>
        <DialogHeader>
          <DialogTitle>{t('User Information')}</DialogTitle>
          <DialogDescription>
            {t(
              'View detailed information about this user including balance, usage statistics, and invitation details.'
            )}
          </DialogDescription>
        </DialogHeader>

        {isLoading ? (
          <div className='flex items-center justify-center py-8'>
            <Loader2 className='text-muted-foreground size-6 animate-spin' />
          </div>
        ) : displayedUserInfo ? (
          <div className='space-y-4 py-4'>
            {/* Basic Info */}
            <div className='grid grid-cols-2 gap-4'>
              <InfoItem
                label={t('Username')}
                value={displayedUserInfo.username}
              />
              {displayedUserInfo.display_name && (
                <InfoItem
                  label={t('Display Name')}
                  value={displayedUserInfo.display_name}
                />
              )}
            </div>

            {/* Balance Info */}
            <div className='grid grid-cols-2 gap-4'>
              <InfoItem
                label={t('Balance')}
                value={formatQuota(displayedUserInfo.quota)}
              />
              <InfoItem
                label={t('Used Quota')}
                value={formatQuota(displayedUserInfo.used_quota)}
              />
            </div>

            {/* Statistics */}
            <div className='grid grid-cols-2 gap-4'>
              <InfoItem
                label={t('Request Count')}
                value={formatCompactNumber(displayedUserInfo.request_count)}
              />
              {displayedUserInfo.group && (
                <InfoItem
                  label={t('User Group')}
                  value={displayedUserInfo.group}
                />
              )}
            </div>

            {/* Invitation Info */}
            {(displayedUserInfo.aff_code ||
              displayedUserInfo.aff_count !== undefined ||
              (displayedUserInfo.aff_quota !== undefined &&
                displayedUserInfo.aff_quota > 0)) && (
              <>
                <div className='grid grid-cols-2 gap-4'>
                  {displayedUserInfo.aff_code && (
                    <InfoItem
                      label={t('Invitation Code')}
                      value={displayedUserInfo.aff_code}
                    />
                  )}
                  {displayedUserInfo.aff_count !== undefined && (
                    <InfoItem
                      label={t('Invited Users')}
                      value={formatCompactNumber(displayedUserInfo.aff_count)}
                    />
                  )}
                </div>

                {displayedUserInfo.aff_quota !== undefined &&
                  displayedUserInfo.aff_quota > 0 && (
                    <InfoItem
                      label={t('Invitation Quota')}
                      value={formatQuota(displayedUserInfo.aff_quota)}
                    />
                  )}
              </>
            )}

            {/* Remark */}
            {displayedUserInfo.remark && (
              <div className='space-y-1.5'>
                <Label className='text-muted-foreground text-xs'>
                  {t('Remark')}
                </Label>
                <div className='text-sm leading-relaxed font-semibold break-words'>
                  {displayedUserInfo.remark}
                </div>
              </div>
            )}
          </div>
        ) : (
          <div className='text-muted-foreground py-8 text-center text-sm'>
            {t('No user information available')}
          </div>
        )}
      </DialogContent>
    </Dialog>
  )
}
