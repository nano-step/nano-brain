import React, { useEffect, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import { useFocusTrap } from '../hooks/useFocusTrap'

export interface ConfirmDialogProps {
  open: boolean
  title: string
  description: string
  confirmText: string
  confirmLabel: string
  onConfirm: () => void
  onCancel: () => void
  danger?: boolean
}

export function ConfirmDialog({
  open,
  title,
  description,
  confirmText,
  confirmLabel,
  onConfirm,
  onCancel,
  danger = false,
}: ConfirmDialogProps) {
  const [typed, setTyped] = useState('')
  const trapRef = useFocusTrap(open)
  const inputRef = useRef<HTMLInputElement>(null)

  useEffect(() => {
    if (open) {
      setTyped('')
      setTimeout(() => inputRef.current?.focus(), 50)
    }
  }, [open])

  useEffect(() => {
    if (!open) return
    function onKey(e: KeyboardEvent) {
      if (e.key === 'Escape') onCancel()
    }
    document.addEventListener('keydown', onKey)
    return () => document.removeEventListener('keydown', onKey)
  }, [open, onCancel])

  if (!open) return null

  const matches = typed === confirmText

  return createPortal(
    <div className="confirm-dialog-backdrop" role="dialog" aria-modal="true" aria-labelledby="confirm-title">
      <div
        className={`confirm-dialog${danger ? ' confirm-dialog--danger' : ''}`}
        ref={trapRef as React.RefObject<HTMLDivElement>}
      >
        <h2 id="confirm-title" className="confirm-dialog-title">{title}</h2>
        <p className="confirm-dialog-desc">{description}</p>
        <label className="confirm-dialog-label" htmlFor="confirm-input">
          Type <strong>{confirmText}</strong> to confirm
        </label>
        <input
          id="confirm-input"
          ref={inputRef}
          className="form-input confirm-dialog-input"
          type="text"
          value={typed}
          onChange={(e) => setTyped(e.target.value)}
          autoComplete="off"
          spellCheck={false}
        />
        <div className="confirm-dialog-actions">
          <button className="btn" onClick={onCancel}>
            Cancel
          </button>
          <button
            className={`btn${danger ? ' btn-danger' : ' btn-primary'}`}
            disabled={!matches}
            onClick={() => { if (matches) onConfirm() }}
          >
            {confirmLabel}
          </button>
        </div>
      </div>
    </div>,
    document.body,
  )
}
