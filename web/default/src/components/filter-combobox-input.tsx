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
import { useMemo, useState, type KeyboardEventHandler } from 'react'
import { Check, ChevronsUpDown, X } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import {
  Command,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
} from '@/components/ui/command'
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from '@/components/ui/popover'

export type FilterComboboxOption = {
  value: string
  label?: string
  description?: string
}

type FilterComboboxInputProps = {
  value?: string
  onValueChange: (value: string) => void
  options?: FilterComboboxOption[]
  placeholder?: string
  searchPlaceholder?: string
  emptyText?: string
  inputType?: string
  className?: string
  onKeyDown?: KeyboardEventHandler<HTMLInputElement>
}

function normalizeOptions(
  options: FilterComboboxOption[] | undefined
): FilterComboboxOption[] {
  const seen = new Set<string>()
  return (options ?? [])
    .map((option) => ({
      ...option,
      value: option.value.trim(),
      label: option.label?.trim() || option.value.trim(),
    }))
    .filter((option) => {
      if (!option.value || seen.has(option.value)) return false
      seen.add(option.value)
      return true
    })
}

export function FilterComboboxInput({
  value = '',
  onValueChange,
  options,
  placeholder,
  searchPlaceholder,
  emptyText,
  inputType = 'text',
  className,
  onKeyDown,
}: FilterComboboxInputProps) {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)
  const [searchValue, setSearchValue] = useState('')
  const normalizedOptions = useMemo(() => normalizeOptions(options), [options])

  const filteredOptions = useMemo(() => {
    const search = searchValue.trim().toLowerCase()
    if (!search) return normalizedOptions
    return normalizedOptions.filter((option) => {
      return (
        option.value.toLowerCase().includes(search) ||
        option.label?.toLowerCase().includes(search) ||
        option.description?.toLowerCase().includes(search)
      )
    })
  }, [normalizedOptions, searchValue])

  const hasOptions = normalizedOptions.length > 0

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <div className='relative'>
        <input
          type={inputType}
          value={value}
          placeholder={placeholder}
          onChange={(event) => onValueChange(event.target.value)}
          onKeyDown={onKeyDown}
          className={cn(
            'border-input bg-background ring-offset-background placeholder:text-muted-foreground focus-visible:ring-ring flex h-8 w-full rounded-md border px-3 py-1 text-sm transition-colors file:border-0 file:bg-transparent file:text-sm file:font-medium focus-visible:ring-2 focus-visible:ring-offset-2 focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-50',
            hasOptions ? 'pr-16' : 'pr-8',
            className
          )}
        />
        {value && (
          <Button
            type='button'
            variant='ghost'
            size='icon'
            className='text-muted-foreground hover:text-foreground absolute top-1/2 right-8 size-7 -translate-y-1/2'
            onClick={() => onValueChange('')}
            tabIndex={-1}
            aria-label={t('Clear')}
          >
            <X className='size-3.5' />
          </Button>
        )}
        <PopoverTrigger
          render={
            <Button
              type='button'
              variant='ghost'
              size='icon'
              className='text-muted-foreground hover:text-foreground absolute top-1/2 right-1 size-7 -translate-y-1/2'
              disabled={!hasOptions}
              aria-label={t('Select')}
            />
          }
        >
          <ChevronsUpDown className='size-3.5' />
        </PopoverTrigger>
      </div>

      <PopoverContent
        className='w-[var(--anchor-width)] overflow-hidden p-0'
        align='start'
        onWheel={(event) => event.stopPropagation()}
        onTouchMove={(event) => event.stopPropagation()}
      >
        <Command shouldFilter={false}>
          <CommandInput
            placeholder={searchPlaceholder || t('Search...')}
            value={searchValue}
            onValueChange={setSearchValue}
          />
          <CommandList className='max-h-64'>
            <CommandEmpty>{emptyText || t('No results found.')}</CommandEmpty>
            <CommandGroup>
              {filteredOptions.map((option) => (
                <CommandItem
                  key={option.value}
                  value={option.value}
                  onSelect={() => {
                    onValueChange(option.value)
                    setOpen(false)
                    setSearchValue('')
                  }}
                  className='items-start gap-2'
                >
                  <Check
                    className={cn(
                      'mt-0.5 size-4',
                      value === option.value ? 'opacity-100' : 'opacity-0'
                    )}
                  />
                  <span className='min-w-0 flex-1'>
                    <span className='block truncate'>{option.label}</span>
                    {option.description && (
                      <span className='text-muted-foreground block truncate text-xs'>
                        {option.description}
                      </span>
                    )}
                  </span>
                </CommandItem>
              ))}
            </CommandGroup>
          </CommandList>
        </Command>
      </PopoverContent>
    </Popover>
  )
}
