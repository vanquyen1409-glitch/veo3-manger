import { create } from 'zustand';
import { GetCDPStatus, LaunchChromeCDP, TestCDPConnection } from '../../wailsjs/go/main/App';
import { EventsOn } from '../../wailsjs/runtime/runtime';

export interface ProgressEvent {
  videoId: string;
  stage: string;
  message: string;
}

interface CDPState {
  connected: boolean;
  message: string;
  testing: boolean;
  launching: boolean;
  progress: ProgressEvent | null;
  refresh: () => Promise<void>;
  test: () => Promise<{ connected: boolean; message: string }>;
  launch: () => Promise<{ connected: boolean; message: string }>;
  initListeners: () => void;
}

let listenersAttached = false;
let progressClearTimer: ReturnType<typeof setTimeout> | null = null;
const TERMINAL_STAGES = new Set(['completed', 'failed']);

export const useCDPStore = create<CDPState>((set) => ({
  connected: false,
  message: 'Đang kiểm tra...',
  testing: false,
  launching: false,
  progress: null,

  refresh: async () => {
    try {
      const s = await GetCDPStatus();
      set({ connected: s.connected, message: s.message });
    } catch (e) {
      set({ connected: false, message: String(e) });
    }
  },

  test: async () => {
    set({ testing: true });
    try {
      const s = await TestCDPConnection();
      set({ connected: s.connected, message: s.message, testing: false });
      return { connected: s.connected, message: s.message };
    } catch (e) {
      const msg = String(e);
      set({ connected: false, message: msg, testing: false });
      return { connected: false, message: msg };
    }
  },

  launch: async () => {
    set({ launching: true });
    try {
      const s = await LaunchChromeCDP();
      set({ connected: s.connected, message: s.message, launching: false });
      return { connected: s.connected, message: s.message };
    } catch (e) {
      const msg = String(e);
      set({ launching: false, message: msg });
      return { connected: false, message: msg };
    }
  },

  initListeners: () => {
    if (listenersAttached) return;
    listenersAttached = true;
    EventsOn('cdp:status', (s: { connected: boolean; message: string }) => {
      set({ connected: s.connected, message: s.message });
    });
    EventsOn('video:progress', (p: ProgressEvent) => {
      if (progressClearTimer !== null) {
        clearTimeout(progressClearTimer);
        progressClearTimer = null;
      }
      set({ progress: p });
      if (TERMINAL_STAGES.has(p.stage)) {
        progressClearTimer = setTimeout(() => {
          set({ progress: null });
          progressClearTimer = null;
        }, 4000);
      }
    });
  },
}));
