import { useState } from 'react'
import { useMutation } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { Download, FileText, Pencil, Trash2 } from 'lucide-react'
import { api } from '../lib/api'
import { useErrorText } from '../lib/errors'
import { useFormat } from '../lib/useFormat'
import type { Attachment } from '../lib/types'
import { IconButton, Button } from './Button'
import { ConfirmDialog, Dialog } from './Dialog'
import { TextField } from './Field'
import { ErrorBanner } from './ui'

/** Browser-displayable image types (HEIC is download-only). */
const previewable = new Set(['image/jpeg', 'image/png', 'image/webp'])

function sizeLabel(bytes: number, fmt: ReturnType<typeof useFormat>): string {
  if (bytes >= 1024 * 1024) return `${fmt.number(bytes / (1024 * 1024), { maxDecimals: 1 })} MB`
  return `${fmt.number(Math.max(1, Math.round(bytes / 1024)))} KB`
}

export function AttachmentGallery({ items, onChanged }: { items: Attachment[]; onChanged: () => void }) {
  const { t } = useTranslation()
  const fmt = useFormat()
  const errorText = useErrorText()
  const [preview, setPreview] = useState<Attachment | null>(null)
  const [editing, setEditing] = useState<Attachment | null>(null)
  const [deleting, setDeleting] = useState<Attachment | null>(null)
  const remove = useMutation({
    mutationFn: (id: number) => api.del(`/attachments/${id}`),
    onSuccess: () => { setDeleting(null); onChanged() },
  })

  if (items.length === 0) return <p className="text-sm text-zinc-400">{t('attachments.empty')}</p>

  return (
    <>
      <ul className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-4">
        {items.map((a) => (
          <li key={a.id} className="overflow-hidden rounded-lg border border-zinc-800 bg-zinc-950">
            {previewable.has(a.mime_type) ? (
              <button type="button" className="focus-ring block aspect-[4/3] w-full" onClick={() => { setPreview(a) }} aria-label={`${t('attachments.preview')}: ${a.original_name}`}>
                <img src={a.url} alt={a.caption || a.original_name} loading="lazy" className="h-full w-full object-cover" />
              </button>
            ) : (
              <a href={a.mime_type === 'application/pdf' ? a.url : `${a.url}?download=1`} target="_blank" rel="noopener noreferrer" className="focus-ring flex aspect-[4/3] w-full flex-col items-center justify-center gap-2 text-zinc-400 hover:text-zinc-200">
                <FileText size={28} />
                <span className="px-2 text-center text-xs">{a.original_name}</span>
              </a>
            )}
            <div className="flex items-center gap-1 border-t border-zinc-800 px-2 py-1.5">
              <div className="min-w-0 flex-1">
                <div className="truncate text-xs text-zinc-200">{a.caption || a.original_name}</div>
                <div className="text-[11px] text-zinc-400">{sizeLabel(a.size_bytes, fmt)}</div>
              </div>
              <a href={`${a.url}?download=1`} download className="focus-ring inline-flex h-7 w-7 items-center justify-center rounded text-zinc-400 hover:bg-zinc-800 hover:text-zinc-100" aria-label={t('attachments.download')} title={t('attachments.download')}>
                <Download size={14} />
              </a>
              <IconButton aria-label={t('attachments.caption')} onClick={() => { setEditing(a) }} className="h-7 w-7">
                <Pencil size={14} />
              </IconButton>
              <IconButton aria-label={t('actions.delete')} onClick={() => { setDeleting(a) }} className="h-7 w-7">
                <Trash2 size={14} />
              </IconButton>
            </div>
          </li>
        ))}
      </ul>
      <Dialog open={preview !== null} onClose={() => { setPreview(null) }} title={preview?.caption || preview?.original_name} wide>
        {preview && <img src={preview.url} alt={preview.caption || preview.original_name} className="mx-auto max-h-[70dvh] w-auto rounded" />}
      </Dialog>
      {editing && <CaptionDialog attachment={editing} onClose={() => { setEditing(null) }} onSaved={onChanged} />}
      <ConfirmDialog
        open={deleting !== null}
        title={t('attachments.deleteTitle')}
        body={t('attachments.deleteBody', { name: deleting?.original_name ?? '' })}
        confirmLabel={t('actions.delete')}
        danger
        busy={remove.isPending}
        error={remove.isError ? errorText(remove.error) : undefined}
        onConfirm={() => { if (deleting) remove.mutate(deleting.id) }}
        onClose={() => { setDeleting(null); remove.reset() }}
      />
    </>
  )
}

function CaptionDialog({ attachment, onClose, onSaved }: { attachment: Attachment; onClose: () => void; onSaved: () => void }) {
  const { t } = useTranslation()
  const errorText = useErrorText()
  const [caption, setCaption] = useState(attachment.caption)
  const save = useMutation({
    mutationFn: () => api.patch(`/attachments/${attachment.id}`, { caption }),
    onSuccess: () => { onSaved(); onClose() },
  })
  return (
    <Dialog
      open
      onClose={onClose}
      title={t('attachments.caption')}
      footer={
        <>
          <Button onClick={onClose}>{t('actions.cancel')}</Button>
          <Button variant="primary" onClick={() => { save.mutate() }} disabled={save.isPending}>
            {t('actions.save')}
          </Button>
        </>
      }
    >
      <div className="space-y-3">
        {save.isError && <ErrorBanner message={errorText(save.error)} />}
        <TextField label={t('attachments.caption')} value={caption} onChange={setCaption} maxLength={120} autoFocus />
      </div>
    </Dialog>
  )
}
