import type { SubmitEvent, ReactNode } from 'react'
import { BRAND } from '../../lib/brand'

export function AuthCard({ title, intro, onSubmit, children }: { title: ReactNode; intro?: ReactNode; onSubmit: (e: SubmitEvent) => void; children: ReactNode }) {
  return (
    <div className="flex min-h-dvh items-center justify-center p-4">
      <form onSubmit={onSubmit} className="w-full max-w-sm space-y-5 rounded-lg border border-zinc-800 bg-zinc-900 p-6 shadow-xl" noValidate>
        <div>
          <div className="mb-4 text-xl font-semibold tracking-tight text-sky-400">{BRAND}</div>
          <h1 className="text-lg font-semibold text-zinc-100">{title}</h1>
          {intro && <p className="mt-1 text-sm text-zinc-400">{intro}</p>}
        </div>
        {children}
      </form>
    </div>
  )
}
