import { mount } from 'svelte';
import App from './App.svelte';
import { initTheme } from './lib/theme';
import './styles/global.css';

// Apply persisted theme BEFORE Svelte mounts so the first paint matches —
// avoids a flash of dark/light when the user has the opposite preference.
initTheme();

const target = document.getElementById('app');
if (!target) {
  throw new Error('emos: #app element missing from index.html');
}

mount(App, { target });
