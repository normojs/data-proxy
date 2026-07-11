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
import { useTranslation } from 'react-i18next'
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import {
  actualPriceAmount,
  actualPriceUnitLabel,
  actualPriceWindowLabel,
  formatActualPriceCount,
  formatActualPriceTimestamp,
  formatActualPriceValue,
  isActualPriceFallback,
} from '../lib/actual-price'
import type { PricingModel, TokenUnit } from '../types'

export function ActualPriceCell(props: {
  model: PricingModel
  tokenUnit: TokenUnit
}) {
  const { t } = useTranslation()
  const actual = props.model.actual_price
  const amount = actualPriceAmount(actual, props.model, props.tokenUnit)
  const cachedAmount = actualPriceAmount(
    actual?.cached_price,
    props.model,
    props.tokenUnit
  )
  const unitLabel = actualPriceUnitLabel(
    props.model,
    props.tokenUnit,
    t('request')
  )
  const fallback = isActualPriceFallback(actual)

  if (
    !actual ||
    !actual.request_count ||
    amount == null ||
    !Number.isFinite(amount)
  ) {
    return (
      <span className='text-muted-foreground/30 text-xs'>
        {t('No recent usage')}
      </span>
    )
  }
  const scopeLabel = fallback
    ? t('Last trade')
    : actualPriceWindowLabel(actual, t('Recent 1h'), (count, limit) => {
        const formattedLimit = formatActualPriceCount(limit)
        if (count < limit) {
          return t('Recent {{count}}/{{limit}} trades', {
            count: formatActualPriceCount(count),
            limit: formattedLimit,
          })
        }
        return t('Recent {{limit}} trades', { limit: formattedLimit })
      })

  return (
    <TooltipProvider>
      <Tooltip>
        <TooltipTrigger render={<div />}>
          <div className='min-w-[130px]'>
            <span className='font-mono text-sm tabular-nums'>
              {formatActualPriceValue(amount)}
            </span>
            <div className='text-muted-foreground/50 text-[10px]'>
              / {unitLabel} · {scopeLabel}
            </div>
            {fallback && (
              <div className='text-[10px] text-amber-600 dark:text-amber-400'>
                {t('May have changed')}
              </div>
            )}
          </div>
        </TooltipTrigger>
        <TooltipContent side='top' className='max-w-[300px] p-2.5'>
          <div className='space-y-1 text-xs'>
            <div className='font-medium'>
              {fallback ? t('Last settled price') : t('Platform actual price')}
            </div>
            <div className='text-muted-foreground'>
              {fallback
                ? t(
                    'No recent settled sample was found. This is the last settled price and may have changed.'
                  )
                : t('Blended from the latest settled platform usage samples.')}
            </div>
            {!fallback &&
              actual.cache_token_threshold &&
              cachedAmount != null &&
              Number.isFinite(cachedAmount) && (
                <div className='text-muted-foreground'>
                  {t('Cache hit')}: {formatActualPriceValue(cachedAmount)} /{' '}
                  {unitLabel}
                  {' · '}
                  {t(
                    'Cache hit samples require cache_tokens > {{threshold}}.',
                    {
                      threshold: formatActualPriceCount(
                        actual.cache_token_threshold
                      ),
                    }
                  )}
                </div>
              )}
            <div className='grid grid-cols-[auto_1fr] gap-x-3 gap-y-1 pt-1'>
              {fallback && (
                <>
                  <span className='text-muted-foreground'>
                    {t('Last trade')}
                  </span>
                  <span className='text-right font-mono'>
                    {formatActualPriceTimestamp(actual.last_transaction_at)}
                  </span>
                </>
              )}
              <span className='text-muted-foreground'>{t('Requests')}</span>
              <span className='text-right font-mono'>
                {formatActualPriceCount(actual.request_count)}
              </span>
              <span className='text-muted-foreground'>
                {t('Billable tokens')}
              </span>
              <span className='text-right font-mono'>
                {formatActualPriceCount(actual.total_billable_tokens)}
              </span>
              <span className='text-muted-foreground'>{t('Input')}</span>
              <span className='text-right font-mono'>
                {formatActualPriceCount(actual.input_tokens)}
              </span>
              <span className='text-muted-foreground'>{t('Output')}</span>
              <span className='text-right font-mono'>
                {formatActualPriceCount(actual.output_tokens)}
              </span>
              <span className='text-muted-foreground'>{t('Cache')}</span>
              <span className='text-right font-mono'>
                {formatActualPriceCount(actual.cache_tokens)}
              </span>
              <span className='text-muted-foreground'>{t('Cache write')}</span>
              <span className='text-right font-mono'>
                {formatActualPriceCount(actual.cache_creation_tokens)}
              </span>
              <span className='text-muted-foreground'>{t('Actual cost')}</span>
              <span className='text-right font-mono'>
                {formatActualPriceValue(actual.cost)}
              </span>
            </div>
          </div>
        </TooltipContent>
      </Tooltip>
    </TooltipProvider>
  )
}
