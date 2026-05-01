// buildLocalFileURL converts an absolute filesystem path into a URL the
// embedded WebView can load via the /localfile/ HTTP handler. Path segments
// are encoded individually so `#` / `?` / `&` in filenames don't get parsed
// as fragment / query separators.
export function buildLocalFileURL(absPath: string): string {
  if (!absPath) return '';
  const normalized = absPath.replace(/\\/g, '/');
  const encoded = normalized.split('/').map(encodeURIComponent).join('/');
  return `/localfile/${encoded}`;
}
