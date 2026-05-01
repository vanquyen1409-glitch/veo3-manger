import {
  ScanLine,
  CircleCheck,
  CircleAlert,
  Plug,
  Rocket,
} from 'lucide-react';
import { Button } from '../../components/ui/Button';
import { Card, CardHeader } from '../../components/ui/Card';
import { Field } from '../../components/ui/Field';
import { Input } from '../../components/ui/Input';

interface BrowserSectionProps {
  chromePath: string;
  setChromePath: (v: string) => void;
  cdpPort: string;
  setCdpPort: (v: string) => void;
  cdpConnected: boolean;
  cdpMessage: string;
  testingCDP: boolean;
  launchingCDP: boolean;
  onDetectChrome: () => void;
  onTestConnection: () => void;
  onLaunchChrome: () => void;
}

export function BrowserSection(props: BrowserSectionProps) {
  const {
    chromePath, setChromePath, cdpPort, setCdpPort,
    cdpConnected, cdpMessage, testingCDP, launchingCDP,
    onDetectChrome, onTestConnection, onLaunchChrome,
  } = props;

  return (
    <Card>
      <CardHeader
        title="Trình duyệt CDP"
        description="Đường dẫn Chrome/Chromium và cổng remote-debugging."
      />
      <div className="flex flex-col gap-4">
        <Field label="Đường dẫn Chrome/Chromium" htmlFor="chromePath">
          <div className="flex gap-2">
            <Input
              id="chromePath"
              placeholder='C:\Program Files\Google\Chrome\Application\chrome.exe'
              value={chromePath}
              onChange={(e) => setChromePath(e.target.value)}
            />
            <Button
              type="button"
              variant="secondary"
              onClick={onDetectChrome}
              leftIcon={<ScanLine className="h-4 w-4" />}
            >
              Tự động phát hiện
            </Button>
          </div>
        </Field>

        <Field label="Cổng CDP" htmlFor="cdpPort" hint="Mặc định: 9222">
          <Input
            id="cdpPort"
            type="number"
            min={1}
            max={65535}
            value={cdpPort}
            onChange={(e) => setCdpPort(e.target.value)}
            className="max-w-[160px]"
          />
        </Field>

        <div className="flex flex-col gap-3 rounded-lg border border-gray-200 bg-gray-50 px-4 py-3">
          <div className="flex items-start gap-2">
            {cdpConnected ? (
              <CircleCheck className="mt-0.5 h-5 w-5 shrink-0 text-emerald-500" />
            ) : (
              <CircleAlert className="mt-0.5 h-5 w-5 shrink-0 text-amber-500" />
            )}
            <div className="flex-1">
              <div
                className={[
                  'text-sm font-medium',
                  cdpConnected ? 'text-emerald-700' : 'text-amber-700',
                ].join(' ')}
              >
                {cdpConnected ? 'CDP đã kết nối' : 'CDP chưa kết nối'}
              </div>
              <div className="text-xs text-gray-500">{cdpMessage}</div>
            </div>
          </div>
          <div className="flex flex-wrap gap-2">
            <Button
              type="button"
              variant="primary"
              loading={launchingCDP}
              disabled={cdpConnected}
              onClick={onLaunchChrome}
              leftIcon={<Rocket className="h-4 w-4" />}
            >
              Khởi chạy Chrome CDP
            </Button>
            <Button
              type="button"
              variant="secondary"
              loading={testingCDP}
              onClick={onTestConnection}
              leftIcon={<Plug className="h-4 w-4" />}
            >
              Kiểm tra kết nối
            </Button>
          </div>
          {!cdpConnected && (
            <p className="text-[11px] leading-relaxed text-gray-500">
              Nút "Khởi chạy" sẽ mở Chrome với cổng CDP {cdpPort} và profile riêng ở
              <code className="mx-1 rounded bg-gray-200 px-1 py-0.5 text-[11px] font-mono">
                %APPDATA%/veo3-manager/chrome-profile
              </code>
              . Lần đầu, bạn cần đăng nhập Google trong cửa sổ vừa mở.
            </p>
          )}
        </div>
      </div>
    </Card>
  );
}
