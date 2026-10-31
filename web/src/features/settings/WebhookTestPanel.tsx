import { useMutation } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { Send } from 'lucide-react'
import { api } from '../../lib/api'
import { useErrorText } from '../../lib/errors'
import type { DeliveryResult, Webhook } from '../../lib/types'
import { Button } from '../../components/Button'
import { Badge, ErrorBanner } from '../../components/ui'

/** "Send test notification" with response status, body and timing (FR-040). */
export function WebhookTestPanel({ webhook, onSent }: { webhook: Webhook; onSent: () => void }) {
  const { t } = useTranslation('webhooks')
  const errorText = useErrorText()
  const send = useMutation({
    mutationFn: () => api.post<{ delivery: DeliveryResult }>(`/webhooks/${webhook.id}/test`).then((r) => r.delivery),
    onSuccess: onSent,
  })
  const r = send.data
  return (
    <div className="space-y-2">
      <Button size="sm" onClick={() => { send.mutate() }} disabled={send.isPending}>
        <Send size={14} />
        {send.isPending ? t('test.sending') : t('test.send')}
      </Button>
      {send.isError && <ErrorBanner message={errorText(send.error)} />}
      {r && (
        <div className="space-y-2 rounded-md border border-zinc-800 bg-zinc-950 p-3 text-sm">
          <div className="flex flex-wrap items-center gap-2">
            <Badge tone={r.status === 'succeeded' ? 'emerald' : 'red'}>{r.status_code ?? t('test.error')}</Badge>
            <span className="tabular-nums text-zinc-400">{t('test.duration', { ms: r.duration_ms })}</span>
            {r.error && <span className="text-red-300">{t(`errors.${r.error}`)}</span>}
          </div>
          <div>
            <div className="mb-1 text-xs uppercase tracking-wide text-zinc-400">{t('test.response')}</div>
            <pre className="max-h-48 overflow-auto whitespace-pre-wrap break-all rounded bg-zinc-900 p-2 font-mono text-xs text-zinc-300">{r.response_excerpt || t('test.noResponse')}</pre>
          </div>
        </div>
      )}
    </div>
  )
}
