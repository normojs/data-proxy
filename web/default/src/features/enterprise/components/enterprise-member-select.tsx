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
import { useMemo, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Check, ChevronsUpDown, X } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { useDebounce } from '@/hooks'
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
import { getEnterpriseMembers } from '../api'
import type { EnterpriseMember } from '../types'

function memberLabel(member: Pick<
  EnterpriseMember,
  'user_id' | 'username' | 'display_name' | 'email'
>) {
  const name = member.display_name || member.username || `user:${member.user_id}`
  if (member.email) {
    return `${name} <${member.email}>`
  }
  return name
}

function memberDescription(member: EnterpriseMember) {
  const parts = [`#${member.user_id}`]
  if (member.org_unit_name) {
    parts.push(member.org_unit_name)
  }
  if (member.username && member.display_name) {
    parts.push(`@${member.username}`)
  }
  return parts.join(' · ')
}

export function EnterpriseMemberSelect(props: {
  value: string
  onValueChange: (value: string) => void
  placeholder?: string
  disabled?: boolean
  allowClear?: boolean
}) {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)
  const [searchValue, setSearchValue] = useState('')
  const debouncedSearch = useDebounce(searchValue, 300)
  const selectedUserId = Number(props.value || 0)

  const membersQuery = useQuery({
    queryKey: ['enterprise', 'member-select', debouncedSearch],
    queryFn: () =>
      getEnterpriseMembers({
        p: 1,
        page_size: 20,
        keyword: debouncedSearch.trim() || undefined,
      }),
    enabled: open || selectedUserId > 0,
  })

  const members = membersQuery.data?.data?.items ?? []

  const selectedMember = useMemo(() => {
    if (selectedUserId <= 0) return null
    return members.find((member) => member.user_id === selectedUserId) ?? null
  }, [members, selectedUserId])

  const selectedLabel =
    selectedMember != null
      ? memberLabel(selectedMember)
      : selectedUserId > 0
        ? `#${selectedUserId}`
        : ''

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <div className='relative'>
        <PopoverTrigger
          render={
            <Button
              type='button'
              variant='outline'
              role='combobox'
              aria-expanded={open}
              disabled={props.disabled}
              className='h-9 w-full justify-between px-3 font-normal'
            />
          }
        >
          <span className='min-w-0 flex-1 truncate text-left'>
            {selectedLabel || props.placeholder || t('Select enterprise member')}
          </span>
          <ChevronsUpDown className='ml-2 size-4 shrink-0 opacity-50' />
        </PopoverTrigger>
        {props.allowClear !== false && selectedUserId > 0 ? (
          <Button
            type='button'
            variant='ghost'
            size='icon-sm'
            className='absolute top-1/2 right-8 size-6 -translate-y-1/2'
            onClick={(event) => {
              event.preventDefault()
              event.stopPropagation()
              props.onValueChange('')
            }}
          >
            <X className='size-3.5' />
            <span className='sr-only'>{t('Clear')}</span>
          </Button>
        ) : null}
      </div>
      <PopoverContent
        className='w-[var(--anchor-width)] p-0'
        onWheel={(event) => event.stopPropagation()}
        onTouchMove={(event) => event.stopPropagation()}
        onPointerDown={(event) => event.stopPropagation()}
      >
        <Command shouldFilter={false}>
          <CommandInput
            placeholder={t('Search members...')}
            value={searchValue}
            onValueChange={setSearchValue}
          />
          <CommandList className='max-h-64'>
            <CommandEmpty>
              {membersQuery.isLoading
                ? t('Loading...')
                : t('No enterprise members found')}
            </CommandEmpty>
            <CommandGroup>
              {members.map((member) => (
                <CommandItem
                  key={member.user_id}
                  value={String(member.user_id)}
                  onSelect={() => {
                    props.onValueChange(String(member.user_id))
                    setOpen(false)
                    setSearchValue('')
                  }}
                  className='items-start gap-2'
                >
                  <Check
                    className={cn(
                      'mt-0.5 size-4 shrink-0',
                      selectedUserId === member.user_id
                        ? 'opacity-100'
                        : 'opacity-0'
                    )}
                  />
                  <span className='min-w-0 flex-1'>
                    <span className='block truncate font-medium'>
                      {memberLabel(member)}
                    </span>
                    <span className='text-muted-foreground block truncate text-xs'>
                      {memberDescription(member)}
                    </span>
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
