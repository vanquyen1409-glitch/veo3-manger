import { SelectHTMLAttributes, forwardRef } from 'react';
import { ChevronDown } from 'lucide-react';

type SelectProps = SelectHTMLAttributes<HTMLSelectElement>;

export const Select = forwardRef<HTMLSelectElement, SelectProps>(function Select(
  { className = '', children, ...rest },
  ref,
) {
  return (
    <div className="relative">
      <select
        ref={ref}
        className={[
          'block w-full appearance-none rounded-lg border border-gray-300 bg-white px-3 pr-9 py-2 text-sm text-gray-900',
          'shadow-sm transition-colors',
          'focus:outline-none focus:border-indigo-500 focus:ring-2 focus:ring-indigo-500/20',
          'disabled:bg-gray-50 disabled:text-gray-500 disabled:cursor-not-allowed',
          className,
        ].join(' ')}
        {...rest}
      >
        {children}
      </select>
      <ChevronDown className="pointer-events-none absolute right-2.5 top-1/2 h-4 w-4 -translate-y-1/2 text-gray-400" />
    </div>
  );
});
