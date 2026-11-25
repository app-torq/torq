import { describe, expect, it } from 'vitest'
import { renderHook } from '@testing-library/react'
import type { ReactNode } from 'react'
import { SessionContext, type Session } from '../app/session'
import { testSession } from '../test/render'
import { canOn, useCan } from './permissions'

function wrapper(session: Session) {
  return ({ children }: { children: ReactNode }) => <SessionContext.Provider value={session}>{children}</SessionContext.Provider>
}

describe('permissions', () => {
  it('reads per-vehicle permissions from the vehicle', () => {
    expect(canOn({ permissions: ['service_logs.write'] }, 'service_logs.write')).toBe(true)
    expect(canOn({ permissions: [] }, 'service_logs.write')).toBe(false)
    expect(canOn(undefined, 'vehicles.edit')).toBe(false)
  })

  it('grants everything to administrators and the role permissions to others', () => {
    expect(renderHook(() => useCan('catalog.update'), { wrapper: wrapper(testSession) }).result.current).toBe(true)
    const viewer: Session = { ...testSession, user: { ...testSession.user, role: 'member', role_builtin_key: 'viewer', permissions: ['categories.manage'] } }
    expect(renderHook(() => useCan('categories.manage'), { wrapper: wrapper(viewer) }).result.current).toBe(true)
    expect(renderHook(() => useCan('vehicles.create'), { wrapper: wrapper(viewer) }).result.current).toBe(false)
  })
})
