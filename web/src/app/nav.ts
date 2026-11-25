import type { LucideIcon } from 'lucide-react'
import type { Permission } from '../lib/types'
import { Bell, Car, ChartColumn, Cog, Hammer, LayoutDashboard, NotebookPen } from 'lucide-react'

export interface NavItem {
  to: string
  labelKey: string
  icon: LucideIcon
  end?: boolean
}

export const mainNav: NavItem[] = [
  { to: '/', labelKey: 'nav.dashboard', icon: LayoutDashboard, end: true },
  { to: '/vehicles', labelKey: 'nav.vehicles', icon: Car },
  { to: '/service', labelKey: 'nav.serviceLog', icon: NotebookPen },
  { to: '/repairs', labelKey: 'nav.repairs', icon: Hammer },
  { to: '/reminders', labelKey: 'nav.reminders', icon: Bell },
  { to: '/reports', labelKey: 'nav.reports', icon: ChartColumn },
  { to: '/settings', labelKey: 'nav.settings', icon: Cog },
]

export interface SettingsNavItem {
  to: string
  labelKey: string
  adminOnly?: boolean
  /** Shown only to users with this garage-wide permission. */
  permission?: Permission
}

export const settingsNav: SettingsNavItem[] = [
  { to: '/settings/preferences', labelKey: 'nav.preferences' },
  { to: '/settings/password', labelKey: 'actions.changePassword' },
  { to: '/settings/devices', labelKey: 'nav.devices' },
  { to: '/settings/categories', labelKey: 'nav.categories', permission: 'categories.manage' },
  { to: '/settings/priorities', labelKey: 'nav.priorities', permission: 'repair_lists.manage' },
  { to: '/settings/tags', labelKey: 'nav.tags', permission: 'repair_lists.manage' },
  { to: '/settings/users', labelKey: 'nav.users', adminOnly: true },
  { to: '/settings/roles', labelKey: 'nav.roles', adminOnly: true },
  { to: '/settings/webhooks', labelKey: 'nav.webhooks', adminOnly: true },
  { to: '/settings/instance', labelKey: 'nav.instance', adminOnly: true },
  { to: '/settings/about', labelKey: 'nav.about' },
]
