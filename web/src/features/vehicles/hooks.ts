import { useQuery, type QueryClient } from '@tanstack/react-query'
import { api } from '../../lib/api'
import { odometerDependentKeys, queryKeys } from '../../lib/queryKeys'
import type { DistanceUnit, ListResponse, Permission, Vehicle } from '../../lib/types'
import { canOn } from '../../lib/permissions'
import { useSession } from '../../app/session'

export type ArchiveFilter = 'false' | 'true' | 'all'

export function useVehicles(archived: ArchiveFilter = 'false') {
  return useQuery({
    queryKey: queryKeys.vehicleList(archived),
    queryFn: () => api.get<ListResponse<Vehicle>>('/vehicles', { archived }),
  })
}

/** Whether some active vehicle allows p, e.g. to offer "new service entry" at all. */
export function useAnyVehicleCan(p: Permission): boolean {
  const vehicles = useVehicles('false')
  return (vehicles.data?.items ?? []).some((v) => canOn(v, p))
}

export function useVehicle(id: number) {
  return useQuery({
    queryKey: queryKeys.vehicle(id),
    queryFn: () => api.get<{ vehicle: Vehicle }>(`/vehicles/${id}`).then((r) => r.vehicle),
  })
}

/**
 * Looks up the odometer unit of a vehicle for distances shown outside its
 * pages; falls back to the user's distance unit while vehicles load.
 */
export function useOdometerUnit(): (vehicleId: number) => DistanceUnit {
  const { user } = useSession()
  const vehicles = useVehicles('all')
  const byId = new Map(vehicles.data?.items.map((v) => [v.id, v.odometer_unit]))
  return (id) => byId.get(id) ?? user.distance_unit
}

/** Invalidates everything that depends on odometer readings or service history. */
export function invalidateOdometerDependents(qc: QueryClient): Promise<unknown> {
  return Promise.all(odometerDependentKeys.map((queryKey) => qc.invalidateQueries({ queryKey })))
}
