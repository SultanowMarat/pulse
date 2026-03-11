const PLACEHOLDER_ONLY_RE = /^[_\-—–\s]+$/;

export function sanitizeOptionalProfileText(value?: string | null): string {
  const trimmed = (value ?? '').trim();
  if (!trimmed) return '';
  if (PLACEHOLDER_ONLY_RE.test(trimmed)) return '';
  return trimmed;
}

export function displayOptionalProfileText(value?: string | null, fallback = '—'): string {
  const normalized = sanitizeOptionalProfileText(value);
  return normalized || fallback;
}
