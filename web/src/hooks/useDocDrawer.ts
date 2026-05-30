import { create } from 'zustand'
import type { Document } from '../api/types'

interface DocDrawerState {
  doc: Document | null
  open: (doc: Document) => void
  close: () => void
}

export const useDocDrawer = create<DocDrawerState>((set) => ({
  doc: null,
  open: (doc) => set({ doc }),
  close: () => set({ doc: null }),
}))
