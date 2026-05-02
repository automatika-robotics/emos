// Tiny classnames helper. The dashboard is small enough that we don't pull in clsx.
export function cn(...parts: (string | false | null | undefined | Record<string, boolean>)[]): string {
  const out: string[] = [];
  for (const p of parts) {
    if (!p) continue;
    if (typeof p === 'string') out.push(p);
    else for (const [k, v] of Object.entries(p)) if (v) out.push(k);
  }
  return out.join(' ');
}
