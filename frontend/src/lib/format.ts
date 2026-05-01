import { format, formatDistanceToNow } from 'date-fns';
import { vi } from 'date-fns/locale';

export function formatDate(input: unknown): string {
  if (!input) return '';
  const d = new Date(input as string);
  if (Number.isNaN(d.getTime())) return '';
  return format(d, 'HH:mm dd/MM/yyyy', { locale: vi });
}

export function formatRelative(input: unknown): string {
  if (!input) return '';
  const d = new Date(input as string);
  if (Number.isNaN(d.getTime())) return '';
  return formatDistanceToNow(d, { addSuffix: true, locale: vi });
}

export function truncate(text: string, max = 120): string {
  if (!text) return '';
  if (text.length <= max) return text;
  return text.slice(0, max).trimEnd() + '…';
}
