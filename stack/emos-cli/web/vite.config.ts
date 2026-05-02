import { defineConfig } from 'vite';
import { svelte } from '@sveltejs/vite-plugin-svelte';
import tailwindcss from '@tailwindcss/vite';
import path from 'node:path';

// We emit straight into the Go embed directory so `make build` is just
// `make web && go build`.
export default defineConfig({
  plugins: [svelte(), tailwindcss()],
  resolve: {
    alias: {
      $lib: path.resolve(__dirname, 'src/lib'),
      $components: path.resolve(__dirname, 'src/components'),
      $routes: path.resolve(__dirname, 'src/routes'),
    },
  },
  build: {
    outDir: '../internal/webui/dist',
    // We deliberately keep emptyOutDir: false so the tracked placeholder.html
    // (used by go:embed on fresh clones) survives a vite build. The Makefile
    // cleans index.html + assets/ before invoking vite.
    emptyOutDir: false,
    sourcemap: false,
    target: 'es2022',
    cssCodeSplit: false,
    rollupOptions: {
      output: {
        manualChunks: undefined, // single bundle keeps cold-start small
      },
    },
  },
  server: {
    port: 5173,
    proxy: {
      '/api': 'http://127.0.0.1:8765',
    },
  },
});
