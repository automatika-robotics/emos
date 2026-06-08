<script lang="ts">
  import { Cloud, CloudOff, RefreshCw, Loader2, Plug, Trash2, Download, Cpu } from 'lucide-svelte';
  import { useQueryClient } from '@tanstack/svelte-query';
  import {
    usePluginsRemote,
    usePluginActive,
    useInstallPlugin,
    useRemovePlugin,
    useJobs,
    useConnectivity,
    keys,
  } from '$lib/queries';
  import { ApiException } from '$lib/api';
  import { confirm as confirmDialog } from '$lib/dialog';
  import { renderMarkdown } from '$lib/markdown';
  import Empty from '$components/Empty.svelte';

  const conn = useConnectivity();
  const qc = useQueryClient();
  const remote = usePluginsRemote();
  const active = usePluginActive();
  const jobs = useJobs();
  const install = useInstallPlugin();
  const remove = useRemovePlugin();

  // At most one install runs at a time (one plugin per robot).
  let pending = $state<string>('');

  async function startInstall(slug: string) {
    if (pending) return;
    pending = slug;
    try {
      await $install.mutateAsync(slug);
      setTimeout(() => {
        if (pending === slug) pending = '';
      }, 5000);
    } catch (err) {
      pending = '';
      const msg = err instanceof ApiException ? err.message : String(err);
      await confirmDialog({
        title: 'Could not start install',
        message: msg,
        confirmLabel: 'OK',
        cancelLabel: 'Dismiss',
      });
    }
  }

  async function removeActive() {
    const ok = await confirmDialog({
      title: 'Remove the active plugin?',
      message: 'The plugin is removed from this robot. You can install it again from the catalog later.',
      confirmLabel: 'Remove',
      intent: 'destructive',
    });
    if (ok) $remove.mutate();
  }

  // Watch plugin_install jobs; refresh the active plugin + robot identity when
  // one settles.
  let seen = new Set<string>();
  $effect(() => {
    for (const j of $jobs.data ?? []) {
      if (j.kind !== 'plugin_install') continue;
      if ((j.status === 'finished' || j.status === 'failed') && !seen.has(j.id)) {
        seen.add(j.id);
        pending = '';
        qc.invalidateQueries({ queryKey: keys.pluginActive });
        qc.invalidateQueries({ queryKey: keys.robot });
      }
    }
  });

  let installJob = $derived(
    ($jobs.data ?? []).find((j) => j.kind === 'plugin_install' && j.status === 'running')
  );

  let activeSlug = $derived($active.data?.slug ?? '');
  let meta = $derived(($active.data?.describe as any)?.metadata ?? null);
  let actions = $derived(
    ((($active.data?.describe as any)?.actions ?? []) as any[]).map((a) => a.name).filter(Boolean)
  );
  let events = $derived(
    ((($active.data?.describe as any)?.events ?? []) as any[]).map((e) => e.name).filter(Boolean)
  );

  function isOffline(): boolean {
    return $remote.error instanceof ApiException && $remote.error.code === 'offline';
  }
</script>

