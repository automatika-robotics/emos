<script lang="ts">
  import { Sun, Moon, Monitor } from 'lucide-svelte';
  import { themeChoice, cycleTheme } from '$lib/theme';

  // Three-state cycle: system → light → dark → system.
  // Icon shows the *current* mode so users always know what they're in.
  const labelFor: Record<string, string> = {
    system: 'System theme — click to switch to light',
    light: 'Light theme — click to switch to dark',
    dark: 'Dark theme — click to follow system',
  };
</script>

<button
  type="button"
  class="inline-flex items-center justify-center rounded-lg w-8 h-8 text-emos-text-2 hover:text-emos-text hover:bg-emos-surface-2/60 transition"
  aria-label={labelFor[$themeChoice]}
  title={labelFor[$themeChoice]}
  onclick={cycleTheme}
>
  {#if $themeChoice === 'system'}
    <Monitor size={16} />
  {:else if $themeChoice === 'light'}
    <Sun size={16} />
  {:else}
    <Moon size={16} />
  {/if}
</button>
