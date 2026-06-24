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
import { useCallback, useMemo } from 'react'
import { getRouteApi, useNavigate } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { useAuthStore } from '@/stores/auth-store'
import { ROLE } from '@/lib/roles'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { SectionPageLayout } from '@/components/layout'
import {
  MCP_SECTION_IDS,
  MCP_DEFAULT_SECTION,
  getMCPSectionContent,
  getMCPSectionMeta,
  isMCPSectionId,
  type MCPSectionId,
} from './section-registry'

const route = getRouteApi('/_authenticated/mcp/$section')
const USER_MCP_SECTIONS = new Set<MCPSectionId>([
  'market',
  'my-tunnel-apps',
  'tunnel-connections',
  'tunnel-sessions',
])

export function MCPDashboard() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const params = route.useParams()
  const userRole = useAuthStore((state) => state.auth.user?.role ?? 0)
  const isAdmin = userRole >= ROLE.ADMIN
  const activeSection: MCPSectionId = isMCPSectionId(params.section)
    ? params.section
    : MCP_DEFAULT_SECTION
  const activeMeta = getMCPSectionMeta(activeSection)

  const tabs = useMemo(
    () =>
      MCP_SECTION_IDS.filter(
        (section) => isAdmin || USER_MCP_SECTIONS.has(section)
      ).map((section) => ({
        id: section,
        titleKey: getMCPSectionMeta(section).titleKey,
      })),
    [isAdmin]
  )

  const handleSectionChange = useCallback(
    (section: string) => {
      void navigate({
        to: '/mcp/$section',
        params: { section: section as MCPSectionId },
      })
    },
    [navigate]
  )

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>
        {t(activeMeta.titleKey)}
      </SectionPageLayout.Title>
      <SectionPageLayout.Content>
        <div className='space-y-4'>
          <p className='text-muted-foreground max-w-3xl text-sm leading-6'>
            {t(activeMeta.descriptionKey)}
          </p>
          <Tabs value={activeSection} onValueChange={handleSectionChange}>
            <div className='max-w-full overflow-x-auto pb-1'>
              <TabsList className='w-max min-w-full justify-start group-data-horizontal/tabs:h-9'>
                {tabs.map((tab) => (
                  <TabsTrigger key={tab.id} value={tab.id}>
                    {t(tab.titleKey)}
                  </TabsTrigger>
                ))}
              </TabsList>
            </div>
          </Tabs>
          {getMCPSectionContent(activeSection)}
        </div>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
