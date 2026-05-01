import { useEffect } from 'react';
import { Sidebar } from './components/Sidebar';
import { CreatePage } from './pages/create/CreatePage';
import { LibraryPage } from './pages/LibraryPage';
import { SettingsPage } from './pages/settings/SettingsPage';
import { useNavStore } from './stores/navStore';
import { useCDPStore } from './stores/cdpStore';
import { useSettingsStore } from './stores/settingsStore';
import { useVideosStore } from './stores/videosStore';

function App() {
  const page = useNavStore((s) => s.page);

  const initCDPListeners = useCDPStore((s) => s.initListeners);
  const refreshCDP = useCDPStore((s) => s.refresh);
  const initVideoListeners = useVideosStore((s) => s.initListeners);
  const loadVideos = useVideosStore((s) => s.load);
  const loadSettings = useSettingsStore((s) => s.load);

  useEffect(() => {
    initCDPListeners();
    initVideoListeners();
    void loadSettings();
    void loadVideos();
    void refreshCDP();
  }, [initCDPListeners, initVideoListeners, loadSettings, loadVideos, refreshCDP]);

  return (
    <div className="flex h-screen w-screen overflow-hidden bg-gray-50 text-gray-900">
      <Sidebar />
      <main className="flex-1 overflow-y-auto">
        <div className="mx-auto w-full max-w-6xl px-8 py-8">
          {page === 'create' && <CreatePage />}
          {page === 'library' && <LibraryPage />}
          {page === 'settings' && <SettingsPage />}
        </div>
      </main>
    </div>
  );
}

export default App;
