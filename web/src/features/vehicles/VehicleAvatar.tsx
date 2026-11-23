import { Car, Motorbike } from 'lucide-react'
import type { Vehicle } from '../../lib/types'

export function VehicleAvatar({ vehicle, size = 48 }: { vehicle: Pick<Vehicle, 'avatar_url' | 'make' | 'vehicle_type'>; size?: number }) {
  const style = { width: size, height: size }
  if (vehicle.avatar_url) {
    return <img src={vehicle.avatar_url} alt="" style={style} loading="lazy" className="shrink-0 rounded-md border border-zinc-800 object-cover" />
  }
  // Without a photo, the icon tells cars and motorcycles apart.
  const Icon = vehicle.vehicle_type === 'motorcycle' ? Motorbike : Car
  return (
    <div style={style} className="flex shrink-0 items-center justify-center rounded-md border border-zinc-800 bg-zinc-950 text-zinc-600" aria-hidden>
      <Icon size={Math.round(size * 0.5)} />
    </div>
  )
}
