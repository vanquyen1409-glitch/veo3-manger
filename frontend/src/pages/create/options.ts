// Static option lists for the Create page form. Kept in a data-only module
// so the page render and tests can import without pulling lucide-react.

export const ASPECT_OPTIONS = [
  { value: '16:9', label: '16:9 (Ngang)' },
  { value: '9:16', label: '9:16 (Dọc)' },
] as const;

export const RESOLUTION_OPTIONS = [
  { value: '720p', label: '720p' },
  { value: '1080p', label: '1080p' },
  { value: '4k', label: '4K' },
] as const;

export const DURATION_OPTIONS = [
  { value: 4, label: '4s' },
  { value: 6, label: '6s' },
  { value: 8, label: '8s' },
] as const;

export type AspectValue = (typeof ASPECT_OPTIONS)[number]['value'];
export type ResolutionValue = (typeof RESOLUTION_OPTIONS)[number]['value'];
export type DurationValue = (typeof DURATION_OPTIONS)[number]['value'];
