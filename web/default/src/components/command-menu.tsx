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
import React from 'react'
import { useLocation, useNavigate } from '@tanstack/react-router'
import { ArrowRight, ChevronRight, Laptop, Moon, Sun } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { useSearch } from '@/context/search-provider'
import { useTheme } from '@/context/theme-provider'
import { useSidebarData } from '@/hooks/use-sidebar-data'
import {
  Command,
  CommandDialog,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
  CommandSeparator,
} from '@/components/ui/command'
import { getNavGroupsForPath } from './layout/lib/sidebar-view-registry'
import { ScrollArea } from './ui/scroll-area'

type SearchFieldItem = {
  title: string
  parent: string
  to: string
  keywords: string[]
}

export function CommandMenu() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const { setTheme } = useTheme()
  const { open, setOpen } = useSearch()
  const { pathname } = useLocation()
  const sidebarData = useSidebarData()

  // Use the active nested sidebar view's nav groups when one matches
  // the current URL; otherwise fall back to the root navigation.
  const navGroups = getNavGroupsForPath(pathname, t) ?? sidebarData.navGroups
  const settingsFieldItems = React.useMemo<SearchFieldItem[]>(
    () => [
      {
        title: t('System Name'),
        parent: t('System Information'),
        to: '/system-settings/site/system-info',
        keywords: [t('Site & Branding'), 'SystemName'],
      },
      {
        title: t('Server Address'),
        parent: t('System Information'),
        to: '/system-settings/site/system-info',
        keywords: [t('Site & Branding'), 'ServerAddress', 'Base URL'],
      },
      {
        title: t('Logo URL'),
        parent: t('System Information'),
        to: '/system-settings/site/system-info',
        keywords: [t('Site & Branding'), t('Upload Logo'), 'Logo'],
      },
      {
        title: t('Footer'),
        parent: t('System Information'),
        to: '/system-settings/site/system-info',
        keywords: [t('Site & Branding'), 'Footer'],
      },
      {
        title: t('About'),
        parent: t('System Information'),
        to: '/system-settings/site/system-info',
        keywords: [t('Site & Branding'), 'About'],
      },
      {
        title: t('Home Page Content'),
        parent: t('System Information'),
        to: '/system-settings/site/system-info',
        keywords: [t('Site & Branding'), 'HomePageContent'],
      },
      {
        title: t('User Agreement'),
        parent: t('System Information'),
        to: '/system-settings/site/system-info',
        keywords: [t('Site & Branding'), 'legal.user_agreement'],
      },
      {
        title: t('Privacy Policy'),
        parent: t('System Information'),
        to: '/system-settings/site/system-info',
        keywords: [t('Site & Branding'), 'legal.privacy_policy'],
      },
      {
        title: t('Notice'),
        parent: t('System Notice'),
        to: '/system-settings/site/notice',
        keywords: [t('Site & Branding'), 'Notice'],
      },
      {
        title: t('API Info'),
        parent: t('Console Content'),
        to: '/system-settings/content/api-info',
        keywords: [t('Content'), 'console_setting.api_info'],
      },
      {
        title: t('Announcements'),
        parent: t('Console Content'),
        to: '/system-settings/content/announcements',
        keywords: [t('Content'), 'console_setting.announcements'],
      },
    ],
    [t]
  )

  const runCommand = React.useCallback(
    (command: () => unknown) => {
      setOpen(false)
      command()
    },
    [setOpen]
  )

  return (
    <CommandDialog modal open={open} onOpenChange={setOpen}>
      <Command>
        <CommandInput placeholder={t('Type a command or search...')} />
        <CommandList>
          <ScrollArea className='h-72 pe-1'>
            <CommandEmpty>{t('No results found.')}</CommandEmpty>
            {navGroups.map((group) => (
              <CommandGroup key={group.id || group.title} heading={group.title}>
                {group.items.map((navItem, i) => {
                  if (navItem.url)
                    return (
                      <CommandItem
                        key={`${navItem.url}-${i}`}
                        value={navItem.title}
                        onSelect={() => {
                          runCommand(() => navigate({ to: navItem.url }))
                        }}
                      >
                        <div className='flex size-4 items-center justify-center'>
                          <ArrowRight className='text-muted-foreground/80 size-2' />
                        </div>
                        {navItem.title}
                      </CommandItem>
                    )

                  return navItem.items?.map((subItem, i) => (
                    <CommandItem
                      key={`${navItem.title}-${subItem.url}-${i}`}
                      value={`${navItem.title}-${subItem.url}`}
                      onSelect={() => {
                        runCommand(() => navigate({ to: subItem.url }))
                      }}
                    >
                      <div className='flex size-4 items-center justify-center'>
                        <ArrowRight className='text-muted-foreground/80 size-2' />
                      </div>
                      {navItem.title} <ChevronRight /> {subItem.title}
                    </CommandItem>
                  ))
                })}
              </CommandGroup>
            ))}
            <CommandSeparator />
            <CommandGroup heading={t('Settings Fields')}>
              {settingsFieldItems.map((item) => (
                <CommandItem
                  key={`${item.to}-${item.title}`}
                  value={`${item.title} ${item.parent} ${item.keywords.join(' ')}`}
                  onSelect={() => {
                    runCommand(() => navigate({ to: item.to }))
                  }}
                >
                  <div className='flex size-4 items-center justify-center'>
                    <ArrowRight className='text-muted-foreground/80 size-2' />
                  </div>
                  <span className='text-muted-foreground'>{item.parent}</span>
                  <ChevronRight />
                  <span>{item.title}</span>
                </CommandItem>
              ))}
            </CommandGroup>
            <CommandSeparator />
            <CommandGroup heading={t('Theme')}>
              <CommandItem onSelect={() => runCommand(() => setTheme('light'))}>
                <Sun /> <span>{t('Light')}</span>
              </CommandItem>
              <CommandItem onSelect={() => runCommand(() => setTheme('dark'))}>
                <Moon className='scale-90' />
                <span>{t('Dark')}</span>
              </CommandItem>
              <CommandItem
                onSelect={() => runCommand(() => setTheme('system'))}
              >
                <Laptop />
                <span>{t('System')}</span>
              </CommandItem>
            </CommandGroup>
          </ScrollArea>
        </CommandList>
      </Command>
    </CommandDialog>
  )
}
