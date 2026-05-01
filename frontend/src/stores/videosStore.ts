import { create } from 'zustand';
import { CreateVideo, DeleteVideo, ListVideos } from '../../wailsjs/go/main/App';
import { EventsOn } from '../../wailsjs/runtime/runtime';
import { types } from '../../wailsjs/go/models';

type Video = types.Video;
type CreateRequest = types.CreateVideoRequest;

export type StatusFilter = 'all' | 'generating' | 'completed' | 'failed';

interface VideosState {
  videos: Video[];
  loading: boolean;
  creating: boolean;
  statusFilter: StatusFilter;
  search: string;
  load: () => Promise<void>;
  create: (req: CreateRequest) => Promise<Video>;
  remove: (id: string) => Promise<void>;
  setStatusFilter: (f: StatusFilter) => void;
  setSearch: (q: string) => void;
  initListeners: () => void;
}

let listenersAttached = false;

export const useVideosStore = create<VideosState>((set, get) => ({
  videos: [],
  loading: false,
  creating: false,
  statusFilter: 'all',
  search: '',

  load: async () => {
    set({ loading: true });
    try {
      const list = await ListVideos();
      set({ videos: list ?? [], loading: false });
    } catch (e) {
      set({ loading: false });
      throw e;
    }
  },

  create: async (req) => {
    set({ creating: true });
    try {
      // Backend emits `videos:changed` after persisting; the listener in
      // initListeners triggers load() automatically. No need to load here.
      const v = await CreateVideo(req);
      set({ creating: false });
      return v;
    } catch (e) {
      set({ creating: false });
      throw e;
    }
  },

  remove: async (id) => {
    // Backend emits `videos:changed` → listener reloads.
    await DeleteVideo(id);
  },

  setStatusFilter: (statusFilter) => set({ statusFilter }),
  setSearch: (search) => set({ search }),

  initListeners: () => {
    if (listenersAttached) return;
    listenersAttached = true;
    EventsOn('videos:changed', () => {
      void get().load();
    });
  },
}));

// Inputs for selectFiltered. Accepting just these three fields (rather than
// the full VideosState) lets callers compute the result inside a useMemo
// keyed only on what actually matters, avoiding spurious recomputation when
// unrelated fields like `loading` change.
export interface FilterInputs {
  videos: Video[];
  statusFilter: StatusFilter;
  search: string;
}

export function selectFiltered({ videos, statusFilter, search }: FilterInputs): Video[] {
  const q = search.trim().toLowerCase();
  return videos.filter((v) => {
    if (statusFilter !== 'all' && v.status !== statusFilter) return false;
    if (!q) return true;
    if (v.prompt.toLowerCase().includes(q)) return true;
    if (v.tags?.some((t) => t.toLowerCase().includes(q))) return true;
    return false;
  });
}
