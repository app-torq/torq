import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Smartphone, X } from 'lucide-react'
import { Button, IconButton } from '../../components/Button'
import { isIOS, isStandalone, isTouchDevice, useInstallPrompt } from './install'

const dismissedKey = 'torq.installHint.dismissed'

function readDismissed(): boolean {
  try {
    return localStorage.getItem(dismissedKey) === '1'
  } catch {
    return false
  }
}

/**
 * Suggests installing torq on phones and tablets that don't run it as an app
 * yet (FR-016). Dismissing it hides it on this device for good.
 */
export function InstallHint() {
  const { t } = useTranslation()
  const install = useInstallPrompt()
  const [dismissed, setDismissed] = useState(readDismissed)
  if (dismissed || !isTouchDevice() || isStandalone()) return null

  const dismiss = () => {
    try {
      localStorage.setItem(dismissedKey, '1')
    } catch {
      // Storage blocked: the hint just comes back next time.
    }
    setDismissed(true)
  }
  const ios = isIOS()

  return (
    <section aria-label={t('install.title')} className="flex items-start gap-3 rounded-lg border border-sky-500/30 bg-sky-500/10 p-4 text-sm text-sky-100">
      <Smartphone size={20} aria-hidden className="mt-0.5 shrink-0 text-sky-300" />
      <div className="flex-1 space-y-1">
        <p className="font-medium text-sky-50">{t('install.title')}</p>
        <p>{t('install.hint')}</p>
        <p>{ios ? t('install.ios') : t('install.android')}</p>
        {ios && <p className="text-sky-200/80">{t('install.iosNote')}</p>}
        {install && (
          <Button size="sm" variant="primary" className="mt-2" onClick={install}>{t('install.install')}</Button>
        )}
      </div>
      <IconButton aria-label={t('install.dismiss')} onClick={dismiss}>
        <X size={16} />
      </IconButton>
    </section>
  )
}
