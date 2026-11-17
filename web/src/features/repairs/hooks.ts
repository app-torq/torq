import { useQuery, type QueryClient } from '@tanstack/react-query'
import { api } from '../../lib/api'
import { queryKeys } from '../../lib/queryKeys'
import type { ListResponse, Repair, RepairStatus, RepairSummary } from '../../lib/types'

export const repairStatuses: RepairStatus[] = ['planned', 'in_progress', 'waiting_on_parts', 'completed']
export const statusTone = { planned: 'zinc', in_progress: 'sky', waiting_on_parts: 'amber', completed: 'emerald' } as const

/** List orders offered in the project list; the board always sorts by priority. */
export const repairSorts = ['priority', 'status', 'updated'] as const
export type RepairSort = (typeof repairSorts)[number]

export function useRepairs(filters: { vehicle_id?: number | undefined; status?: string[]; priority?: string[]; tag?: string[]; sort?: RepairSort; q?: string }) {
  return useQuery({
    queryKey: queryKeys.repairList(filters),
    queryFn: () =>
      api.get<ListResponse<RepairSummary>>('/repairs', {
        vehicle_id: filters.vehicle_id, status: filters.status, priority: filters.priority, tag: filters.tag, sort: filters.sort, q: filters.q, limit: 200,
      }),
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
    // Saving a project can create tags.
    qc.invalidateQueries({ queryKey: queryKeys.tags }),
    qc.invalidateQueries({ queryKey: queryKeys.dashboard }),
    qc.invalidateQueries({ queryKey: queryKeys.reports }),
    ...(id ? [qc.invalidateQueries({ queryKey: queryKeys.repair(id) })] : []),
  ])
}
