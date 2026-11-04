import { WifiOff } from 'lucide-react'
import i18n from '../../i18n'
import { Logo } from '../../components/Logo'
import { Button } from '../../components/Button'

/**
 * Shown when the torq server can't be reached at start (specs/004-installable-pwa,
 * research W7). The user's language setting lives on the server, so this
 * screen follows the browser's language.
 */
export function OfflineScreen({ onRetry }: { onRetry: () => void }) {
  const lang = navigator.languages[0]?.toLowerCase().startsWith('de') ? 'de' : 'en'
  const t = i18n.getFixedT(lang, 'common')
  return (
    <div lang={lang} className="flex min-h-dvh flex-col items-center justify-center gap-6 p-6 text-center">
      <Logo />
      <div className="max-w-sm space-y-2">
        <WifiOff size={28} aria-hidden className="mx-auto text-zinc-500" />
        <h1 className="text-lg font-semibold text-zinc-100">{t('offline.title')}</h1>
        <p className="text-sm text-zinc-400">{t('offline.body', { host: window.location.host })}</p>
      </div>
      <Button variant="primary" onClick={onRetry}>{t('offline.retry')}</Button>
    </div>
  )
}
