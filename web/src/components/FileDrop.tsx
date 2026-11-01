import { useRef, useState, type DragEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { Upload } from 'lucide-react'
import { api } from '../lib/api'
import { useErrorText } from '../lib/errors'
import { useSession } from '../app/session'
import type { Attachment } from '../lib/types'
import { ErrorBanner } from './ui'

interface Props {
  /** API path, e.g. /repairs/3/attachments */
  path: string
  method?: 'POST' | 'PUT'
  imagesOnly?: boolean
  multiple?: boolean
  onUploaded: (a: Attachment | undefined) => void
  compact?: boolean
}

const ACCEPT_ALL = 'image/jpeg,image/png,image/webp,image/heic,image/heif,.heic,.heif,application/pdf'
const ACCEPT_IMAGES = 'image/jpeg,image/png,image/webp,image/heic,image/heif,.heic,.heif'

/** Drag-and-drop or picker upload with size pre-check and progress (research R18). */
export function FileDrop({ path, method = 'POST', imagesOnly, multiple = true, onUploaded, compact }: Props) {
  const { t } = useTranslation()
  const { instance } = useSession()
  const errorText = useErrorText()
  const input = useRef<HTMLInputElement>(null)
  const [progress, setProgress] = useState<number | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [over, setOver] = useState(false)
  const maxBytes = instance.max_upload_mb * 1024 * 1024

  const send = async (files: File[]) => {
    setError(null)
    for (const file of files) {
      if (file.size > maxBytes) {
        setError(t('attachments.tooLarge', { name: file.name, mb: instance.max_upload_mb }))
        continue
      }
      try {
        setProgress(0)
        const res = await api.upload<{ attachment?: Attachment }>(method, path, file, {}, setProgress)
        onUploaded(res.attachment)
      } catch (err) {
        setError(errorText(err))
      } finally {
        setProgress(null)
      }
    }
  }

  const onDrop = (e: DragEvent) => {
    e.preventDefault()
    setOver(false)
    void send(Array.from(e.dataTransfer.files).slice(0, multiple ? undefined : 1))
  }

  return (
    <div className="space-y-2">
      <button
        type="button"
        onClick={() => input.current?.click()}
        onDragOver={(e) => { e.preventDefault(); setOver(true) }}
        onDragLeave={() => { setOver(false) }}
        onDrop={onDrop}
        disabled={progress !== null}
        className={`focus-ring flex w-full flex-col items-center justify-center gap-1 rounded-lg border border-dashed px-4 text-sm transition-colors ${compact ? 'py-3' : 'py-6'} ${over ? 'border-sky-400 bg-sky-500/10 text-sky-200' : 'border-zinc-700 text-zinc-400 hover:border-zinc-600 hover:text-zinc-200'}`}
      >
        <Upload size={18} />
        {progress === null ? t('attachments.drop') : t('attachments.uploading', { percent: Math.round(progress * 100) })}
        {!compact && <span className="text-xs text-zinc-400">{t('attachments.types', { mb: instance.max_upload_mb })}</span>}
        {progress !== null && (
          <span className="mt-1 h-1 w-40 overflow-hidden rounded bg-zinc-800">
            <span className="block h-full bg-sky-400" style={{ width: `${Math.round(progress * 100)}%` }} />
          </span>
        )}
      </button>
      <input
        ref={input}
        type="file"
        hidden
        multiple={multiple}
        accept={imagesOnly ? ACCEPT_IMAGES : ACCEPT_ALL}
        onChange={(e) => {
          const files = Array.from(e.target.files ?? [])
          e.target.value = ''
          void send(files)
        }}
      />
      {error && <ErrorBanner message={error} />}
    </div>
  )
}