<section class="space-y-5">
  <div class="flex items-end justify-between gap-3 flex-wrap">
    <div>
      <div class="text-xs uppercase tracking-wider text-emos-text-3">Robot</div>
      <h2 class="text-2xl font-semibold tracking-tight">Plugins</h2>
    </div>
    <div class="flex items-center gap-2">
      {#if $conn.data}
        {#if $conn.data.online}
          <span class="pill pill-good" title="Connected to the Automatika catalog">
            <Cloud size={12} /> cloud
          </span>
        {:else}
          <span class="pill pill-warn" title="No internet — the catalog is unavailable">
            <CloudOff size={12} /> offline
          </span>
        {/if}
      {/if}
      <button
        class="btn btn-ghost"
        onclick={() => $remote.refetch()}
        disabled={$remote.isFetching}
        aria-label="refresh catalog"
      >
        {#if $remote.isFetching}<Loader2 size={14} class="animate-spin" />{:else}<RefreshCw size={14} />{/if}
      </button>
    </div>
  </div>

  <p class="text-sm text-emos-text-3 max-w-2xl">
    A robot plugin adapts a specific robot to the EMOS stack. A robot runs
    <strong>one plugin at a time</strong>; installing a new one replaces the active one.
  </p>

  <!-- Active plugin -->
  {#if $active.data}
    <div class="surface p-5 space-y-3">
      <div class="flex items-start justify-between gap-3">
        <div class="flex items-center gap-2">
          <Plug size={16} class="text-emos-accent" />
          <div>
            <div class="font-semibold">{meta?.name ?? activeSlug}</div>
            <div class="text-xs text-emos-text-3 font-mono">{$active.data.entry_point}</div>
          </div>
        </div>
        <span class="pill pill-good">● active</span>
      </div>
      {#if meta?.vendor || meta?.version}
        <div class="text-sm text-emos-text-2">
          {meta?.vendor ?? ''}{meta?.version ? ` · v${meta.version}` : ''}
        </div>
      {/if}
      {#if meta?.description}
        <div class="md-content md-compact">{@html renderMarkdown(meta.description)}</div>
      {/if}
      {#if actions.length}
        <div class="text-sm">
          <span class="text-emos-text-3">actions: </span>
          {#each actions as a (a)}<span class="pill mr-1">{a}</span>{/each}
        </div>
      {/if}
      {#if events.length}
        <div class="text-sm">
          <span class="text-emos-text-3">events: </span>
          {#each events as e (e)}<span class="pill mr-1">{e}</span>{/each}
        </div>
      {/if}
      <div class="pt-1">
        <button class="btn btn-ghost" onclick={removeActive} disabled={$remove.isPending}>
          <Trash2 size={14} /> Remove plugin
        </button>
      </div>
    </div>
  {/if}

  <!-- Catalog -->
  <div class="text-xs uppercase tracking-wider text-emos-text-3 pt-1">Catalog</div>

  {#if isOffline()}
    <Empty
      icon={CloudOff}
      title="You're offline"
      description="Connect this robot to the internet to browse the plugin catalog."
    >
      {#snippet actions()}
        <button class="btn btn-ghost" onclick={() => $remote.refetch()}>Try again</button>
      {/snippet}
    </Empty>
  {:else if $remote.isLoading}
    <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
      {#each Array(2) as _, i (i)}
        <div class="surface h-36 shimmer"></div>
      {/each}
    </div>
  {:else if ($remote.data ?? []).length}
    <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
      {#each $remote.data ?? [] as p (p.slug)}
        {@const isActive = p.slug === activeSlug}
        {@const busy = (pending === p.slug) || (installJob?.target === p.slug)}
        <div class="surface p-5 space-y-2 flex flex-col">
          <div class="flex items-start justify-between gap-2">
            <div class="flex items-center gap-2">
              <Cpu size={16} class="text-emos-text-3" />
              <div class="font-semibold">{p.name}</div>
            </div>
            {#if isActive}<span class="pill pill-good">active</span>{/if}
          </div>
          {#if p.vendor}<div class="text-xs text-emos-text-3">{p.vendor}</div>{/if}
          {#if p.description}
            <div class="md-content md-compact line-clamp-3">{@html renderMarkdown(p.description)}</div>
          {/if}
          {#if p.tags?.length}
            <div class="flex flex-wrap gap-1">
              {#each p.tags as t (t)}<span class="pill text-[0.7rem]">{t}</span>{/each}
            </div>
          {/if}
          <div class="mt-auto pt-2">
            {#if busy}
              <button class="btn btn-ghost" disabled>
                <Loader2 size={14} class="animate-spin" />
                {installJob?.message || 'installing…'}
              </button>
            {:else}
              <button class="btn btn-primary" onclick={() => startInstall(p.slug)} disabled={!!pending}>
                <Download size={14} /> {isActive ? 'Reinstall' : 'Install'}
              </button>
            {/if}
          </div>
        </div>
      {/each}
    </div>
  {:else}
    <Empty title="No plugins available" description="The catalog has no robot plugins yet." />
  {/if}
</section>
