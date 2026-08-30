<script lang="ts">
  import { Cloud, CloudOff, RefreshCw, Loader2, Bot, Radar, Download } from 'lucide-svelte';
  import { useQueryClient } from '@tanstack/svelte-query';
  import {
    usePluginsRemote,
    usePluginsInstalled,
    useInstallPlugin,
    useRemovePlugin,
    useJobs,
    useConnectivity,
    keys,
  } from '$lib/queries';
  import { ApiException, type CatalogPlugin, type InstalledPlugin } from '$lib/api';
  import { confirm as confirmDialog } from '$lib/dialog';
  import { renderMarkdown } from '$lib/markdown';
  import Empty from '$components/Empty.svelte';
  import PluginCard from '$components/PluginCard.svelte';

  const conn = useConnectivity();
  const qc = useQueryClient();
  const remote = usePluginsRemote();
  const installed = usePluginsInstalled();
  const jobs = useJobs();
  const install = useInstallPlugin();
  const remove = useRemovePlugin();

  const PLUGIN_JOBS = new Set(['plugin_install', 'plugin_remove']);

  // Slug of the plugin whose job we just requested, until the jobs list
  // reflects it. Installs and removals rebuild one shared overlay, so one at
  // a time.
  let pending = $state<string>('');

  let robot = $derived($installed.data?.robot ?? null);
  let sensors = $derived($installed.data?.sensors ?? []);
  let installedSlugs = $derived(
    new Set([robot?.slug, ...sensors.map((s) => s.slug)].filter(Boolean) as string[])
  );

  let runningJob = $derived(
    ($jobs.data ?? []).find((j) => PLUGIN_JOBS.has(j.kind) && j.status === 'running')
  );
  let anyBusy = $derived(!!pending || !!runningJob);

  // The job message for a plugin currently being installed or removed, or ''.
  function busyFor(slug: string): string {
    if (runningJob?.target === slug) return runningJob.message || 'working…';
    if (pending === slug) return 'starting…';
    return '';
  }

  async function report(title: string, err: unknown) {
    const msg = err instanceof ApiException ? err.message : String(err);
    await confirmDialog({ title, message: msg, confirmLabel: 'OK', cancelLabel: 'Dismiss' });
  }

  async function startInstall(p: CatalogPlugin) {
    if (anyBusy) return;
    if (p.role === 'robot' && robot && robot.slug !== p.slug) {
      const ok = await confirmDialog({
        title: `Replace ${robotName()} with ${p.name}?`,
        message: 'A robot runs one robot plugin. The current one is removed; sensor plugins stay.',
        confirmLabel: 'Replace',
        intent: 'destructive',
      });
      if (!ok) return;
    }
    pending = p.slug;
    try {
      await $install.mutateAsync(p.slug);
    } catch (err) {
      pending = '';
      await report('Could not start install', err);
    }
  }

  async function startRemove(plugin: InstalledPlugin) {
    if (anyBusy) return;
    const name = ((plugin.describe as any)?.metadata?.name as string) ?? plugin.slug;
    const ok = await confirmDialog({
      title: `Remove ${name}?`,
      message:
        plugin.role === 'sensor'
          ? 'The sensor plugin is removed from this robot. You can add it again from the catalog later.'
          : 'The robot plugin is removed from this robot. Sensor plugins stay installed.',
      confirmLabel: 'Remove',
      intent: 'destructive',
    });
    if (!ok) return;
    pending = plugin.slug;
    try {
      await $remove.mutateAsync(plugin.slug);
    } catch (err) {
      pending = '';
      await report('Could not start removal', err);
    }
  }

  function robotName(): string {
    return ((robot?.describe as any)?.metadata?.name as string) ?? robot?.slug ?? 'the current robot';
  }

  // Once the jobs list shows our job, the pending marker has done its work;
  // when a plugin job settles, refresh what is installed.
  let seen = new Set<string>();
  $effect(() => {
    for (const j of $jobs.data ?? []) {
      if (!PLUGIN_JOBS.has(j.kind)) continue;
      if (j.status === 'running' && j.target === pending) pending = '';
      if ((j.status === 'finished' || j.status === 'failed') && !seen.has(j.id)) {
        seen.add(j.id);
        if (j.target === pending) pending = '';
        qc.invalidateQueries({ queryKey: keys.pluginsInstalled });
        qc.invalidateQueries({ queryKey: keys.robot });
      }
    }
  });

  function installLabel(p: CatalogPlugin): string {
    if (installedSlugs.has(p.slug)) return 'Reinstall';
    if (p.role === 'sensor') return 'Add';
    return robot ? 'Replace robot' : 'Install';
  }

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
    An EMOS system can have <strong>one robot plugin</strong> plus any number of
    <strong>sensor plugins</strong>. Installing a robot plugin replaces the current one; sensor
    plugins are added alongside it and used for <strong>extra</strong> sensors in the environment or
    mounted on board the robot.
  </p>

  <!-- Installed: each section appears only once it has something in it, so a
       fresh robot goes straight to the catalog. -->
  {#if robot}
    <div class="text-xs uppercase tracking-wider text-emos-text-3 pt-1 flex items-center gap-1">
      <Bot size={12} /> Robot
    </div>
    <PluginCard plugin={robot} busy={busyFor(robot.slug)} onRemove={startRemove} />
  {/if}

  {#if sensors.length}
    <div class="text-xs uppercase tracking-wider text-emos-text-3 pt-1 flex items-center gap-1">
      <Radar size={12} /> Sensors
    </div>
    <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
      {#each sensors as s (s.slug)}
        <PluginCard plugin={s} busy={busyFor(s.slug)} onRemove={startRemove} />
      {/each}
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
        {@const isInstalled = installedSlugs.has(p.slug)}
        {@const busy = busyFor(p.slug)}
        <div class="surface p-5 space-y-2 flex flex-col">
          <div class="flex items-start justify-between gap-2">
            <div class="flex items-center gap-2">
              {#if p.role === 'sensor'}
                <Radar size={16} class="text-emos-text-3" />
              {:else}
                <Bot size={16} class="text-emos-text-3" />
              {/if}
              <div class="font-semibold">{p.name}</div>
            </div>
            <div class="flex items-center gap-1">
              <span class="pill">{p.role}</span>
              {#if isInstalled}<span class="pill pill-good">installed</span>{/if}
            </div>
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
                <Loader2 size={14} class="animate-spin" /> {busy}
              </button>
            {:else}
              <button class="btn btn-primary" onclick={() => startInstall(p)} disabled={anyBusy}>
                <Download size={14} /> {installLabel(p)}
              </button>
            {/if}
          </div>
        </div>
      {/each}
    </div>
  {:else}
    <Empty title="No plugins available" description="The catalog has no plugins yet." />
  {/if}
</section>
