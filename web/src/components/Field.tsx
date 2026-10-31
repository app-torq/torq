import { useId, type InputHTMLAttributes, type ReactNode, type SelectHTMLAttributes, type TextareaHTMLAttributes } from 'react'
import { useTranslation } from 'react-i18next'

export const inputClass =
  'focus-ring block w-full rounded-md border border-zinc-700 bg-zinc-950 px-3 py-2 text-sm text-zinc-100 placeholder:text-zinc-400 disabled:text-zinc-400 aria-[invalid=true]:border-red-500'

/** Briefly marks a field that a catalog lookup just filled (vehicle form). */
export const autoFilledClass = 'border-amber-500/70 bg-amber-500/10 ring-2 ring-amber-500/50 transition-colors duration-500'

export function fieldInputClass(highlighted?: boolean): string {
  return highlighted ? `${inputClass} ${autoFilledClass}` : `${inputClass} transition-colors duration-500`
}

interface FieldProps {
  label: ReactNode
  /** Error code from the API (translated via the errors namespace) or a ready message. */
  error?: string | undefined
  hint?: ReactNode
  optional?: boolean
  className?: string
  children: (props: { id: string; 'aria-invalid': boolean; 'aria-describedby'?: string }) => ReactNode
}

export function Field({ label, error, hint, optional, className = '', children }: FieldProps) {
  const id = useId()
  const { t } = useTranslation(['errors', 'common'])
  const describedBy = error || hint ? `${id}-desc` : undefined
  const message = error ? t(error, { ns: 'errors', defaultValue: error }) : undefined
  return (
    <div className={`space-y-1.5 ${className}`}>
      <label htmlFor={id} className="block text-sm font-medium text-zinc-300">
        {label}
        {optional && <span className="ml-1.5 font-normal text-zinc-400">({t('common:state.optional')})</span>}
      </label>
      {children(describedBy ? { id, 'aria-invalid': !!error, 'aria-describedby': describedBy } : { id, 'aria-invalid': !!error })}
      {(message ?? hint) && (
        <p id={describedBy} className={`text-xs ${message ? 'text-red-400' : 'text-zinc-400'}`}>
          {message ?? hint}
        </p>
      )}
    </div>
  )
}

type TextFieldProps = Omit<InputHTMLAttributes<HTMLInputElement>, 'onChange' | 'value'> & {
  label: ReactNode
  value: string
  onChange: (value: string) => void
  error?: string | undefined
  hint?: ReactNode
  optional?: boolean
  highlighted?: boolean
}

export function TextField({ label, value, onChange, error, hint, optional, highlighted, className, ...rest }: TextFieldProps) {
  return (
    <Field label={label} error={error} hint={hint} optional={optional ?? false} className={className ?? ''}>
      {(a) => <input {...a} {...rest} value={value} onChange={(e) => { onChange(e.target.value) }} className={fieldInputClass(highlighted)} />}
    </Field>
  )
}

type TextAreaFieldProps = Omit<TextareaHTMLAttributes<HTMLTextAreaElement>, 'onChange' | 'value'> & {
  label: ReactNode
  value: string
  onChange: (value: string) => void
  error?: string | undefined
  hint?: ReactNode
  optional?: boolean
}

export function TextAreaField({ label, value, onChange, error, hint, optional, className, rows = 4, ...rest }: TextAreaFieldProps) {
  return (
    <Field label={label} error={error} hint={hint} optional={optional ?? false} className={className ?? ''}>
      {(a) => (
        <textarea {...a} {...rest} rows={rows} value={value} onChange={(e) => { onChange(e.target.value) }} className={`${inputClass} resize-y`} />
      )}
    </Field>
  )
}

type SelectFieldProps = Omit<SelectHTMLAttributes<HTMLSelectElement>, 'onChange' | 'value'> & {
  label: ReactNode
  value: string
  onChange: (value: string) => void
  options: readonly { value: string; label: string }[]
  error?: string | undefined
  hint?: ReactNode
  optional?: boolean
  highlighted?: boolean
}

export function SelectField({ label, value, onChange, options, error, hint, optional, highlighted, className, ...rest }: SelectFieldProps) {
  return (
    <Field label={label} error={error} hint={hint} optional={optional ?? false} className={className ?? ''}>
      {(a) => (
        <select {...a} {...rest} value={value} onChange={(e) => { onChange(e.target.value) }} className={fieldInputClass(highlighted)}>
          {options.map((o) => (
            <option key={o.value} value={o.value}>
              {o.label}
            </option>
          ))}
        </select>
      )}
    </Field>
  )
}

export function Checkbox({ label, checked, onChange, disabled }: { label: ReactNode; checked: boolean; onChange: (v: boolean) => void; disabled?: boolean }) {
  return (
    <label className="inline-flex cursor-pointer items-center gap-2 text-sm text-zinc-200">
      <input
        type="checkbox"
        className="focus-ring h-4 w-4 rounded border-zinc-600 bg-zinc-950 accent-sky-500"
        checked={checked}
        disabled={disabled}
        onChange={(e) => { onChange(e.target.checked) }}
      />
      {label}
    </label>
  )
}
