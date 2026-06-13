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
export const UI_V2_HOME_ROUTE = '/ui-lab'
export const UI_V2_MCP_ROUTE = '/ui-lab/mcp'

export type UIV2NavItem = {
  id: 'mcp'
  titleKey: string
  descriptionKey: string
  to: typeof UI_V2_MCP_ROUTE
}

export const UI_V2_NAV_ITEMS: UIV2NavItem[] = [
  {
    id: 'mcp',
    titleKey: 'MCP operations',
    descriptionKey: 'Bridge, Proxy, OpenAPI, and tool-call operations',
    to: UI_V2_MCP_ROUTE,
  },
]
