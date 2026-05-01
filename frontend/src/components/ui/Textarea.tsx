import { TextareaHTMLAttributes, forwardRef } from 'react';

type TextareaProps = TextareaHTMLAttributes<HTMLTextAreaElement>;

export const Textarea = forwardRef<HTMLTextAreaElement, TextareaProps>(function Textarea(
  { className = '', rows = 4, ...rest },
  ref,
) {
  return (
    <textarea
      ref={ref}
      rows={rows}
      className={[
        'block w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm text-gray-900',
        'placeholder:text-gray-400 shadow-sm transition-colors resize-y',
        'focus:outline-none focus:border-indigo-500 focus:ring-2 focus:ring-indigo-500/20',
        'disabled:bg-gray-50 disabled:text-gray-500 disabled:cursor-not-allowed',
        className,
      ].join(' ')}
      {...rest}
    />
  );
});
