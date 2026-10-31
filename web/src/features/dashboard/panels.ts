import type { ComponentType } from 'react'
import { AttentionList } from './AttentionList'
import { OpenRepairs } from './OpenRepairs'

/** Panels shown above the garage on the dashboard, most important first. */
export const dashboardPanels: ComponentType[] = [AttentionList, OpenRepairs]
