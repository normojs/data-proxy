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
import { Copy } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { useCopyToClipboard } from '@/hooks/use-copy-to-clipboard'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { ScrollArea } from '@/components/ui/scroll-area'

type JsonDetailDialogProps = {
  description?: string
  open: boolean
  title: string
  value: unknown
  onOpenChange: (open: boolean) => void
}

function formatJsonValue(value: unknown): string {
  if (typeof value === 'string') {
    const trimmed = value.trim()
    if (!trimmed) return ''
    try {
      return JSON.stringify(JSON.parse(trimmed), null, 2)
    } catch {
      return value
    }
  }

  try {
    return JSON.stringify(value ?? {}, null, 2)
  } catch {
    return String(value ?? '')
  }
}

export function JsonDetailDialog(props: JsonDetailDialogProps) {
  const { t } = useTranslation()
  const { copyToClipboard } = useCopyToClipboard()
  const content = formatJsonValue(props.value)

  return (
    <Dialog open={props.open} onOpenChange={props.onOpenChange}>
      <DialogContent className='max-w-[calc(100%-2rem)] sm:max-w-3xl'>
        <DialogHeader>
          <div className='flex items-start justify-between gap-3 pr-8'>
            <div className='min-w-0'>
              <DialogTitle>{props.title}</DialogTitle>
              {props.description && (
                <DialogDescription className='mt-1'>
                  {props.description}
                </DialogDescription>
              )}
            </div>
            <Button
              type='button'
              variant='outline'
              size='sm'
              onClick={() => void copyToClipboard(content)}
            >
              <Copy className='size-3.5' />
              {t('Copy')}
            </Button>
          </div>
        </DialogHeader>
        <ScrollArea className='bg-muted/30 max-h-[60vh] rounded-lg border'>
          <pre className='min-w-0 overflow-x-auto p-3 font-mono text-xs leading-relaxed whitespace-pre-wrap'>
            {content || '-'}
          </pre>
        </ScrollArea>
      </DialogContent>
    </Dialog>
  )
}
