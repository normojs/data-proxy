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
import { useAuthStore } from '@/stores/auth-store'
import { parseHeaderNavModulesFromStatus } from '@/lib/nav-modules'
import { ROLE } from '@/lib/roles'
import { useNotifications } from '@/hooks/use-notifications'
import { useStatus } from '@/hooks/use-status'
import { useTopNavLinks } from '@/hooks/use-top-nav-links'
import { ConfigDrawer } from '@/components/config-drawer'
import { LanguageSwitcher } from '@/components/language-switcher'
import {
  NotificationPopover,
  RequiredAnnouncementDialog,
} from '@/components/notification-popover'
import { ProfileDropdown } from '@/components/profile-dropdown'
import { Search } from '@/components/search'
import { ServiceStatusIndicator } from '@/features/service-status/service-status-indicator'
import { defaultTopNavLinks } from '../config/top-nav.config'
import { type TopNavLink } from '../types'
import { Header } from './header'
import { SystemBrand } from './system-brand'
import { TopNav } from './top-nav'

/**
 * General application Header component
 * Integrates navigation bar, search, configuration and profile functions
 *
 * @example
 * // Basic usage
 * <AppHeader />
 *
 * @example
 * // Custom navigation links
 * <AppHeader navLinks={customLinks} />
 *
 * @example
 * // Hide navigation bar and search box
 * <AppHeader showTopNav={false} showSearch={false} />
 *
 * @example
 * // Fully customize left and right content
 * <AppHeader
 *   leftContent={<CustomLeft />}
 *   rightContent={<CustomRight />}
 * />
 */
type AppHeaderProps = {
  /**
   * Custom navigation links, uses default global navigation or dynamically generated from backend if not provided
   */
  navLinks?: TopNavLink[]
  /**
   * Whether to show top navigation bar
   * @default true
   */
  showTopNav?: boolean
  /**
   * Left content, overrides TopNav if provided
   */
  leftContent?: React.ReactNode
  /**
   * Whether to show search box
   * @default true
   */
  showSearch?: boolean
  /**
   * Custom right content, overrides default right content if provided
   */
  rightContent?: React.ReactNode
  /**
   * Whether to show notification button
   * @default true
   */
  showNotifications?: boolean
  /**
   * Whether to show config drawer
   * @default true
   */
  showConfigDrawer?: boolean
  /**
   * Whether to show profile dropdown
   * @default true
   */
  showProfileDropdown?: boolean
}

export function AppHeader({
  navLinks = defaultTopNavLinks,
  showTopNav = true,
  leftContent,
  showSearch = true,
  rightContent,
  showNotifications = true,
  showConfigDrawer = true,
  showProfileDropdown = true,
}: AppHeaderProps) {
  // Prioritize dynamically generated links from backend
  const dynamicLinks = useTopNavLinks()
  const links = dynamicLinks.length > 0 ? dynamicLinks : navLinks
  const userRole = useAuthStore((state) => state.auth.user?.role ?? 0)
  const isAdmin = userRole >= ROLE.ADMIN
  const { status } = useStatus()
  const headerNavModules = parseHeaderNavModulesFromStatus(
    status as Record<string, unknown> | null
  )
  const showServiceStatus = isAdmin && headerNavModules.serviceStatus !== false

  // Notifications hook
  const notifications = useNotifications()

  return (
    <>
      <Header>
        <SystemBrand variant='inline' />

        {leftContent ? (
          <div className='ms-2 flex items-center'>{leftContent}</div>
        ) : null}

        {rightContent ?? (
          <div className='ms-auto flex items-center gap-1 sm:gap-2'>
            {showTopNav && (
              <div className='me-1 hidden lg:block'>
                <TopNav links={links} />
              </div>
            )}
            {showServiceStatus && <ServiceStatusIndicator />}
            {showSearch && <Search />}
            {showNotifications && (
              <NotificationPopover
                open={notifications.popoverOpen}
                onOpenChange={notifications.setPopoverOpen}
                unreadCount={notifications.unreadCount}
                activeTab={notifications.activeTab}
                onTabChange={notifications.setActiveTab}
                notice={notifications.notice}
                announcements={notifications.announcements}
                approvalNotifications={notifications.approvalNotifications}
                showApprovals={notifications.approvalNotificationsEnabled}
                showApprovalAuditLinks={notifications.approvalAuditLinksEnabled}
                loading={notifications.loading}
                approvalsLoading={notifications.approvalsLoading}
                approvalsUnreadOnly={notifications.approvalsUnreadOnly}
                approvalsHasMore={notifications.hasMoreApprovalNotifications}
                approvalsLoadingMore={notifications.approvalsFetchingNextPage}
                onMarkAllAnnouncementsAsRead={() =>
                  notifications.markAnnouncementsAsRead()
                }
                onMarkAnnouncementRead={(key) =>
                  notifications.markAnnouncementsAsRead([key])
                }
                onMarkAllApprovalsAsRead={() =>
                  notifications.markApprovalsAsRead()
                }
                onMarkApprovalRead={(key) =>
                  notifications.markApprovalsAsRead([key])
                }
                onApprovalsUnreadOnlyChange={
                  notifications.setApprovalsUnreadOnly
                }
                onLoadMoreApprovals={() =>
                  notifications.loadMoreApprovalNotifications()
                }
              />
            )}
            <LanguageSwitcher />
            {showConfigDrawer && <ConfigDrawer />}
            {showProfileDropdown && <ProfileDropdown />}
          </div>
        )}
      </Header>
      {showNotifications && (
        <RequiredAnnouncementDialog
          announcement={notifications.popupAnnouncement}
          onDismiss={(key) => notifications.dismissAnnouncementPopups([key])}
          onMarkRead={(key) => notifications.markAnnouncementsAsRead([key])}
          onMarkAllRead={() => notifications.markAnnouncementsAsRead()}
        />
      )}
    </>
  )
}
