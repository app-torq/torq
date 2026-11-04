import { useTranslation } from 'react-i18next'
import { useNavigate } from 'react-router'
import { ArrowLeft } from 'lucide-react'

/**
 * Goes back to the previous screen, or to fallback when torq was opened right
 * here. Installed apps have no browser back button (specs/004-installable-pwa,
 * FR-005).
 */
export function BackLink({ fallback }: { fallback: string }) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  // React Router keeps the position in its own history stack in state.idx.
  const canGoBack = ((window.history.state as { idx?: number } | null)?.idx ?? 0) > 0
  return (
    <button
      type="button"
      onClick={() => void (canGoBack ? navigate(-1) : navigate(fallback))}
      className="focus-ring inline-flex items-center gap-1 rounded text-sm text-sky-300 hover:underline"
    >
      <ArrowLeft size={15} aria-hidden />
      {t('common:actions.back')}
    </button>
  )
}
