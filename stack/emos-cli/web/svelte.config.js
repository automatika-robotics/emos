import { vitePreprocess } from '@sveltejs/vite-plugin-svelte';

// We don't force `runes: true` globally — libraries (like @tanstack/svelte-query)
// still ship legacy components and crash when forced into runes mode.
// Our own files opt into runes via syntax (the compiler auto-detects).
export default {
  preprocess: vitePreprocess(),
};
