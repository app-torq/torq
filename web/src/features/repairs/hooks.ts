import { useQuery, type QueryClient } from '@tanstack/react-query'
import { api } from '../../lib/api'
import { queryKeys } from '../../lib/queryKeys'
import type { ListResponse, Repair, RepairStatus, RepairSummary } from '../../lib/types'

export const repairStatuses: RepairStatus[] = ['planned', 'in_progress', 'waiting_on_parts', 'completed']
export const statusTone = { planned: 'zinc', in_progress: 'sky', waiting_on_parts: 'amber', completed: 'emerald' } as const

export function useRepairs(filters: { vehicle_id?: number | undefined; status?: string[]; q?: string }) {
  return useQuery({
    queryKey: queryKeys.repairList(filters),
    queryFn: () => api.get<ListResponse<RepairSummary>>('/repairs', { vehicle_id: filters.vehicle_id, status: filters.status, q: filters.q, limit: 200 }),
  })
}

export function useRepair(id: number) {
  return useQuery({
    queryKey: queryKeys.repair(id),
    queryFn: () => api.get<{ repair: Repair }>(`/repairs/${id}`).then((r) => r.repair),
  })
}

export function invalidateRepairs(qc: QueryClient, id?: number): Promise<unknown> {
  return Promise.all([
    qc.invalidateQueries({ queryKey: queryKeys.repairs }),
    qc.invalidateQueries({ queryKey: queryKeys.dashboard }),
    qc.invalidateQueries({ queryKey: queryKeys.reports }),
    ...(id ? [qc.invalidateQueries({ queryKey: queryKeys.repair(id) })] : []),
  ])
}
