interface TagPillsProps {
  tags: string[];
  // 'gray' is the muted card-context style; 'indigo' is the prominent
  // form/preview style. Defaults to 'indigo' since that's the more common use.
  variant?: 'gray' | 'indigo';
  // When set, only the first `maxVisible` tags render; the rest are
  // collapsed into a "+N" overflow chip.
  maxVisible?: number;
}

const VARIANT_CLASSES = {
  gray: 'bg-gray-100 text-gray-600',
  indigo: 'bg-indigo-50 text-indigo-700',
} as const;

// Reusable tag-chip list. Used in CreatePage (form preview), VideoCard
// (gray, with overflow), and VideoDetailModal (indigo, no overflow).
export function TagPills({ tags, variant = 'indigo', maxVisible }: TagPillsProps) {
  const visible = maxVisible !== undefined ? tags.slice(0, maxVisible) : tags;
  const overflow = maxVisible !== undefined ? Math.max(tags.length - visible.length, 0) : 0;
  const chipClass = `rounded-full px-2 py-0.5 text-[11px] font-medium ${VARIANT_CLASSES[variant]}`;

  return (
    <div className="flex flex-wrap gap-1.5">
      {visible.map((t) => (
        <span key={t} className={chipClass}>
          #{t}
        </span>
      ))}
      {overflow > 0 && <span className="text-[11px] text-gray-400">+{overflow}</span>}
    </div>
  );
}
