import { useState, type KeyboardEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { X } from 'lucide-react'
import { Field, inputClass } from '../../components/Field'

/** Chip input for OBD2 fault codes; codes are uppercased. */
export function FaultCodeInput({ value, onChange }: { value: string[]; onChange: (codes: string[]) => void }) {
  const { t } = useTranslation('repairs')
  const [draft, setDraft] = useState('')
  const commit = () => {
    const codes = draft.split(/[\s,;]+/).map((c) => c.trim().toUpperCase()).filter(Boolean)
    if (codes.length) onChange(Array.from(new Set([...value, ...codes])))
    setDraft('')
  }
  const onKey = (e: KeyboardEvent<HTMLInputElement>) => {
    if (e.key === 'Enter' || e.key === ',') {
      e.preventDefault()
      commit()
    } else if (e.key === 'Backspace' && draft === '' && value.length > 0) {
      onChange(value.slice(0, -1))
    }
  }
  return (
    <Field label={t('fields.faultCodes')} hint={t('fields.faultCodesHint')} optional>
      {(a) => (
        <div className="flex flex-wrap items-center gap-1.5 rounded-md border border-zinc-700 bg-zinc-950 p-1.5">
          {value.map((c) => (
            <span key={c} className="inline-flex items-center gap-1 rounded bg-amber-500/15 px-1.5 py-0.5 font-mono text-xs text-amber-200">
              {c}
              <button type="button" aria-label={`${t('common:actions.remove')} ${c}`} className="text-amber-300 hover:text-amber-100" onClick={() => { onChange(value.filter((x) => x !== c)) }}>
                <X size={12} />
              </button>
            </span>
          ))}
          <input {...a} value={draft} onChange={(e) => { setDraft(e.target.value) }} onKeyDown={onKey} onBlur={commit} className={`${inputClass} min-w-24 flex-1 border-0 bg-transparent p-1 font-mono uppercase`} />
        </div>
      )}
    </Field>
  )
}
