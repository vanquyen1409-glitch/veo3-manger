import { create } from 'zustand';

export type Page = 'create' | 'library' | 'settings';

interface NavState {
  page: Page;
  setPage: (p: Page) => void;
}

export const useNavStore = create<NavState>((set) => ({
  page: 'create',
  setPage: (page) => set({ page }),
}));
