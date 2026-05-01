import { useEffect, useState } from 'react';
import { toast } from 'sonner';
import { Save } from 'lucide-react';
import { Button } from '../../components/ui/Button';
import { useCDPStore } from '../../stores/cdpStore';
import { useSettingsStore } from '../../stores/settingsStore';
import { BrowserSection } from './BrowserSection';
import { SelectorSection } from './SelectorSection';
import { OutputDirSection } from './OutputDirSection';

export function SettingsPage() {
  const settings = useSettingsStore((s) => s.settings);
  const saving = useSettingsStore((s) => s.saving);
  const save = useSettingsStore((s) => s.save);
  const detectChrome = useSettingsStore((s) => s.detectChrome);
  const pickOutputDir = useSettingsStore((s) => s.pickOutputDir);
  const openOutputDir = useSettingsStore((s) => s.openOutputDir);

  const cdpConnected = useCDPStore((s) => s.connected);
  const cdpMessage = useCDPStore((s) => s.message);
  const testingCDP = useCDPStore((s) => s.testing);
  const launchingCDP = useCDPStore((s) => s.launching);
  const testCDP = useCDPStore((s) => s.test);
  const launchCDP = useCDPStore((s) => s.launch);

  const [chromePath, setChromePath] = useState(settings.chromePath);
  const [cdpPort, setCdpPort] = useState(String(settings.cdpPort));
  const [outputDir, setOutputDir] = useState(settings.outputDir);

  useEffect(() => {
    setChromePath(settings.chromePath);
    setCdpPort(String(settings.cdpPort));
    setOutputDir(settings.outputDir);
  }, [settings.chromePath, settings.cdpPort, settings.outputDir]);

  function parseCdpPort(): number | null {
    const n = parseInt(cdpPort, 10);
    if (Number.isNaN(n) || n <= 0) {
      toast.error('Cổng CDP không hợp lệ');
      return null;
    }
    return n;
  }

  async function persistCurrentForm(): Promise<boolean> {
    const portNum = parseCdpPort();
    if (portNum === null) return false;
    try {
      await save({ chromePath, cdpPort: portNum, outputDir });
      return true;
    } catch (e) {
      toast.error('Không lưu được cấu hình', { description: String(e) });
      return false;
    }
  }

  async function onDetectChrome() {
    try {
      const detected = await detectChrome();
      if (!detected) {
        toast.warning('Không tìm thấy Chrome', {
          description: 'Vui lòng nhập đường dẫn thủ công.',
        });
        return;
      }
      setChromePath(detected);
      toast.success('Đã tìm thấy Chrome', { description: detected });
    } catch (e) {
      toast.error('Lỗi khi dò Chrome', { description: String(e) });
    }
  }

  async function onPickFolder() {
    try {
      const picked = await pickOutputDir();
      if (!picked) return;
      setOutputDir(picked);
      toast.success('Đã chọn thư mục lưu', { description: picked });
    } catch (e) {
      toast.error('Không mở được hộp thoại', { description: String(e) });
    }
  }

  async function onTestConnection() {
    if (!(await persistCurrentForm())) return;
    const testResult = await testCDP();
    if (testResult.connected) {
      toast.success('Kết nối CDP thành công', { description: testResult.message });
    } else {
      toast.error('Không kết nối được', { description: testResult.message });
    }
  }

  async function onLaunchChrome() {
    if (parseCdpPort() === null) return;
    if (!chromePath.trim()) {
      toast.error('Chưa cấu hình đường dẫn Chrome', {
        description: 'Bấm Tự động phát hiện hoặc nhập tay đường dẫn chrome.exe.',
      });
      return;
    }
    if (!(await persistCurrentForm())) return;
    const launchResult = await launchCDP();
    if (launchResult.connected) {
      toast.success('Đã khởi chạy Chrome CDP', {
        description: launchResult.message + '. Đăng nhập Google trong cửa sổ Chrome vừa mở.',
      });
    } else {
      toast.error('Không khởi chạy được Chrome', { description: launchResult.message });
    }
  }

  async function onSave() {
    if (await persistCurrentForm()) {
      toast.success('Đã lưu cấu hình');
    }
  }

  return (
    <div className="mx-auto flex max-w-3xl flex-col gap-6">
      <header>
        <h1 className="text-2xl font-semibold text-gray-900">Cài Đặt</h1>
        <p className="mt-1 text-sm text-gray-500">
          Cấu hình trình duyệt CDP, selector và thư mục lưu video.
        </p>
      </header>

      <BrowserSection
        chromePath={chromePath}
        setChromePath={setChromePath}
        cdpPort={cdpPort}
        setCdpPort={setCdpPort}
        cdpConnected={cdpConnected}
        cdpMessage={cdpMessage}
        testingCDP={testingCDP}
        launchingCDP={launchingCDP}
        onDetectChrome={() => void onDetectChrome()}
        onTestConnection={() => void onTestConnection()}
        onLaunchChrome={() => void onLaunchChrome()}
      />

      <SelectorSection />

      <OutputDirSection
        outputDir={outputDir}
        setOutputDir={setOutputDir}
        onPickFolder={() => void onPickFolder()}
        onOpenOutputDir={() => void openOutputDir()}
      />

      <div className="flex justify-end">
        <Button
          variant="primary"
          size="lg"
          loading={saving}
          onClick={onSave}
          leftIcon={<Save className="h-4 w-4" />}
        >
          Lưu
        </Button>
      </div>
    </div>
  );
}
