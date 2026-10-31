import type { InputHTMLAttributes, ReactNode } from 'react'
import { Field, inputClass } from './Field'

type NumberFieldProps = Omit<InputHTMLAttributes<HTMLInputElement>, 'onChange' | 'value' | 'type'> & {
  label: ReactNode
  /** Raw text as typed; parse with parseNumber() using the user's number format. */
  value: string
  onChange: (value: string) => void
  unit?: string
  error?: string | undefined
  hint?: ReactNode
  optional?: boolean
  decimal?: boolean
}

/** Text input for locale-formatted numbers (accepts "6,5" or "6.5") with a metric unit suffix. */
export function NumberField({ label, value, onChange, unit, error, hint, optional, decimal = true, className, ...rest }: NumberFieldProps) {
  return (
    <Field label={label} error={error} hint={hint} optional={optional ?? false} className={className ?? ''}>
      {(a) => (
        <div className="relative">
          <input
            {...a}
            {...rest}
            type="text"
            inputMode={decimal ? 'decimal' : 'numeric'}
            autoComplete="off"
            value={value}
            onChange={(e) => { onChange(e.target.value) }}
            className={`${inputClass} tabular-nums ${unit ? 'pr-12' : ''}`}
          />
          {unit && (
            <span className="pointer-events-none absolute inset-y-0 right-3 flex items-center text-sm text-zinc-400">{unit}</span>
          )}
        </div>
      )}
    </Field>
  )
}
