import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { LogOut } from 'lucide-react'
import { api } from '../lib/api'
import { queryKeys } from '../lib/queryKeys'
import { Button } from '../components/Button'
import { Logo } from '../components/Logo'
import { SecurityPage } from '../features/settings/SecurityPage'

/** Shown instead of the app while the user must set up a second factor first. */
export function MfaSetupGate() {
  const { t } = useTranslation()
  const qc = useQueryClient()
  const logout = useMutation({
    mutationFn: () => api.post('/auth/logout'),
    onSettled: () => {
      qc.clear()
      qc.setQueryData(queryKeys.setupStatus, { setup_required: false })
      qc.setQueryData(queryKeys.me, null)
    },
  })
  return (
    <div className="mx-auto max-w-3xl px-4 py-6">
      <div className="mb-6 flex items-center justify-between gap-4">
        <Logo className="h-8" />
        <Button size="sm" onClick={() => { logout.mutate() }}>
          <LogOut size={15} />
          {t('actions.logout')}
        </Button>
      </div>
      <SecurityPage />
    </div>
  )
}
