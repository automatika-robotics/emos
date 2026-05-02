// Theme management. Three states:
//   'system' — follows prefers-color-scheme (default for new devices)
//   'dark'   — explicit dark
//   'light'  — explicit light
//
// We persist the user's choice and reapply on mount so the dashboard
// doesn't flash to the default theme on every load. The `data-theme`
// attribute on <html> is what flips the CSS tokens in global.css.

import { writable, type Readable } from 'svelte/store';

export type ThemeChoice = 'system' | 'dark' | 'light';
export type ResolvedTheme = 'dark' | 'light';

const STORAGE_KEY = 'emos.theme';

function readChoice(): ThemeChoice {
  if (typeof localStorage === 'undefined') return 'system';
  const v = localStorage.getItem(STORAGE_KEY);
  if (v === 'dark' || v === 'light' || v === 'system') return v;
  return 'system';
}

function systemPrefersDark(): boolean {
  if (typeof window === 'undefined') return true;
  return window.matchMedia?.('(prefers-color-scheme: dark)').matches ?? true;
}

function resolve(choice: ThemeChoice): ResolvedTheme {
  if (choice === 'system') return systemPrefersDark() ? 'dark' : 'light';
  return choice;
}

function apply(resolved: ResolvedTheme): void {
  if (typeof document === 'undefined') return;
  document.documentElement.setAttribute('data-theme', resolved);
  // Update the meta theme-color so mobile browser chrome matches.
  const meta = document.querySelector('meta[name="theme-color"]') as HTMLMetaElement | null;
  if (meta) meta.content = resolved === 'dark' ? '#0a0a0b' : '#ffffff';
}

const _choice = writable<ThemeChoice>(readChoice());
const _resolved = writable<ResolvedTheme>(resolve(readChoice()));

export const themeChoice: Readable<ThemeChoice> = { subscribe: _choice.subscribe };
export const theme: Readable<ResolvedTheme> = { subscribe: _resolved.subscribe };

export function setTheme(choice: ThemeChoice): void {
  _choice.set(choice);
  try {
    if (choice === 'system') localStorage.removeItem(STORAGE_KEY);
    else localStorage.setItem(STORAGE_KEY, choice);
  } catch { /* private mode */ }
  const r = resolve(choice);
  _resolved.set(r);
  apply(r);
}

/** Cycle: system → light → dark → system. Used by the header toggle. */
export function cycleTheme(): void {
  let next: ThemeChoice = 'system';
  _choice.subscribe((c) => {
    next = c === 'system' ? 'light' : c === 'light' ? 'dark' : 'system';
  })();
  setTheme(next);
}

/** Initialise theme on app mount. Apply current choice and listen for OS
 *  theme changes when the user is on 'system'. */
export function initTheme(): void {
  if (typeof window === 'undefined') return;
  const choice = readChoice();
  apply(resolve(choice));
  _choice.set(choice);
  _resolved.set(resolve(choice));

  const mq = window.matchMedia('(prefers-color-scheme: dark)');
  const onChange = () => {
    let c: ThemeChoice = 'system';
    _choice.subscribe((v) => (c = v))();
    if (c === 'system') {
      const r = resolve('system');
      _resolved.set(r);
      apply(r);
    }
  };
  // Modern browsers; old Safari uses .addListener.
  mq.addEventListener?.('change', onChange);
}
