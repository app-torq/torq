import { useTranslation } from 'react-i18next'
import { SelectField } from '../../components/Field'
import { priorityName, useRepairPriorities } from './priorities'

/**
 * Picks a project's priority. value is a priority id as text, '' for none,
 * or null for "not chosen yet", which shows the garage's default.
 */
export function PrioritySelect({ value, onChange, error }: { value: string | null; onChange: (v: string) => void; error?: string | undefined }) {
  const { t } = useTranslation('repairs')
  const priorities = useRepairPriorities()
  const items = priorities.data?.items ?? []
  const shown = value ?? String(items.find((p) => p.is_default)?.id ?? '')
  return (
    <SelectField
      label={t('fields.priority')}
      value={shown}
      onChange={onChange}
      error={error}
      options={[...items.map((p) => ({ value: String(p.id), label: priorityName(p, t) })), { value: '', label: t('priorities.none') }]}
    />
  )
}

/** The request value for a PrioritySelect: omitted while untouched, null for none. */
export function priorityBody(value: string | null): { priority_id?: number | null } {
  if (value === null) return {}
  return { priority_id: value === '' ? null : Number(value) }
}
