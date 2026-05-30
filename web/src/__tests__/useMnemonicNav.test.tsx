import { describe, it, expect, vi, beforeEach } from 'vitest'
import { renderHook, act } from '@testing-library/react'
import { useMnemonicNav } from '../hooks/useMnemonicNav'

const mockNavigate = vi.fn()

vi.mock('@tanstack/react-router', () => ({
  useRouter: () => ({ navigate: mockNavigate }),
}))

function fireKey(key: string, target?: EventTarget) {
  const e = new KeyboardEvent('keydown', { key, bubbles: true })
  ;(target ?? document).dispatchEvent(e)
}

describe('useMnemonicNav', () => {
  beforeEach(() => {
    mockNavigate.mockClear()
  })

  it('navigates to /dashboard on g d', async () => {
    renderHook(() => useMnemonicNav())
    act(() => { fireKey('g') })
    await act(async () => { fireKey('d') })
    expect(mockNavigate).toHaveBeenCalledWith({ to: '/dashboard' })
  })

  it('navigates to /memory on g m', async () => {
    renderHook(() => useMnemonicNav())
    act(() => { fireKey('g') })
    await act(async () => { fireKey('m') })
    expect(mockNavigate).toHaveBeenCalledWith({ to: '/memory' })
  })

  it('navigates to /graph on g g', async () => {
    renderHook(() => useMnemonicNav())
    act(() => { fireKey('g') })
    await act(async () => { fireKey('g') })
    expect(mockNavigate).toHaveBeenCalledWith({ to: '/graph' })
  })

  it('navigates to /settings on g ,', async () => {
    renderHook(() => useMnemonicNav())
    act(() => { fireKey('g') })
    await act(async () => { fireKey(',') })
    expect(mockNavigate).toHaveBeenCalledWith({ to: '/settings' })
  })

  it('does not navigate on single g key', () => {
    renderHook(() => useMnemonicNav())
    act(() => { fireKey('g') })
    expect(mockNavigate).not.toHaveBeenCalled()
  })

  it('ignores keystrokes when focused on input', () => {
    const input = document.createElement('input')
    document.body.appendChild(input)
    input.focus()
    renderHook(() => useMnemonicNav())
    act(() => { fireKey('g', document) })
    act(() => { fireKey('d', document) })
    expect(mockNavigate).not.toHaveBeenCalled()
    document.body.removeChild(input)
  })

  it('expires pending g after 800ms without navigate', async () => {
    vi.useFakeTimers()
    renderHook(() => useMnemonicNav())
    act(() => { fireKey('g') })
    act(() => { vi.advanceTimersByTime(900) })
    act(() => { fireKey('d') })
    expect(mockNavigate).not.toHaveBeenCalled()
    vi.useRealTimers()
  })
})
