import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { AlertTriangle, Inbox, Loader2 } from 'lucide-react'
import { Button } from './Button'

export type Tone = 'zinc' | 'amber' | 'red' | 'emerald' | 'sky'

const tones: Record<Tone, string> = {
  zinc: 'bg-zinc-800 text-zinc-300 ring-zinc-700',
  amber: 'bg-amber-500/15 text-amber-300 ring-amber-500/30',
  red: 'bg-red-500/15 text-red-300 ring-red-500/30',
  emerald: 'bg-emerald-500/15 text-emerald-300 ring-emerald-500/30',
  sky: 'bg-sky-500/15 text-sky-300 ring-sky-500/30',
}

export function Badge({ tone = 'zinc', children }: { tone?: Tone; children: ReactNode }) {
  return <span className={`inline-flex items-center gap-1 rounded px-1.5 py-0.5 text-xs font-medium ring-1 ring-inset ${tones[tone]}`}>{children}</span>
}

export function PageHeader({ title, subtitle, actions }: { title: ReactNode; subtitle?: ReactNode; actions?: ReactNode }) {
  return (
    <div className="mb-6 flex flex-wrap items-end justify-between gap-3">
      <div className="min-w-0">
        <h1 className="text-2xl font-semibold tracking-tight text-zinc-100">{title}</h1>
        {subtitle && <p className="mt-1 text-sm text-zinc-400">{subtitle}</p>}
      </div>
      {actions && <div className="flex flex-wrap gap-2">{actions}</div>}
    </div>
  )
}

export function Card({ title, actions, children, className = '' }: { title?: ReactNode; actions?: ReactNode; children: ReactNode; className?: string }) {
  return (
    <section className={`rounded-lg border border-zinc-800 bg-zinc-900 ${className}`}>
      {(title ?? actions) && (
        <div className="flex flex-wrap items-center justify-between gap-2 border-b border-zinc-800 px-4 py-3">
          {title && <h2 className="text-sm font-semibold text-zinc-200">{title}</h2>}
          {actions && <div className="flex flex-wrap gap-2">{actions}</div>}
        </div>
      )}
      <div className="p-4">{children}</div>
    </section>
  )
}

export function EmptyState({ title, action }: { title: ReactNode; action?: ReactNode }) {
  return (
    <div className="flex flex-col items-center gap-3 rounded-lg border border-dashed border-zinc-800 px-6 py-10 text-center">
      <Inbox className="text-zinc-600" size={28} />
      <p className="text-sm text-zinc-400">{title}</p>
      {action}
    </div>
  )
}

export function Loading() {
  const { t } = useTranslation()
  return (
    <div className="flex items-center gap-2 py-8 text-sm text-zinc-400" role="status">
      <Loader2 className="animate-spin" size={16} />
      {t('state.loading')}
    </div>
  )
}

export function ErrorBanner({ message, onRetry }: { message: ReactNode; onRetry?: () => void }) {
  const { t } = useTranslation()
  return (
    <div role="alert" className="flex flex-wrap items-center gap-3 rounded-md border border-red-500/30 bg-red-500/10 px-3 py-2 text-sm text-red-300">
      <AlertTriangle size={16} className="shrink-0" />
      <span className="flex-1">{message}</span>
      {onRetry && (
        <Button size="sm" onClick={onRetry}>
          {t('actions.retry')}
        </Button>
      )}
    </div>
  )
}

export function Notice({ tone = 'sky', children }: { tone?: 'sky' | 'amber' | 'emerald'; children: ReactNode }) {
  const cls = { sky: 'border-sky-500/30 bg-sky-500/10 text-sky-200', amber: 'border-amber-500/30 bg-amber-500/10 text-amber-200', emerald: 'border-emerald-500/30 bg-emerald-500/10 text-emerald-200' }[tone]
  return <div role="status" className={`rounded-md border px-3 py-2 text-sm ${cls}`}>{children}</div>
}

/** Horizontally scrollable table wrapper so wide tables never break the page on phones. */
export function Table({ children }: { children: ReactNode }) {
  return (
    <div className="overflow-x-auto rounded-lg border border-zinc-800">
      <table className="w-full min-w-max border-collapse text-left text-sm [&_td]:border-t [&_td]:border-zinc-800 [&_td]:px-3 [&_td]:py-2 [&_th]:bg-zinc-900 [&_th]:px-3 [&_th]:py-2 [&_th]:font-medium [&_th]:text-zinc-400">
        {children}
      </table>
    </div>
  )
}

export function DefinitionList({ items }: { items: { label: ReactNode; value: ReactNode }[] }) {
  return (
    <dl className="grid grid-cols-1 gap-x-6 gap-y-3 sm:grid-cols-2">
      {items.map((it, i) => (
        <div key={i} className="min-w-0">
          <dt className="text-xs uppercase tracking-wide text-zinc-400">{it.label}</dt>
          <dd className="mt-0.5 break-words text-sm text-zinc-100">{it.value}</dd>
        </div>
      ))}
    </dl>
  )
}
