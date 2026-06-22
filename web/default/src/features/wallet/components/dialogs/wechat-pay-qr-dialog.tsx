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
import { CheckCircle2, History, RefreshCw } from 'lucide-react'
import { QRCodeSVG } from 'qrcode.react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import type { WechatPayPaymentData } from '../../types'

interface WechatPayQRDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  payment: WechatPayPaymentData | null
  onRefresh: () => void | Promise<void>
  onOpenBilling: () => void
}

export function WechatPayQRDialog({
  open,
  onOpenChange,
  payment,
  onRefresh,
  onOpenBilling,
}: WechatPayQRDialogProps) {
  const { t } = useTranslation()
  const expiresAt =
    typeof payment?.expires_at === 'number'
      ? new Date(payment.expires_at * 1000).toLocaleTimeString()
      : null
  const handleOpenChange = (
    nextOpen: boolean,
    eventDetails: { reason: string }
  ) => {
    if (!nextOpen && eventDetails.reason === 'escape-key') {
      return
    }
    onOpenChange(nextOpen)
  }

  return (
    <Dialog
      open={open}
      onOpenChange={handleOpenChange}
      disablePointerDismissal
    >
      <DialogContent className='max-sm:w-[calc(100vw-1.5rem)] sm:max-w-md'>
        <DialogHeader>
          <DialogTitle>{t('WeChat Pay')}</DialogTitle>
          <DialogDescription>{t('Scan the QR code to pay')}</DialogDescription>
        </DialogHeader>

        <div className='flex flex-col items-center gap-4 py-2'>
          <div className='rounded-lg border bg-white p-3 shadow-sm'>
            {payment?.code_url ? (
              <QRCodeSVG value={payment.code_url} size={216} marginSize={2} />
            ) : (
              <div className='h-[216px] w-[216px]' />
            )}
          </div>
          <div className='bg-muted/30 w-full rounded-lg border p-3 text-sm'>
            <div className='flex items-center justify-between gap-3'>
              <span className='text-muted-foreground'>{t('Order')}</span>
              <code className='truncate text-xs'>
                {payment?.trade_no || '-'}
              </code>
            </div>
            {expiresAt && (
              <div className='mt-2 flex items-center justify-between gap-3'>
                <span className='text-muted-foreground'>{t('Expires')}</span>
                <span className='text-xs font-medium'>{expiresAt}</span>
              </div>
            )}
          </div>
        </div>

        <DialogFooter className='grid grid-cols-1 gap-2 sm:grid-cols-3 sm:justify-stretch'>
          <Button variant='outline' onClick={onRefresh} className='gap-2'>
            <RefreshCw className='h-4 w-4' />
            {t('Refresh')}
          </Button>
          <Button variant='outline' onClick={onOpenBilling} className='gap-2'>
            <History className='h-4 w-4' />
            {t('Order History')}
          </Button>
          <Button onClick={() => onOpenChange(false)} className='gap-2'>
            <CheckCircle2 className='h-4 w-4' />
            {t('Done')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
