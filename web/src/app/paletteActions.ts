import type { LucideIcon } from 'lucide-react'
import { Car, Gauge, Hammer, NotebookPen } from 'lucide-react'

/** A palette action. Actions that need a vehicle first show a vehicle picker. */
export interface QuickActionDef {
  id: string
  /** i18n key with namespace, e.g. "vehicles:quick.logOdometer". */
  labelKey: string
  icon: LucideIcon
  needsVehicle: boolean
  /** Either an inline odometer form or a navigation target. */
  kind: 'odometer' | 'navigate'
  to?: (vehicleId: number | null) => string
}

export const quickActions: QuickActionDef[] = [
  { id: 'odometer', labelKey: 'vehicles:quick.logOdometer', icon: Gauge, needsVehicle: true, kind: 'odometer' },
  { id: 'new-service', labelKey: 'service:quick.newService', icon: NotebookPen, needsVehicle: true, kind: 'navigate', to: (id) => `/service/new?vehicle=${id ?? ''}` },
  { id: 'new-repair', labelKey: 'repairs:quick.newRepair', icon: Hammer, needsVehicle: false, kind: 'navigate', to: () => '/repairs?new=1' },
  { id: 'add-vehicle', labelKey: 'vehicles:quick.addVehicle', icon: Car, needsVehicle: false, kind: 'navigate', to: () => '/vehicles/new' },
]
