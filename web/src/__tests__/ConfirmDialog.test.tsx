import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { ConfirmDialog } from '../components/ConfirmDialog'

function renderDialog(overrides: Partial<React.ComponentProps<typeof ConfirmDialog>> = {}) {
  const onConfirm = vi.fn()
  const onCancel = vi.fn()
  render(
    <ConfirmDialog
      open={true}
      title="Delete item"
      description="This cannot be undone"
      confirmText="my-workspace"
      confirmLabel="Delete"
      onConfirm={onConfirm}
      onCancel={onCancel}
      {...overrides}
    />,
  )
  return { onConfirm, onCancel }
}

describe('ConfirmDialog', () => {
  it('renders title and description', () => {
    renderDialog()
    expect(screen.getByText('Delete item')).toBeTruthy()
    expect(screen.getByText('This cannot be undone')).toBeTruthy()
  })

  it('confirm button is disabled when input is empty', () => {
    renderDialog()
    const btn = screen.getByRole('button', { name: 'Delete' })
    expect(btn).toBeDisabled()
  })

  it('confirm button stays disabled for partial match', () => {
    renderDialog()
    const input = screen.getByRole('textbox')
    fireEvent.change(input, { target: { value: 'my-work' } })
    expect(screen.getByRole('button', { name: 'Delete' })).toBeDisabled()
  })

  it('confirm button enables on exact match', () => {
    renderDialog()
    const input = screen.getByRole('textbox')
    fireEvent.change(input, { target: { value: 'my-workspace' } })
    expect(screen.getByRole('button', { name: 'Delete' })).not.toBeDisabled()
  })

  it('calls onConfirm when button clicked with exact match', () => {
    const { onConfirm } = renderDialog()
    const input = screen.getByRole('textbox')
    fireEvent.change(input, { target: { value: 'my-workspace' } })
    fireEvent.click(screen.getByRole('button', { name: 'Delete' }))
    expect(onConfirm).toHaveBeenCalledOnce()
  })

  it('calls onCancel when Cancel button clicked', () => {
    const { onCancel } = renderDialog()
    fireEvent.click(screen.getByRole('button', { name: 'Cancel' }))
    expect(onCancel).toHaveBeenCalledOnce()
  })

  it('calls onCancel on Escape key', () => {
    const { onCancel } = renderDialog()
    fireEvent.keyDown(document, { key: 'Escape' })
    expect(onCancel).toHaveBeenCalledOnce()
  })

  it('does not render when open=false', () => {
    renderDialog({ open: false })
    expect(screen.queryByText('Delete item')).toBeNull()
  })
})
