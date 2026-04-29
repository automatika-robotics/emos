// Markdown helpers for recipe descriptions and other long-form text we
// pull from manifests / catalog. Recipe descriptions reach the dashboard
// from an external catalog (or, for sideloaded recipes, from manifests
// authored by anyone with file-system access).
//
// marked settings:
//   - gfm: true       — task lists, autolinks, strikethrough
//   - breaks: true    — newline → <br>; matches how recipe descriptions
//                       are written in plain text today

import DOMPurify from 'dompurify';
import { marked } from 'marked';

marked.use({
  gfm: true,
  breaks: true,
});

// Locked-down profile: only the tags / attrs that make sense in a recipe
// description card. Drops `<script>`, `<iframe>`, event handlers, and
// `javascript:` URLs by default. Links open with `target="_blank"` and
// `rel="noopener noreferrer"`.
const PURIFY_CONFIG = {
  ALLOWED_TAGS: [
    'a', 'b', 'strong', 'i', 'em',
    'code', 'pre', 'kbd', 'br', 'p',
    'ul', 'ol', 'li',
    'h1', 'h2', 'h3', 'h4', 'h5', 'h6',
    'blockquote', 'hr', 'span', 'del', 's',
    'input',
  ] as string[],
  ALLOWED_ATTR: ['href', 'title', 'target', 'rel', 'type', 'checked', 'disabled'] as string[],
  ALLOW_DATA_ATTR: false,
};

DOMPurify.addHook('afterSanitizeAttributes', (node) => {
  if (node instanceof HTMLAnchorElement) {
    node.setAttribute('target', '_blank');
    node.setAttribute('rel', 'noopener noreferrer');
  }
  // GFM task lists render <input type="checkbox">. Disable interaction so
  // it can't dispatch synthetic clicks against page handlers.
  if (node instanceof HTMLInputElement) {
    node.setAttribute('disabled', 'disabled');
  }
});

/** Render a markdown string to sanitised HTML for use with `{@html ...}`. */
export function renderMarkdown(input: string | undefined | null): string {
  if (!input) return '';
  const raw = marked.parse(input, { async: false }) as string;
  return DOMPurify.sanitize(raw, PURIFY_CONFIG) as string;
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
