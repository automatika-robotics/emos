<script lang="ts">
  import { Bot, Radar, Trash2, Loader2 } from 'lucide-svelte';
  import type { InstalledPlugin } from '$lib/api';
  import { renderMarkdown } from '$lib/markdown';

  // An installed plugin on the Plugins page: what it is, where it came from,
  // and a count of what it provides. The full feeds, actions and events lists
  // live on the System page.
  let {
    plugin,
    busy = '',
    onRemove,
  }: {
    plugin: InstalledPlugin;
    busy?: string; // non-empty while a job targets this plugin: the job's message
    onRemove: (plugin: InstalledPlugin) => void;
  } = $props();

  const count = (items: unknown): number => ((items ?? []) as unknown[]).length;
  const plural = (n: number, word: string) => `${n} ${word}${n === 1 ? '' : 's'}`;

  let d = $derived((plugin.describe as any) ?? {});
  let meta = $derived(d.metadata ?? null);
  let isSensor = $derived(plugin.role === 'sensor');
  let installed = $derived(plugin.installed_at ? new Date(plugin.installed_at).toLocaleDateString() : '');
  let summary = $derived(
    [
      count(d.feedbacks) ? plural(count(d.feedbacks), 'feed') : '',
      count(d.actions) ? plural(count(d.actions), 'action') : '',
      count(d.events) ? plural(count(d.events), 'event') : '',
    ]
      .filter(Boolean)
      .join(' · ')
  );
</script>

<div class="surface p-5 space-y-3">
  <div class="flex items-start justify-between gap-3">
    <div class="flex items-center gap-2">
      {#if isSensor}
        <Radar size={16} class="text-emos-accent" />
      {:else}
        <Bot size={16} class="text-emos-accent" />
      {/if}
      <div>
        <div class="font-semibold">{meta?.name ?? plugin.slug}</div>
        {#if meta?.vendor}<div class="text-xs text-emos-text-3">{meta.vendor}</div>{/if}
      </div>
    </div>
    <span class="pill">{plugin.role}</span>
  </div>

  {#if meta?.description}
    <div class="md-content md-compact">{@html renderMarkdown(meta.description)}</div>
  {/if}

  <div class="grid grid-cols-2 gap-y-1 text-sm">
    {#if meta?.version}<div class="text-emos-text-3">version</div><div class="font-mono">{meta.version}</div>{/if}
    <div class="text-emos-text-3">entry point</div>
    <div class="font-mono truncate" title={plugin.entry_point}>{plugin.entry_point}</div>
    <div class="text-emos-text-3">repository</div>
    <div class="font-mono truncate" title={plugin.repo}>{plugin.repo}{plugin.ref ? ` @ ${plugin.ref}` : ''}</div>
    {#if installed}<div class="text-emos-text-3">installed</div><div>{installed}</div>{/if}
    {#if plugin.sources?.length}
      <div class="text-emos-text-3">built from source</div>
      <div class="font-mono">{plugin.sources.join(', ')}</div>
    {/if}
  </div>

  {#if summary}
    <div class="text-sm text-emos-text-3">{summary}</div>
  {/if}

  <div class="pt-1">
    {#if busy}
      <button class="btn btn-ghost" disabled>
        <Loader2 size={14} class="animate-spin" /> {busy}
      </button>
    {:else}
      <button class="btn btn-ghost" onclick={() => onRemove(plugin)}>
        <Trash2 size={14} /> Remove
      </button>
    {/if}
  </div>
</div>
