import { ReactNode } from 'react';

interface Option<T extends string | number> {
  value: T;
  label: ReactNode;
}

interface SegmentedControlProps<T extends string | number> {
  value: T;
  onChange: (next: T) => void;
  options: Option<T>[];
  className?: string;
  disabled?: boolean;
  size?: 'sm' | 'md';
}

export function SegmentedControl<T extends string | number>({
  value,
  onChange,
  options,
  className = '',
  disabled = false,
  size = 'md',
}: SegmentedControlProps<T>) {
  const padding = size === 'sm' ? 'px-3 py-1.5 text-xs' : 'px-4 py-2 text-sm';
  return (
    <div
      role="tablist"
      className={[
        'inline-flex flex-wrap items-stretch gap-1 rounded-lg border border-gray-300 bg-gray-50 p-1',
        className,
      ].join(' ')}
    >
      {options.map((opt) => {
        const active = opt.value === value;
        return (
          <button
            key={String(opt.value)}
            type="button"
            role="tab"
            aria-selected={active}
            disabled={disabled}
            onClick={() => onChange(opt.value)}
            className={[
              'rounded-md font-medium transition-colors',
              padding,
              active
                ? 'bg-white text-gray-900 shadow-sm ring-1 ring-gray-200'
                : 'text-gray-600 hover:text-gray-900',
              disabled ? 'opacity-50 cursor-not-allowed' : '',
            ].join(' ')}
          >
            {opt.label}
          </button>
        );
      })}
    </div>
  );
}
