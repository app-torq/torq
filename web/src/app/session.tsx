import { createContext, useContext } from 'react'
import type { InstanceInfo, User } from '../lib/types'

export interface Session {
  user: User
  instance: InstanceInfo
}

export const SessionContext = createContext<Session | null>(null)

/** Returns the signed-in session; only call inside the authenticated app. */
export function useSession(): Session {
  const s = useContext(SessionContext)
  if (!s) throw new Error('useSession outside of an authenticated session')
  return s
}
