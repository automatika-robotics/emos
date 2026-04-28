// Tiny hash-based router. ~60 LoC. We pick hash routing on purpose:
// the SPA is served from go:embed under arbitrary paths, and hash routes
// don't require server-side route fall-through tweaks.

import { writable, derived, type Readable } from 'svelte/store';
import { parse } from 'regexparam';

export interface RouteState {
  path: string;
  params: Record<string, string>;
}

function readHash(): string {
  if (typeof window === 'undefined') return '/';
  const h = window.location.hash.replace(/^#/, '');
  return h || '/';
}

const _path = writable(readHash());

if (typeof window !== 'undefined') {
  window.addEventListener('hashchange', () => _path.set(readHash()));
  // Make sure deep-links work: if no hash, set one.
  if (!window.location.hash) window.location.hash = '#/';
}

/** Programmatic navigation. */
export function navigate(to: string): void {
  if (typeof window === 'undefined') return;
  window.location.hash = '#' + (to.startsWith('/') ? to : '/' + to);
}

/** Read-only current path store. */
export const path: Readable<string> = { subscribe: _path.subscribe };

/** Read-only current query-params store, derived from the hash route's
 *  `?key=value&...` portion. Empty object when no query string is present. */
export const query: Readable<Record<string, string>> = derived(_path, ($p) => {
  const i = $p.indexOf('?');
  if (i < 0) return {};
  const usp = new URLSearchParams($p.slice(i + 1));
  const out: Record<string, string> = {};
  usp.forEach((v, k) => (out[k] = v));
  return out;
});

/** Match a path against a route pattern, returning params or null. */
export function match(pattern: string, p: string): Record<string, string> | null {
  const { keys, pattern: rx } = parse(pattern);
  const matches = rx.exec(p);
  if (!matches) return null;
  const out: Record<string, string> = {};
  if (Array.isArray(keys)) {
    for (let i = 0; i < keys.length; i++) {
      out[keys[i]] = decodeURIComponent(matches[i + 1] ?? '');
    }
  }
  return out;
}

/** Action: <a use:link href="/recipes"> updates the hash on click. */
export function link(node: HTMLAnchorElement): { destroy: () => void } {
  function onClick(e: MouseEvent) {
    if (e.defaultPrevented || e.button !== 0 || e.metaKey || e.ctrlKey || e.shiftKey) return;
    const href = node.getAttribute('href');
    if (!href || /^https?:\/\//.test(href)) return;
    e.preventDefault();
    navigate(href);
  }
  node.addEventListener('click', onClick);
  return { destroy: () => node.removeEventListener('click', onClick) };
}

/** Build a routing helper: returns the active component and params for a route table. */
export function useRoutes<T>(table: Record<string, T>): Readable<{ component: T; params: Record<string, string> }> {
  return derived(_path, ($p) => {
    // Strip query string for matching, keep params separate via parse() above.
    const cleanPath = $p.split('?')[0];
    for (const pattern of Object.keys(table)) {
      if (pattern === '*') continue;
      const params = match(pattern, cleanPath);
      if (params !== null) return { component: table[pattern], params };
    }
    return { component: table['*'], params: {} };
  });
}

/** Convenience: has the path settled past `/pair`? */
export function startsWith(prefix: string, p: string): boolean {
  return p === prefix || p.startsWith(prefix + '/') || p.startsWith(prefix);
}
