<script lang="ts">
  import { link, path } from '$lib/router';
  import { LayoutDashboard, BookMarked, Activity, Cpu, LifeBuoy } from 'lucide-svelte';
  import Logo from './Logo.svelte';
  import { useInfo } from '$lib/queries';

  const info = useInfo();

  const items = [
    { href: '/', label: 'Console', icon: LayoutDashboard },
    { href: '/recipes', label: 'Recipes', icon: BookMarked },
    { href: '/runs', label: 'Runs', icon: Activity },
    { href: '/system', label: 'System', icon: Cpu },
  ];

  function isActive(href: string): boolean {
    if (href === '/') return $path === '/' || $path === '';
    return $path.startsWith(href);
  }
</script>

<aside class="hidden md:flex w-60 shrink-0 flex-col gap-1 p-4 border-r border-emos-line/60">
  <div class="flex items-center gap-2 px-2 py-3">
    <!-- The EMOS wordmark IS the logo + text. No separate label needed. -->
    <Logo height={22} />
    <span class="text-xs text-emos-text-3 ml-auto pr-1">
      {$info.data?.version ? `v${$info.data.version}` : ''}
    </span>
  </div>

  <nav class="flex flex-col gap-1 mt-4">
    {#each items as item}
      <a
        use:link
        href={item.href}
        class:active={isActive(item.href)}
        class="flex items-center gap-3 px-3 py-2 rounded-lg text-sm text-emos-text-2 hover:text-emos-text hover:bg-emos-surface-2/60 transition"
      >
        <item.icon size={18} class="opacity-80" />
        {item.label}
      </a>
    {/each}
  </nav>

  <div class="mt-auto px-2 py-3 text-xs text-emos-text-3 flex items-center gap-2">
    <LifeBuoy size={14} />
    <a
      href="https://emos.automatikarobotics.com"
      target="_blank"
      rel="noreferrer"
      class="hover:text-emos-text transition"
    >
      docs &amp; support
    </a>
  </div>
</aside>

<style>
  a.active {
    color: var(--color-emos-text);
    background: color-mix(in oklab, var(--color-emos-surface-2) 80%, transparent);
    box-shadow: inset 2px 0 0 var(--color-emos-accent);
  }
</style>
