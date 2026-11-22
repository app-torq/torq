import type { Vehicle } from '../../lib/types'
import { HoursQuickUpdate } from './HoursQuickUpdate'
import { OdometerQuickUpdate } from './OdometerQuickUpdate'

/** The quick update for a vehicle's main reading: its odometer, or its hours when it has none. */
export function VehicleReadingUpdate({ vehicle, onDone, compact, autoFocus }: { vehicle: Vehicle; onDone?: () => void; compact?: boolean; autoFocus?: boolean }) {
  const meter = vehicle.hour_meters[0]
  if (!vehicle.has_odometer && meter) return <HoursQuickUpdate meter={meter} {...(onDone ? { onDone } : {})} {...(compact ? { compact } : {})} {...(autoFocus ? { autoFocus } : {})} />
  return <OdometerQuickUpdate vehicle={vehicle} {...(onDone ? { onDone } : {})} {...(compact ? { compact } : {})} {...(autoFocus ? { autoFocus } : {})} />
}
