export interface VehicleTab {
  to: string
  labelKey: string
  end?: boolean
}

export const vehicleTabs: VehicleTab[] = [
  { to: '', labelKey: 'tabs.overview', end: true },
  { to: 'specs', labelKey: 'tabs.specs' },
  { to: 'service', labelKey: 'tabs.service' },
  { to: 'reminders', labelKey: 'tabs.reminders' },
  { to: 'repairs', labelKey: 'tabs.repairs' },
  { to: 'history', labelKey: 'tabs.history' },
]
