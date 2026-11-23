import { useContext } from 'react'
import { useQuery } from '@tanstack/react-query'
import { api } from '../lib/api'
import { queryKeys } from '../lib/queryKeys'
import { BRAND } from '../lib/brand'
import type { Branding, ServerInfo } from '../lib/types'
import { SessionContext } from './session'

/** The public server info; signed-out screens read the branding from it. */
export function useServerInfo(enabled = true) {
  return useQuery({ queryKey: queryKeys.serverInfo, queryFn: () => api.get<ServerInfo>('/instance'), enabled, staleTime: 60_000 })
}

/** The instance name and logo: from the session when signed in, else from GET /instance. */
export function useBranding(): Branding | undefined {
  const session = useContext(SessionContext)
  const info = useServerInfo(!session)
  return session?.instance.branding ?? info.data?.branding
}

/** The browser title: "<name> · torq", or "torq" without a name. */
export function documentTitle(name: string | undefined): string {
  return name ? `${name} · ${BRAND}` : BRAND
}
