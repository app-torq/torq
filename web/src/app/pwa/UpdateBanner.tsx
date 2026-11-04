import { useTranslation } from 'react-i18next'
import { RefreshCw } from 'lucide-react'
import { Button } from '../../components/Button'
import { getUpdater, useUpdatePending } from './register'

/**
 * Offers the new version after a server upgrade. Without unsaved input the
 * update also applies on the next navigation; this banner covers staying on
 * one page, and pages with input that must not be lost.
 */
export function UpdateBanner() {
  const { t } = useTranslation()
  const pending = useUpdatePending()
  if (!pending) return null
  return (
    <div role="status" className="fixed inset-x-0 bottom-0 z-40 flex justify-center p-3">
      <div className="flex max-w-xl flex-wrap items-center gap-3 rounded-md border border-sky-500/30 bg-zinc-900 px-4 py-2 text-sm text-sky-100 shadow-lg">
        <RefreshCw size={16} aria-hidden className="shrink-0 text-sky-300" />
        <span className="flex-1">{t('update.available')}</span>
        <Button size="sm" variant="primary" onClick={() => { getUpdater()?.applyNow() }}>{t('update.reloadNow')}</Button>
      </div>
    </div>
  )
}
