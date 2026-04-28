// Markdown helpers for recipe descriptions and any other long-form text we
// pull from manifests / catalog. Trust model: content comes from our own
// catalog API or the local recipe manifest on disk — both authored by us
// or the device owner. Attempting full XSS sanitization (DOMPurify etc.)
// would double the bundle size for a self-XSS-only risk surface.
//
// Settings:
//   - gfm: true       — task lists, autolinks, strikethrough
//   - breaks: true    — newline → <br>; matches how recipe descriptions
//                       are written in plain text today
//   - mangle/headerIds disabled to keep output predictable for embedding

import { marked } from 'marked';

// marked v9+ recommends `use(...)` for global option registration; the older
// `setOptions` was deprecated and silently ignores some flags in v18.
marked.use({
  gfm: true,
  breaks: true,
});

/** Render a markdown string to HTML for use with Svelte's `{@html ...}`. */
export function renderMarkdown(input: string | undefined | null): string {
  if (!input) return '';
  // marked.parse can be sync or async depending on extensions; we don't use any.
  return marked.parse(input, { async: false }) as string;
}

/** Strip markdown for plain-text contexts (e.g. fuzzy-search needles). */
export function stripMarkdown(input: string | undefined | null): string {
  if (!input) return '';
  return input
    .replace(/\*\*(.*?)\*\*/g, '$1')
    .replace(/\*(.*?)\*/g, '$1')
    .replace(/`([^`]+)`/g, '$1')
    .replace(/^#+\s+/gm, '')
    .replace(/^\s*[-*+]\s+/gm, '')
    .replace(/\s+/g, ' ')
    .trim();
}
