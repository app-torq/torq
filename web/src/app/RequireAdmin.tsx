import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { useSession } from './session'
import { useCan } from '../lib/permissions'
import type { Permission } from '../lib/types'
import { ErrorBanner } from '../components/ui'

/** Shown in place of a page the user may not use. */
export function Forbidden() {
  const { t } = useTranslation('errors')
  return <ErrorBanner message={t('auth.forbidden')} />
}

export function RequireAdmin({ children }: { children: ReactNode }) {
  const { user } = useSession()
  if (user.role !== 'admin') return <Forbidden />
  return <>{children}</>
}

/** Shows children only to users with a garage-wide permission. */
export function RequirePermission({ permission, children }: { permission: Permission; children: ReactNode }) {
  if (!useCan(permission)) return <Forbidden />
  return <>{children}</>
}
