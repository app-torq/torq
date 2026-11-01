import { Link } from 'react-router'
import { useTranslation } from 'react-i18next'
import { BRAND } from '../lib/brand'
import logoMark from '../assets/torq-logo.svg'
import logoWordmark from '../assets/torq-logo-w-title.svg'

/** The torq logo: the mark alone, or with the "torq" wordmark. Size it with a height class. */
export function Logo({ wordmark = true, className = '' }: { wordmark?: boolean; className?: string }) {
  return <img src={wordmark ? logoWordmark : logoMark} alt={BRAND} className={`block w-auto ${className}`} />
}

/** The logo as a link to the start page (dashboard). */
export function HomeLink({ wordmark = true, className = '', onClick }: { wordmark?: boolean; className?: string; onClick?: () => void }) {
  const { t } = useTranslation('common')
  return (
    <Link to="/" aria-label={t('nav.home', { brand: BRAND })} title={t('nav.home', { brand: BRAND })} onClick={onClick} className="focus-ring inline-flex rounded-md">
      <img src={wordmark ? logoWordmark : logoMark} alt="" className={`block w-auto ${className}`} />
    </Link>
  )
}
