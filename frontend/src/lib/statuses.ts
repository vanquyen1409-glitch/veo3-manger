export type VideoStatusValue = 'generating' | 'completed' | 'failed';

export const STATUS_META: Record<VideoStatusValue, { label: string; tone: 'amber' | 'emerald' | 'rose' }> = {
  generating: { label: 'Đang tạo', tone: 'amber' },
  completed: { label: 'Hoàn thành', tone: 'emerald' },
  failed: { label: 'Lỗi', tone: 'rose' },
};

export const STATUS_BADGE_CLASS: Record<'amber' | 'emerald' | 'rose', string> = {
  amber: 'bg-amber-100 text-amber-800 ring-1 ring-amber-200',
  emerald: 'bg-emerald-100 text-emerald-800 ring-1 ring-emerald-200',
  rose: 'bg-rose-100 text-rose-800 ring-1 ring-rose-200',
};

export function statusFromString(s: string): VideoStatusValue {
  if (s === 'completed' || s === 'failed') return s;
  return 'generating';
}

export const STAGE_LABEL: Record<string, string> = {
  opening_browser: 'Đang mở trình duyệt...',
  entering_prompt: 'Đang nhập prompt...',
  waiting_video: 'Đang chờ video...',
  downloading: 'Đang tải video...',
  completed: 'Hoàn thành',
  failed: 'Lỗi',
};
