import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { useSession } from './session'
import { ErrorBanner } from '../components/ui'

export function RequireAdmin({ children }: { children: ReactNode }) {
  const { user } = useSession()
  const { t } = useTranslation('errors')
  if (user.role !== 'admin') return <ErrorBanner message={t('auth.forbidden')} />
  return <>{children}</>
}
