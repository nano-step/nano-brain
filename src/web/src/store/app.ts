import { create } from 'zustand';

type AppState = {
  workspace?: string;
  setWorkspace: (workspace?: string) => void;
};

export const useAppStore = create<AppState>((set) => ({
  workspace: undefined,
  setWorkspace: (workspace) => set({ workspace }),
}));
