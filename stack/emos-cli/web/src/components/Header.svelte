<script lang="ts">
  import { Search } from 'lucide-svelte';
  import { useInfo, useRobot } from '$lib/queries';
  import Heartbeat from './Heartbeat.svelte';
  import ThemeToggle from './ThemeToggle.svelte';
  import { paletteOpen } from './CommandPalette.svelte';

  const info = useInfo();
  const robot = useRobot();

  // Title priority: a real robot manifest (model/name from licensed deployment)
  // wins; otherwise we use the friendly device identity from /info; the OS
  // hostname is a final fallback for legacy installs that haven't computed
  // an identity yet.
  let title = $derived(
    $robot.data?.name ||
      $robot.data?.model ||
      $info.data?.name ||
      $info.data?.hostname ||
      'EMOS'
  );
  let subtitle = $derived(
    $robot.data?.serial
      ? `Serial ${$robot.data.serial}`
      : $info.data
        ? `${$info.data.platform} · ${$info.data.mode ?? 'unconfigured'}`
        : 'connecting…'
  );

  const isMac = typeof navigator !== 'undefined' && /Mac|iPhone|iPad/.test(navigator.platform);
</script>

<header class="border-b border-emos-line/60 bg-emos-bg/40 backdrop-blur sticky top-0 z-30">
  <div class="max-w-7xl mx-auto px-6 md:px-8 py-4 flex items-center gap-4">
    <div class="min-w-0 flex-1">
      <div class="flex items-center gap-2">
        <Heartbeat />
        <h1 class="text-lg md:text-xl font-medium tracking-tight truncate">
          {title}
        </h1>
      </div>
      <div class="text-xs text-emos-text-3 truncate">{subtitle}</div>
    </div>

    <button
      class="hidden md:inline-flex items-center gap-2 px-3 py-1.5 rounded-lg text-sm text-emos-text-2 surface-2 hover:text-emos-text transition"
      onclick={() => paletteOpen.set(true)}
    >
      <Search size={14} />
      <span>Search recipes &amp; actions</span>
      <span class="kbd ml-2">{isMac ? '⌘' : 'Ctrl'} K</span>
    </button>

    <div class="ml-2 flex items-center gap-2">
      <ThemeToggle />
    </div>
  </div>
</header>
