<script lang="ts">
  import { link, path } from '$lib/router';
  import { LayoutDashboard, BookMarked, Activity, Cpu, BookOpen, ExternalLink } from 'lucide-svelte';
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

<aside class="flex w-14 md:w-60 shrink-0 flex-col gap-1 p-2 md:p-4 border-r border-emos-line/60">
  <div class="hidden md:flex items-center gap-2 px-2 py-3">
    <!-- The EMOS wordmark IS the logo + text. No separate label needed. -->
    <Logo height={22} />
    <span class="text-xs text-emos-text-3 ml-auto pr-1">
      {$info.data?.version ? `v${$info.data.version}` : ''}
    </span>
  </div>

  <nav class="flex flex-col gap-1 mt-2 md:mt-4">
    {#each items as item}
      <a
        use:link
        href={item.href}
        title={item.label}
        class:active={isActive(item.href)}
        class="flex items-center justify-center md:justify-start gap-3 px-2 md:px-3 py-2 rounded-lg text-sm text-emos-text-2 hover:text-emos-text hover:bg-emos-surface-2/60 transition"
      >
        <item.icon size={18} class="opacity-80" />
        <span class="hidden md:inline">{item.label}</span>
      </a>
    {/each}
  </nav>

  <!-- Footer: docs + community -->
  <div class="mt-auto pt-3 border-t border-emos-line/40 flex flex-col gap-1">
    <a
      href="https://emos.automatikarobotics.com"
      target="_blank"
      rel="noreferrer"
      title="Documentation"
      class="footer-link"
    >
      <BookOpen size={14} class="opacity-80" />
      <span class="hidden md:inline">Docs</span>
      <ExternalLink size={10} class="hidden md:inline ml-auto opacity-50" />
    </a>
    <a
      href="https://discord.gg/B9ZU6qjzND"
      target="_blank"
      rel="noreferrer"
      title="Join the EMOS Discord"
      class="footer-link discord"
    >
      <!-- Discord brand mark, scaled to match the lucide icon weight. -->
      <svg viewBox="0 0 24 24" fill="currentColor" width="14" height="14" aria-hidden="true">
        <path d="M20.317 4.37a19.79 19.79 0 0 0-4.885-1.515.075.075 0 0 0-.079.037c-.21.375-.444.864-.608 1.249a18.27 18.27 0 0 0-5.487 0 12.65 12.65 0 0 0-.617-1.25.077.077 0 0 0-.079-.037A19.736 19.736 0 0 0 3.677 4.37a.07.07 0 0 0-.032.027C.533 9.046-.32 13.58.099 18.057a.082.082 0 0 0 .031.057 19.9 19.9 0 0 0 5.993 3.03.078.078 0 0 0 .084-.028c.462-.63.874-1.295 1.226-1.994a.076.076 0 0 0-.041-.106 13.107 13.107 0 0 1-1.872-.892.077.077 0 0 1-.008-.128 10.2 10.2 0 0 0 .372-.292.074.074 0 0 1 .077-.01c3.928 1.793 8.18 1.793 12.062 0a.074.074 0 0 1 .078.01c.12.098.246.198.373.292a.077.077 0 0 1-.006.127 12.299 12.299 0 0 1-1.873.892.077.077 0 0 0-.041.107c.36.698.772 1.362 1.225 1.993a.076.076 0 0 0 .084.028 19.839 19.839 0 0 0 6.002-3.03.077.077 0 0 0 .032-.054c.5-5.177-.838-9.674-3.549-13.66a.061.061 0 0 0-.031-.029ZM8.02 15.331c-1.182 0-2.157-1.085-2.157-2.419 0-1.333.956-2.419 2.157-2.419 1.21 0 2.176 1.096 2.157 2.42 0 1.333-.956 2.418-2.157 2.418Zm7.974 0c-1.182 0-2.157-1.085-2.157-2.419 0-1.333.955-2.419 2.157-2.419 1.21 0 2.176 1.096 2.157 2.42 0 1.333-.946 2.418-2.157 2.418Z" />
      </svg>
      <span class="hidden md:inline">Discord</span>
      <ExternalLink size={10} class="hidden md:inline ml-auto opacity-50" />
    </a>
  </div>
</aside>

<style>
  a.active {
    color: var(--color-emos-text);
    background: color-mix(in oklab, var(--color-emos-surface-2) 80%, transparent);
    box-shadow: inset 2px 0 0 var(--color-emos-accent);
  }

  .footer-link {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 0.5rem;
    padding: 0.5rem;
    border-radius: 0.5rem;
    font-size: 0.75rem;
    color: var(--color-emos-text-3);
    transition: color 0.15s ease, background 0.15s ease;
  }
  @media (min-width: 768px) {
    .footer-link {
      justify-content: flex-start;
      padding: 0.5rem 0.75rem;
    }
  }
  .footer-link:hover {
    color: var(--color-emos-text);
    background: color-mix(in oklab, var(--color-emos-surface-2) 60%, transparent);
  }
  /* Discord row picks up the brand colour on hover */
  .footer-link.discord:hover {
    color: #5865f2;
  }
</style>
