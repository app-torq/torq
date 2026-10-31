import { useEffect, useMemo, type ReactNode } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { api, isApiError, UNAUTHENTICATED_EVENT } from '../lib/api'
import { queryKeys } from '../lib/queryKeys'
import type { MeResponse } from '../lib/types'
import { useErrorText } from '../lib/errors'
import { ErrorBanner, Loading } from '../components/ui'
import { SessionContext } from './session'
import { SetupPage } from '../features/auth/SetupPage'
import { LoginPage } from '../features/auth/LoginPage'

/** Shows first-boot setup or login until a session exists, then renders the app. */
export function AuthGate({ children }: { children: ReactNode }) {
  const qc = useQueryClient()
  const { i18n } = useTranslation()
  const errorText = useErrorText()

  const setup = useQuery({
    queryKey: queryKeys.setupStatus,
    queryFn: () => api.get<{ setup_required: boolean }>('/setup/status'),
    staleTime: Infinity,
  })
  const me = useQuery({
    queryKey: queryKeys.me,
    queryFn: async () => {
      try {
        return await api.get<MeResponse>('/auth/me')
      } catch (err) {
        if (isApiError(err) && err.status === 401) return null
        throw err
      }
    },
    enabled: setup.data?.setup_required === false,
    retry: false,
    staleTime: 60_000,
  })

  useEffect(() => {
    const onUnauthenticated = () => {
      qc.setQueryData(queryKeys.me, null)
      qc.removeQueries({ predicate: (q) => q.queryKey[0] !== queryKeys.me[0] && q.queryKey[0] !== queryKeys.setupStatus[0] })
    }
    window.addEventListener(UNAUTHENTICATED_EVENT, onUnauthenticated)
    return () => { window.removeEventListener(UNAUTHENTICATED_EVENT, onUnauthenticated) }
  }, [qc])

  const locale = me.data?.user.locale
  useEffect(() => {
    if (locale && i18n.language !== locale) void i18n.changeLanguage(locale)
    document.documentElement.lang = locale ?? i18n.language
  }, [locale, i18n])

  const session = useMemo(() => (me.data ? { user: me.data.user, instance: me.data.instance } : null), [me.data])

  if (setup.isPending) return <Centered><Loading /></Centered>
  if (setup.isError) return <Centered><ErrorBanner message={errorText(setup.error)} onRetry={() => void setup.refetch()} /></Centered>
  if (setup.data.setup_required) return <SetupPage />
  if (me.isPending) return <Centered><Loading /></Centered>
  if (me.isError) return <Centered><ErrorBanner message={errorText(me.error)} onRetry={() => void me.refetch()} /></Centered>
  if (!session) return <LoginPage />
  return <SessionContext.Provider value={session}>{children}</SessionContext.Provider>
}

export function Centered({ children }: { children: ReactNode }) {
  return <div className="flex min-h-dvh items-center justify-center p-4">{children}</div>
}
