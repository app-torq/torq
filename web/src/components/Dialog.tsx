import { useEffect, useRef, useState, type ReactNode } from 'react'
import { Trans, useTranslation } from 'react-i18next'
import { X } from 'lucide-react'
import { Button, IconButton } from './Button'
import { inputClass } from './Field'

interface DialogProps {
  open: boolean
  onClose: () => void
  title: ReactNode
  children: ReactNode
  footer?: ReactNode
  wide?: boolean
}

/** Modal dialog on the native <dialog> element (focus trap and Esc handling built in). */
export function Dialog({ open, onClose, title, children, footer, wide }: DialogProps) {
  const ref = useRef<HTMLDialogElement>(null)
  const { t } = useTranslation()
  useEffect(() => {
    const el = ref.current
    if (!el) return
    if (open && !el.open) el.showModal()
    if (!open && el.open) el.close()
  }, [open])
  return (
    <dialog
      ref={ref}
      onCancel={(e) => { e.preventDefault(); onClose() }}
      onClick={(e) => { if (e.target === ref.current) onClose() }}
      className={`m-auto w-[calc(100%-2rem)] ${wide ? 'max-w-2xl' : 'max-w-md'} rounded-lg border border-zinc-800 bg-zinc-900 p-0 text-zinc-100 shadow-2xl backdrop:bg-zinc-950/80`}
    >
      {open && (
        <div className="flex max-h-[85dvh] flex-col">
          <div className="flex items-center justify-between border-b border-zinc-800 px-5 py-3">
            <h2 className="text-base font-semibold">{title}</h2>
            <IconButton aria-label={t('actions.close')} onClick={onClose}>
              <X size={18} />
            </IconButton>
          </div>
          <div className="overflow-y-auto px-5 py-4">{children}</div>
          {footer && <div className="flex flex-wrap justify-end gap-2 border-t border-zinc-800 px-5 py-3">{footer}</div>}
        </div>
      )}
    </dialog>
  )
}

interface ConfirmDialogProps {
  open: boolean
  title: ReactNode
  body: ReactNode
  confirmLabel: ReactNode
  danger?: boolean
  /** When set, the user must type this value to enable the confirm button. */
  typeToConfirm?: string
  busy?: boolean
  error?: ReactNode
  onConfirm: () => void
  onClose: () => void
}

export function ConfirmDialog(props: ConfirmDialogProps) {
  // Remount on open/close so the typed confirmation resets.
  return <ConfirmDialogInner key={props.open ? 'open' : 'closed'} {...props} />
}

function ConfirmDialogInner({ open, title, body, confirmLabel, danger, typeToConfirm, busy, error, onConfirm, onClose }: ConfirmDialogProps) {
  const { t } = useTranslation()
  const [typed, setTyped] = useState('')
  const blocked = typeToConfirm !== undefined && typed.trim().toLowerCase() !== typeToConfirm.trim().toLowerCase()
  return (
    <Dialog
      open={open}
      onClose={onClose}
      title={title}
      footer={
        <>
          <Button onClick={onClose}>{t('actions.cancel')}</Button>
          <Button variant={danger ? 'danger' : 'primary'} disabled={blocked || busy} onClick={onConfirm}>
            {confirmLabel}
          </Button>
        </>
      }
    >
      <div className="space-y-3 text-sm text-zinc-300">
        <div>{body}</div>
        {typeToConfirm !== undefined && (
          <>
            <p>
              <Trans i18nKey="confirm.typeToConfirm" values={{ value: typeToConfirm }} components={{ strong: <strong className="text-zinc-100" /> }} />
            </p>
            <input className={inputClass} value={typed} onChange={(e) => { setTyped(e.target.value) }} aria-label={typeToConfirm} autoFocus />
          </>
        )}
        {error && <p className="text-red-400">{error}</p>}
      </div>
    </Dialog>
  )
}
