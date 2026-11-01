import { Link } from 'react-router'
import { useTranslation } from 'react-i18next'
import { EmptyState } from '../components/ui'

export function NotFoundPage() {
  const { t } = useTranslation(['errors', 'common'])
  return (
    <EmptyState
      title={t('not_found')}
      action={<Link to="/" className="text-sm text-sky-400 hover:underline">{t('common:actions.back')}</Link>}
    />
  )
}
