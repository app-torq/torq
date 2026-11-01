import { useQuery, type QueryClient } from '@tanstack/react-query'
import { api } from '../../lib/api'
import { odometerDependentKeys, queryKeys } from '../../lib/queryKeys'
import type { ListResponse, Vehicle } from '../../lib/types'

export type ArchiveFilter = 'false' | 'true' | 'all'

export function useVehicles(archived: ArchiveFilter = 'false') {
  return useQuery({
    queryKey: queryKeys.vehicleList(archived),
    queryFn: () => api.get<ListResponse<Vehicle>>('/vehicles', { archived }),
  })
}

export function useVehicle(id: number) {
  return useQuery({
    queryKey: queryKeys.vehicle(id),
    queryFn: () => api.get<{ vehicle: Vehicle }>(`/vehicles/${id}`).then((r) => r.vehicle),
  })
}

/** Invalidates everything that depends on odometer readings or service history. */
export function invalidateOdometerDependents(qc: QueryClient): Promise<unknown> {
  return Promise.all(odometerDependentKeys.map((queryKey) => qc.invalidateQueries({ queryKey })))
}
