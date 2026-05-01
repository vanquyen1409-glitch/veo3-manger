import { create } from 'zustand';
import {
  DetectChromePath,
  GetSettings,
  OpenPathInOS,
  SaveSettings,
  SelectFolder,
} from '../../wailsjs/go/main/App';
import { types } from '../../wailsjs/go/models';

type Settings = types.Settings;

interface SettingsState {
  settings: Settings;
  loading: boolean;
  saving: boolean;
  load: () => Promise<void>;
  save: (next: Partial<Settings>) => Promise<Settings>;
  detectChrome: () => Promise<string>;
  pickOutputDir: () => Promise<string>;
  openSelectorFile: () => Promise<void>;
  openOutputDir: () => Promise<void>;
}

const empty: Settings = new types.Settings({
  chromePath: '',
  cdpPort: 9222,
  selectorConfigPath: '',
  outputDir: '',
});

export const useSettingsStore = create<SettingsState>((set, get) => ({
  settings: empty,
  loading: false,
  saving: false,

  load: async () => {
    set({ loading: true });
    try {
      const s = await GetSettings();
      set({ settings: s, loading: false });
    } catch (e) {
      set({ loading: false });
      throw e;
    }
  },

  save: async (patch) => {
    set({ saving: true });
    try {
      const merged = new types.Settings({ ...get().settings, ...patch });
      const saved = await SaveSettings(merged);
      set({ settings: saved, saving: false });
      return saved;
    } catch (e) {
      set({ saving: false });
      throw e;
    }
  },

  detectChrome: async () => {
    const p = await DetectChromePath();
    if (p) {
      set({ settings: new types.Settings({ ...get().settings, chromePath: p }) });
    }
    return p;
  },

  pickOutputDir: async () => {
    const p = await SelectFolder();
    if (p) {
      set({ settings: new types.Settings({ ...get().settings, outputDir: p }) });
    }
    return p;
  },

  openSelectorFile: async () => {
    const path = get().settings.selectorConfigPath;
    if (!path) return;
    await OpenPathInOS(path);
  },

  openOutputDir: async () => {
    const path = get().settings.outputDir;
    if (!path) return;
    await OpenPathInOS(path);
  },
}));
