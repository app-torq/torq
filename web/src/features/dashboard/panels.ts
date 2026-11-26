import type { ComponentType } from 'react'
import type { Permission } from '../../lib/types'
import { AttentionList } from './AttentionList'
import { OpenRepairs } from './OpenRepairs'

export interface DashboardPanel {
  Panel: ComponentType
  /** Shown only to users who see this area. */
  sees: Permission
}

/** Panels shown above the garage on the dashboard, most important first. */
export const dashboardPanels: DashboardPanel[] = [
  { Panel: AttentionList, sees: 'reminders.view' },
  { Panel: OpenRepairs, sees: 'repairs.view' },
]
