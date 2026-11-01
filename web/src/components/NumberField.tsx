import type { InputHTMLAttributes, ReactNode } from 'react'
import { Field, fieldInputClass } from './Field'

type NumberFieldProps = Omit<InputHTMLAttributes<HTMLInputElement>, 'onChange' | 'value' | 'type'> & {
  label: ReactNode
  labelAside?: ReactNode
  /** Raw text as typed; parse with parseNumber() using the user's number format. */
  value: string
  onChange: (value: string) => void
  unit?: string
  error?: string | undefined
  hint?: ReactNode
  optional?: boolean
  decimal?: boolean
  highlighted?: boolean
}

/** Text input for locale-formatted numbers (accepts "6,5" or "6.5") with a metric unit suffix. */
export function NumberField({ label, labelAside, value, onChange, unit, error, hint, optional, decimal = true, highlighted, className, ...rest }: NumberFieldProps) {
  return (
    <Field label={label} labelAside={labelAside} error={error} hint={hint} optional={optional ?? false} className={className ?? ''}>
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
            className={`${fieldInputClass(highlighted)} tabular-nums ${unit ? 'pr-12' : ''}`}
          />
          {unit && (
            <span className="pointer-events-none absolute inset-y-0 right-3 flex items-center text-sm text-zinc-400">{unit}</span>
          )}
        </div>
      )}
    </Field>
  )
}

type NumberRangeFieldProps = {
  label: ReactNode
  labelAside?: ReactNode
  /** Lower value (or the only one) and optional upper value, raw text as typed. */
  from: string
  to: string
  onFromChange: (value: string) => void
  onToChange: (value: string) => void
  /** Accessible name of the upper input, e.g. "Front tyre pressure, up to". */
  toLabel: string
  toPlaceholder?: string
  unit?: string
  error?: string | undefined
  hint?: ReactNode
  optional?: boolean
}

/** A single number or a range ("2,2 – 2,4 bar"): two inputs sharing one label, one cell and one unit. */
export function NumberRangeField({ label, labelAside, from, to, onFromChange, onToChange, toLabel, toPlaceholder, unit, error, hint, optional }: NumberRangeFieldProps) {
  return (
    <Field label={label} labelAside={labelAside} error={error} hint={hint} optional={optional ?? false}>
      {(a) => (
        <div className="flex items-center gap-1.5">
          <input
            {...a}
            type="text"
            inputMode="decimal"
            autoComplete="off"
            value={from}
            onChange={(e) => { onFromChange(e.target.value) }}
            className={`${fieldInputClass()} min-w-0 tabular-nums`}
          />
          <span aria-hidden className="text-zinc-500">–</span>
          <input
            type="text"
            inputMode="decimal"
            autoComplete="off"
            aria-label={toLabel}
            aria-invalid={a['aria-invalid']}
            aria-describedby={a['aria-describedby']}
            placeholder={toPlaceholder}
            value={to}
            onChange={(e) => { onToChange(e.target.value) }}
            className={`${fieldInputClass()} min-w-0 tabular-nums`}
          />
          {unit && <span className="shrink-0 text-sm text-zinc-400">{unit}</span>}
        </div>
      )}
    </Field>
  )
}
