import { Film, Library, Settings as SettingsIcon, Sparkles, CircleCheck, CircleAlert } from 'lucide-react';
import { Page, useNavStore } from '../stores/navStore';
import { useCDPStore } from '../stores/cdpStore';

interface NavItem {
  page: Page;
  label: string;
  icon: typeof Film;
}

const NAV_ITEMS: NavItem[] = [
  { page: 'create', label: 'Tạo Video Mới', icon: Film },
  { page: 'library', label: 'Thư Viện Video', icon: Library },
  { page: 'settings', label: 'Cài Đặt', icon: SettingsIcon },
];

export function Sidebar() {
  const page = useNavStore((s) => s.page);
  const setPage = useNavStore((s) => s.setPage);
  const connected = useCDPStore((s) => s.connected);
  const message = useCDPStore((s) => s.message);

  return (
    <aside className="flex h-screen w-60 shrink-0 flex-col border-r border-gray-200 bg-white">
      <div className="flex items-center gap-2 px-5 py-5">
        <div className="flex h-9 w-9 items-center justify-center rounded-lg bg-indigo-600 text-white">
          <Sparkles className="h-5 w-5" />
        </div>
        <div className="flex flex-col">
          <span className="text-sm font-semibold text-gray-900">Veo3 Manager</span>
          <span className="text-[11px] text-gray-500">Tự động hóa Veo3</span>
        </div>
      </div>

      <nav className="flex flex-1 flex-col gap-1 px-3">
        {NAV_ITEMS.map((item) => {
          const active = page === item.page;
          const Icon = item.icon;
          return (
            <button
              key={item.page}
              type="button"
              onClick={() => setPage(item.page)}
              className={[
                'flex items-center gap-3 rounded-lg px-3 py-2.5 text-sm font-medium transition-colors',
                active
                  ? 'bg-indigo-50 text-indigo-700'
                  : 'text-gray-700 hover:bg-gray-100 hover:text-gray-900',
              ].join(' ')}
            >
              <Icon className={['h-4 w-4', active ? 'text-indigo-600' : 'text-gray-500'].join(' ')} />
              <span>{item.label}</span>
            </button>
          );
        })}
      </nav>

      <div className="border-t border-gray-200 p-4">
        <div className="flex items-start gap-2">
          {connected ? (
            <CircleCheck className="mt-0.5 h-4 w-4 shrink-0 text-emerald-500" />
          ) : (
            <CircleAlert className="mt-0.5 h-4 w-4 shrink-0 text-amber-500" />
          )}
          <div className="min-w-0">
            <div
              className={[
                'text-xs font-medium',
                connected ? 'text-emerald-700' : 'text-amber-700',
              ].join(' ')}
            >
              {connected ? 'CDP đã kết nối' : 'CDP chưa kết nối'}
            </div>
            <div className="truncate text-[11px] text-gray-500" title={message}>
              {message}
            </div>
          </div>
        </div>
      </div>
    </aside>
  );
}
