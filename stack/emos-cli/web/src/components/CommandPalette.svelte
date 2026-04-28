<script lang="ts" module>
  import { writable } from 'svelte/store';
  // Exported store so the header button (and ⌘K) can open the palette.
  export const paletteOpen = writable(false);
</script>

<script lang="ts">
  import { onMount } from 'svelte';
  import { navigate } from '$lib/router';
  import { Search, Play, Download, ArrowRight } from 'lucide-svelte';
  import { useRecipesLocal, useRecipesRemote, useStartRun, usePullRecipe } from '$lib/queries';

  let q = $state('');
  let inputEl = $state<HTMLInputElement | undefined>(undefined);

  const local = useRecipesLocal();
  const remote = useRecipesRemote();
  const startRun = useStartRun();
  const pull = usePullRecipe();

  type Item = {
    id: string;
    label: string;
    sub: string;
    icon: typeof Play;
    run: () => Promise<void> | void;
  };

  let items = $derived.by(() => {
    const out: Item[] = [];
    const ql = q.trim().toLowerCase();

    for (const r of $local.data ?? []) {
      const label = r.display_name || r.name;
      if (ql && !label.toLowerCase().includes(ql) && !r.name.toLowerCase().includes(ql)) continue;
      out.push({
        id: 'local:' + r.name,
        label: 'Run ' + label,
        sub: 'Installed · ' + r.name,
        icon: Play,
        run: async () => {
          paletteOpen.set(false);
          navigate('/recipes/' + encodeURIComponent(r.name));
          $startRun.mutate({ recipe: r.name });
        },
      });
    }
    for (const r of $remote.data ?? []) {
      // Hide remote items that we already have locally to keep the list tight.
      if (($local.data ?? []).some((l) => l.name === r.name)) continue;
      const label = r.display_name || r.name;
      const haystack = (label + ' ' + r.name).toLowerCase();
      if (ql && !haystack.includes(ql)) continue;
      out.push({
        id: 'remote:' + r.name,
        label: 'Get ' + label,
        sub: 'Catalog · pull to this device',
        icon: Download,
        run: () => {
          paletteOpen.set(false);
          $pull.mutate(r.name);
        },
      });
    }
    out.push(
      { id: 'nav:recipes', label: 'Open Recipes', sub: 'Browse installed and catalog', icon: ArrowRight,
        run: () => { paletteOpen.set(false); navigate('/recipes'); } },
      { id: 'nav:runs', label: 'Open Runs', sub: 'See recent activity', icon: ArrowRight,
        run: () => { paletteOpen.set(false); navigate('/runs'); } },
      { id: 'nav:system', label: 'Open System', sub: 'Install mode, distro, QR', icon: ArrowRight,
        run: () => { paletteOpen.set(false); navigate('/system'); } },
    );
    return out.slice(0, 12);
  });

  let active = $state(0);
  $effect(() => {
    if (active >= items.length) active = 0;
  });

  function close() { paletteOpen.set(false); }

  function onKey(e: KeyboardEvent) {
    if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'k') {
      e.preventDefault();
      paletteOpen.update((v) => !v);
      return;
    }
    if (!$paletteOpen) return;
    if (e.key === 'Escape') close();
    else if (e.key === 'ArrowDown') { e.preventDefault(); active = Math.min(active + 1, items.length - 1); }
    else if (e.key === 'ArrowUp')   { e.preventDefault(); active = Math.max(active - 1, 0); }
    else if (e.key === 'Enter')     { e.preventDefault(); items[active]?.run(); }
  }

  onMount(() => {
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  });

  $effect(() => {
    if ($paletteOpen) {
      // tick after the modal mounts so the input exists.
      queueMicrotask(() => inputEl?.focus());
    }
  });
</script>

{#if $paletteOpen}
  <div
    class="fixed inset-0 z-50 flex items-start justify-center pt-[12vh] bg-black/50 backdrop-blur-sm"
    role="dialog"
    aria-modal="true"
    aria-label="Command palette"
    onclick={close}
    onkeydown={(e) => e.key === 'Escape' && close()}
    tabindex="-1"
  >
    <div
      class="surface w-[min(640px,90vw)] overflow-hidden animate-rise"
      onclick={(e) => e.stopPropagation()}
      role="presentation"
    >
      <div class="flex items-center gap-2 px-4 py-3 border-b border-emos-line/60">
        <Search size={16} class="text-emos-text-3" />
        <input
          bind:this={inputEl}
          bind:value={q}
          placeholder="Search recipes, run actions…"
          class="bg-transparent flex-1 outline-none text-sm placeholder:text-emos-text-3"
        />
        <span class="kbd">esc</span>
      </div>
      <ul class="max-h-[50vh] overflow-y-auto py-1">
        {#each items as item, i (item.id)}
          <li>
            <button
              type="button"
              class="w-full text-left px-4 py-2.5 flex items-center gap-3 transition"
              class:active={i === active}
              onmouseenter={() => (active = i)}
              onclick={() => item.run()}
            >
              <item.icon size={16} class="opacity-80" />
              <div class="min-w-0 flex-1">
                <div class="text-sm">{item.label}</div>
                <div class="text-xs text-emos-text-3 truncate">{item.sub}</div>
              </div>
            </button>
          </li>
        {/each}
        {#if items.length === 0}
          <li class="px-4 py-6 text-sm text-emos-text-3">No matches.</li>
        {/if}
      </ul>
    </div>
  </div>
{/if}

<style>
  button.active {
    background: color-mix(in oklab, var(--color-emos-surface-2) 90%, transparent);
  }
</style>
