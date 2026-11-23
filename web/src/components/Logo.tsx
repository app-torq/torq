import { Link } from 'react-router'
import { useTranslation } from 'react-i18next'
import { BRAND } from '../lib/brand'
import { useBranding } from '../app/branding'
import logoMark from '../assets/torq-logo.svg'
import logoWordmark from '../assets/torq-logo-w-title.svg'

/**
 * The instance logo and name set in the settings, or the torq logo. The
 * instance logo takes the place of the torq mark and the name that of the
 * wordmark, so a logo without a name shows on its own.
 */
function Mark({ className, label }: { className: string; label: string }) {
  const branding = useBranding()
  const name = branding?.name ?? ''
  const logo = branding?.logo_url ?? null
  if (!name && !logo) return <img src={logoWordmark} alt={label} className={`block w-auto ${className}`} />
  if (!name) return <img src={logo ?? logoMark} alt={label} className={`block w-auto max-w-full object-contain ${className}`} />
  return (
    <span className="flex min-w-0 items-center gap-2.5">
      <img src={logo ?? logoMark} alt="" className={`block w-auto max-w-28 shrink-0 object-contain ${className}`} />
      <span className="truncate text-lg font-semibold text-zinc-100">{name}</span>
    </span>
  )
}

/** The torq logo regardless of branding, for screens shown when the server is unreachable. */
export function TorqLogo({ className = '' }: { className?: string }) {
  return <img src={logoWordmark} alt={BRAND} className={`block w-auto ${className}`} />
}

/** The instance or torq logo, without a link. Size it with a height class. */
export function Logo({ className = '' }: { className?: string }) {
  const branding = useBranding()
  return <Mark className={className} label={branding?.name || BRAND} />
}

/** The logo as a link to the start page (dashboard). */
export function HomeLink({ className = '', onClick }: { className?: string; onClick?: () => void }) {
  const { t } = useTranslation('common')
  const branding = useBranding()
  const label = t('nav.home', { brand: branding?.name || BRAND })
  return (
    <Link to="/" aria-label={label} title={label} onClick={onClick} className="focus-ring inline-flex min-w-0 max-w-full rounded-md">
      <Mark className={className} label="" />
    </Link>
  )
}
