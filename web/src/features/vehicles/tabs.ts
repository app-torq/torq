import type { Permission } from '../../lib/types'

export interface VehicleTab {
  to: string
  labelKey: string
  end?: boolean
  /** Shown only to users who see this area. */
  sees?: Permission
}

export const vehicleTabs: VehicleTab[] = [
  { to: '', labelKey: 'tabs.overview', end: true },
  { to: 'specs', labelKey: 'tabs.specs' },
  { to: 'service', labelKey: 'tabs.service', sees: 'service_logs.view' },
  { to: 'reminders', labelKey: 'tabs.reminders', sees: 'reminders.view' },
  { to: 'repairs', labelKey: 'tabs.repairs', sees: 'repairs.view' },
  { to: 'history', labelKey: 'tabs.history', sees: 'reports.view' },
]
