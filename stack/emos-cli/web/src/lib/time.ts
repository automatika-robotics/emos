// Compact relative-time formatter. Intl.RelativeTimeFormat in 16 lines.
const rtf = new Intl.RelativeTimeFormat('en', { numeric: 'auto' });

const STEPS: [Intl.RelativeTimeFormatUnit, number][] = [
  ['second', 1],
  ['minute', 60],
  ['hour', 60 * 60],
  ['day', 60 * 60 * 24],
  ['week', 60 * 60 * 24 * 7],
  ['month', 60 * 60 * 24 * 30],
  ['year', 60 * 60 * 24 * 365],
];

export function relTime(input: string | Date): string {
  const t = typeof input === 'string' ? new Date(input) : input;
  if (isNaN(t.getTime())) return '';
  const seconds = (t.getTime() - Date.now()) / 1000;
  let unit: Intl.RelativeTimeFormatUnit = 'second';
  let value = seconds;
  for (let i = STEPS.length - 1; i >= 0; i--) {
    const [u, threshold] = STEPS[i];
    if (Math.abs(seconds) >= threshold) {
      unit = u;
      value = seconds / threshold;
      break;
    }
  }
  return rtf.format(Math.round(value), unit);
}

export function formatDuration(start: string | Date, end?: string | Date | null): string {
  const a = typeof start === 'string' ? new Date(start) : start;
  const b = end ? (typeof end === 'string' ? new Date(end) : end) : new Date();
  const ms = b.getTime() - a.getTime();
  if (ms < 1000) return `${ms}ms`;
  const s = Math.floor(ms / 1000);
  if (s < 60) return `${s}s`;
  const m = Math.floor(s / 60);
  const r = s % 60;
  if (m < 60) return `${m}m ${r}s`;
  const h = Math.floor(m / 60);
  return `${h}h ${m % 60}m`;
}
