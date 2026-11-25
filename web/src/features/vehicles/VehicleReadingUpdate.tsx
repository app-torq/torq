import type { Vehicle } from '../../lib/types'
import { HoursQuickUpdate } from './HoursQuickUpdate'
import { OdometerQuickUpdate } from './OdometerQuickUpdate'

/**
 * The quick update for what a vehicle is tracked by (#39): its odometer, its
 * hour meters, or both. Only the first input takes autoFocus.
 */
export function VehicleReadingUpdate({ vehicle, onDone, compact, autoFocus }: { vehicle: Vehicle; onDone?: () => void; compact?: boolean; autoFocus?: boolean }) {
  const meters = vehicle.hour_meters
  // A vehicle without an odometer always has a meter; fall back to the odometer if not.
  const showOdometer = vehicle.has_odometer || meters.length === 0
  const shared = { ...(onDone ? { onDone } : {}), ...(compact ? { compact } : {}) }
  return (
    <div className="space-y-3">
      {showOdometer && <OdometerQuickUpdate vehicle={vehicle} {...shared} {...(autoFocus ? { autoFocus } : {})} />}
      {meters.map((m, i) => (
        <HoursQuickUpdate key={m.id} meter={m} {...shared} {...(autoFocus && !showOdometer && i === 0 ? { autoFocus } : {})} />
      ))}
    </div>
  )
}
