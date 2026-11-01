import { useId, useRef, useState, type ClipboardEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { Undo2, Zap } from 'lucide-react'
import type { KbaVariant } from '../../lib/types'
import { useFormat } from '../../lib/useFormat'
import { useQueryClient } from '@tanstack/react-query'
import { HSN_RE, TSN_RE, kbaLookupOptions, useCatalogLang } from './catalog'

interface Props {
  hsn: string
  tsn: string
  onChange: (hsn: string, tsn: string) => void
  /** Called once per looked-up key with the single match or the picked variant. */
  onFill: (variant: KbaVariant) => void
  canUndo: boolean
  onUndo: () => void
  hsnError?: string | undefined
  tsnError?: string | undefined
}

// "0710/ABC123", "0710 abc", "0710-ABC" → HSN 0710, TSN ABC (check digits dropped).
const PASTE_RE = /^\s*(\d{4})[\s/\-,;:.]*([0-9a-z]{3})/i

const pillClass =
  'focus-ring rounded-full border px-3 py-1 text-left text-xs transition-colors'

/**
 * Quick-Fill bar for the German registration certificate (Zulassungsbescheinigung
 * Teil I, fields 2.1 and 2.2). Lookups run only after the user edits the key,
 * so opening a saved vehicle never overwrites its data.
 */
export function HsnTsnQuickFill({ hsn, tsn, onChange, onFill, canUndo, onUndo, hsnError, tsnError }: Props) {
  const { t } = useTranslation(['vehicles', 'errors'])
  const fmt = useFormat()
  const id = useId()
  const tsnRef = useRef<HTMLInputElement>(null)
  const qc = useQueryClient()
  const lang = useCatalogLang()
  // Latest requested key; answers for older keys are ignored.
  const latest = useRef<string | null>(null)
  const [loading, setLoading] = useState(false)
  const [result, setResult] = useState<{ key: string; variants: KbaVariant[] } | null>(null)
  const [resolved, setResolved] = useState<string | null>(null)
  const [dismissed, setDismissed] = useState<string | null>(null)

  const key = `${hsn}/${tsn}`
  const variants = result?.key === key ? result.variants : undefined

  // Lookups start only from user edits, so opening a saved vehicle never
  // overwrites its data; the 7th character triggers the lookup (FR-005).
  const edit = (nextHsn: string, nextTsn: string) => {
    onChange(nextHsn, nextTsn)
    const nextKey = `${nextHsn}/${nextTsn}`
    latest.current = nextKey
    setDismissed(null)
    if (!HSN_RE.test(nextHsn) || !TSN_RE.test(nextTsn)) {
      setResult(null)
      setLoading(false)
      return
    }
    setLoading(true)
    qc.query(kbaLookupOptions(nextHsn, nextTsn, lang))
      .then((items) => {
        if (latest.current !== nextKey) return
        setResult({ key: nextKey, variants: items })
        const [only] = items
        if (items.length === 1 && only) {
          setResolved(nextKey)
          onFill(only)
        }
      })
      .catch(() => {
        // Lookup failures never block the form (FR-018); treat as not found.
        if (latest.current === nextKey) setResult({ key: nextKey, variants: [] })
      })
      .finally(() => {
        if (latest.current === nextKey) setLoading(false)
      })
  }

  const onHsn = (raw: string) => {
    const next = raw.replace(/\D/g, '').slice(0, 4)
    edit(next, tsn)
    if (next.length === 4 && hsn.length < 4) tsnRef.current?.focus()
  }

  const onTsn = (raw: string) => {
    edit(hsn, raw.toUpperCase().replace(/[^0-9A-Z]/g, '').slice(0, 3))
  }

  const onPaste = (e: ClipboardEvent<HTMLInputElement>) => {
    const m = PASTE_RE.exec(e.clipboardData.getData('text'))
    if (!m?.[1] || !m[2]) return
    e.preventDefault()
    edit(m[1], m[2].toUpperCase())
    tsnRef.current?.focus()
  }

  const pick = (v: KbaVariant) => {
    setResolved(key)
    onFill(v)
  }

  const showPills = !!variants && variants.length > 1 && resolved !== key && dismissed !== key
  const notFound = !!variants && variants.length === 0

  const describe = (v: KbaVariant) =>
    [
      v.model && v.model.toUpperCase() !== v.trade_name.toUpperCase() ? v.model : null,
      v.body_code ? t(`body.${v.body_code}`, { defaultValue: v.body_code }) : null,
      v.year_from ? String(v.year_from) : null,
      v.power_kw != null ? fmt.power(v.power_kw) : null,
      v.displacement_cc != null ? fmt.displacement(v.displacement_cc) : null,
      v.fuel_type ? t(`fuel.${v.fuel_type}`) : null,
    ]
      .filter(Boolean)
      .join(' · ')

  const input =
    'focus-ring block rounded-md border border-zinc-700 bg-zinc-950 px-3 py-2 text-center font-mono text-sm uppercase tracking-widest text-zinc-100 placeholder:text-zinc-500 aria-[invalid=true]:border-red-500'

  return (
    <section aria-labelledby={`${id}-title`} className="rounded-lg border border-zinc-800 bg-zinc-900 p-3 sm:p-4">
      <div className="flex flex-wrap items-end gap-3">
        <div className="min-w-0 flex-1 basis-full sm:basis-auto">
          <h2 id={`${id}-title`} className="flex items-center gap-1.5 text-sm font-medium text-zinc-100">
            <Zap aria-hidden className="h-4 w-4 text-amber-400" />
            {t('quickFill.title')}
          </h2>
          <p className="mt-0.5 text-xs text-zinc-400">{t('quickFill.intro')}</p>
        </div>
        <div className="space-y-1">
          <label htmlFor={`${id}-hsn`} className="block text-xs font-medium text-zinc-300">{t('quickFill.hsn')}</label>
          <input
            id={`${id}-hsn`}
            value={hsn}
            onChange={(e) => { onHsn(e.target.value) }}
            onPaste={onPaste}
            inputMode="numeric"
            autoComplete="off"
            placeholder="0000"
            maxLength={4}
            size={6}
            aria-invalid={!!hsnError}
            className={`${input} w-24`}
          />
        </div>
        <div className="space-y-1">
          <label htmlFor={`${id}-tsn`} className="block text-xs font-medium text-zinc-300">{t('quickFill.tsn')}</label>
          <input
            id={`${id}-tsn`}
            ref={tsnRef}
            value={tsn}
            onChange={(e) => { onTsn(e.target.value) }}
            onPaste={onPaste}
            autoComplete="off"
            autoCapitalize="characters"
            placeholder="AAA"
            maxLength={3}
            size={5}
            aria-invalid={!!tsnError}
            className={`${input} w-20`}
          />
        </div>
        {canUndo && (
          <button
            type="button"
            onClick={onUndo}
            className="focus-ring inline-flex items-center gap-1.5 rounded-md border border-zinc-700 px-3 py-2 text-sm text-zinc-200 hover:bg-zinc-800"
          >
            <Undo2 aria-hidden className="h-4 w-4" />
            {t('quickFill.undo')}
          </button>
        )}
      </div>

      {(hsnError ?? tsnError) && (
        <p className="mt-2 text-xs text-red-400">{t(hsnError ?? tsnError ?? '', { ns: 'errors' })}</p>
      )}

      <div aria-live="polite" className="text-xs">
        {loading && <p className="mt-2 text-zinc-400">{t('quickFill.searching')}</p>}
        {notFound && <p className="mt-2 text-zinc-400">{t('quickFill.notFound')}</p>}
        {variants && resolved === key && <p className="mt-2 text-zinc-400">{t('quickFill.filled')}</p>}
      </div>

      {showPills && (
        <div className="mt-3">
          <p id={`${id}-variants`} className="mb-2 text-xs text-zinc-300">{t('quickFill.chooseVariant', { count: variants.length })}</p>
          <div
            role="group"
            aria-labelledby={`${id}-variants`}
            className="flex flex-wrap gap-2"
            onKeyDown={(e) => { if (e.key === 'Escape') setDismissed(key) }}
          >
            {variants.map((v, i) => (
              <button
                key={`${v.trade_name}-${String(i)}`}
                type="button"
                onClick={() => { pick(v) }}
                className={`${pillClass} border-zinc-700 bg-zinc-950 text-zinc-200 hover:border-amber-500/60 hover:bg-zinc-800`}
              >
                <span className="font-medium text-zinc-100">{v.trade_name}</span>
                {describe(v) && <span className="ml-1.5 text-zinc-400">{describe(v)}</span>}
              </button>
            ))}
            <button
              type="button"
              onClick={() => { setDismissed(key) }}
              className={`${pillClass} border-transparent text-zinc-400 hover:text-zinc-200`}
            >
              {t('quickFill.dismiss')}
            </button>
          </div>
        </div>
      )}
    </section>
  )
}
