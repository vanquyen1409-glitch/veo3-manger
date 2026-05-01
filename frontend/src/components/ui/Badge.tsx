import { ReactNode } from 'react';
import { STATUS_BADGE_CLASS, STATUS_META, VideoStatusValue } from '../../lib/statuses';

interface BadgeProps {
  children: ReactNode;
  tone?: 'amber' | 'emerald' | 'rose' | 'gray' | 'indigo';
  className?: string;
}

const TONE_CLASS: Record<NonNullable<BadgeProps['tone']>, string> = {
  ...STATUS_BADGE_CLASS,
  gray: 'bg-gray-100 text-gray-700 ring-1 ring-gray-200',
  indigo: 'bg-indigo-100 text-indigo-800 ring-1 ring-indigo-200',
};

export function Badge({ children, tone = 'gray', className = '' }: BadgeProps) {
  return (
    <span
      className={[
        'inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-xs font-medium',
        TONE_CLASS[tone],
        className,
      ].join(' ')}
    >
      {children}
    </span>
  );
}

export function StatusBadge({ status }: { status: string }) {
  const key = (['generating', 'completed', 'failed'].includes(status) ? status : 'generating') as VideoStatusValue;
  const meta = STATUS_META[key];
  return <Badge tone={meta.tone}>{meta.label}</Badge>;
}
