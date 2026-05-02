<script lang="ts">
  import { Cloud, CloudOff, Search, RefreshCw, BookMarked, Loader2 } from 'lucide-svelte';
  import { useQueryClient } from '@tanstack/svelte-query';
  import {
    useRecipesLocal,
    useRecipesRemote,
    useRuns,
    useJobs,
    useStartRun,
    usePullRecipe,
    useDeleteRecipe,
    useConnectivity,
    keys,
  } from '$lib/queries';
  import { ApiException } from '$lib/api';
  import { navigate } from '$lib/router';
  import { confirm as confirmDialog } from '$lib/dialog';
  import RecipeCard from '$components/RecipeCard.svelte';
  import Empty from '$components/Empty.svelte';

  const conn = useConnectivity();
  const qc = useQueryClient();

  // Optimistic in-flight set. A click on Get adds the recipe name here
  // immediately so the card flips to "pulling" without waiting for the
  // jobs poll to catch up. The entry stays until either:
  //   - the polled jobs list shows the corresponding job (running/failed/done)
  //   - or the mutation rejects (network error before the job was created)
  let pendingPulls = $state(new Set<string>());

  async function startPull(name: string) {
    if (pendingPulls.has(name)) return;
    pendingPulls.add(name);
    pendingPulls = new Set(pendingPulls); // poke reactivity
    try {
      await $pull.mutateAsync(name);
      // Once mutation resolves the backend has registered the job; the
      // /jobs polling will pick it up within ~2s. Auto-clear the
      // optimistic flag after a grace period as a safety net.
      setTimeout(() => {
        pendingPulls.delete(name);
        pendingPulls = new Set(pendingPulls);
      }, 4000);
    } catch (err) {
      pendingPulls.delete(name);
      pendingPulls = new Set(pendingPulls);
      const msg = err instanceof ApiException ? err.message : String(err);
      await confirmDialog({
        title: 'Could not start pull',
        message: msg,
        confirmLabel: 'OK',
        cancelLabel: 'Dismiss',
      });
    }
  }

  async function runRecipe(name: string) {
    try {
      const res = await $startRun.mutateAsync({ recipe: name });
      // Drop straight into the live console, pre-flight progress streams
      // there as "[setup] ..." log lines.
      navigate('/runs/' + res.id);
    } catch (err) {
      const msg = err instanceof ApiException ? err.message : String(err);
      await confirmDialog({
        title: 'Could not start run',
        message: msg,
        confirmLabel: 'OK',
        cancelLabel: 'Dismiss',
      });
    }
  }

  async function deleteRecipe(name: string) {
    const ok = await confirmDialog({
      title: `Remove ${name}?`,
      message: 'The recipe will be deleted from this device. You can pull it again from the catalog later.',
      confirmLabel: 'Remove',
      intent: 'destructive',
    });
    if (ok) $remove.mutate(name);
  }

  type Tab = 'installed' | 'catalog';
  let tab = $state<Tab>('installed');
  let q = $state('');

  const local = useRecipesLocal();
  const remote = useRecipesRemote();
  const runs = useRuns();
  const jobs = useJobs();
  const startRun = useStartRun();
  const pull = usePullRecipe();
  const remove = useDeleteRecipe();

  let activeRun = $derived(($runs.data ?? []).find((r) => r.status === 'running' || r.status === 'preparing'));

  // Watch the polled jobs list. As soon as a recipe_pull transitions to
  // "finished", refresh the local recipes list so the just-installed recipe
  // appears in the Installed tab. Use a Set so each job triggers exactly
  // one invalidation no matter how many times the query re-fetches.
  let invalidatedJobIds = new Set<string>();
  $effect(() => {
    for (const j of $jobs.data ?? []) {
      if (j.kind !== 'recipe_pull') continue;
      if (j.status === 'finished' && !invalidatedJobIds.has(j.id)) {
        invalidatedJobIds.add(j.id);
        qc.invalidateQueries({ queryKey: keys.recipesLocal });
        // Also clear our optimistic flag for the recipe — keeps the
        // catalog tidy if the user hops back to it.
        if (pendingPulls.has(j.target)) {
          pendingPulls.delete(j.target);
          pendingPulls = new Set(pendingPulls);
        }
      }
    }
  });

  // Map recipe-name → in-flight pull job, so cards can animate.
  let pullJobByName = $derived.by(() => {
    const m = new Map<string, any>();
    for (const j of $jobs.data ?? []) {
      if (j.kind === 'recipe_pull' && j.status === 'running') m.set(j.target, j);
      // Also include recently-failed so the card can show the error.
      if (j.kind === 'recipe_pull' && j.status === 'failed' && Date.now() - new Date(j.started_at).getTime() < 30000)
        m.set(j.target, j);
    }
    return m;
  });

  let installedFiltered = $derived.by(() => {
    const list = $local.data ?? [];
    const ql = q.trim().toLowerCase();
    if (!ql) return list;
    return list.filter(
      (r) =>
        r.name.toLowerCase().includes(ql) ||
        (r.display_name ?? '').toLowerCase().includes(ql) ||
        (r.description ?? '').toLowerCase().includes(ql)
    );
  });

  let catalogFiltered = $derived.by(() => {
    const installed = new Set(($local.data ?? []).map((l) => l.name));
    const list = ($remote.data ?? []).filter((r) => !installed.has(r.name));
    const ql = q.trim().toLowerCase();
    if (!ql) return list;
    return list.filter((r) => r.name.toLowerCase().includes(ql));
  });

  function isOffline(): boolean {
    return $remote.error instanceof ApiException && $remote.error.code === 'offline';
  }
</script>

<section class="space-y-5">
  <div class="flex items-end justify-between gap-3 flex-wrap">
    <div>
      <div class="text-xs uppercase tracking-wider text-emos-text-3">Library</div>
      <h2 class="text-2xl font-semibold tracking-tight">Recipes</h2>
    </div>

    <div class="flex items-center gap-2">
      <div class="surface-2 flex items-center p-1 rounded-xl">
        {#each ['installed', 'catalog'] as t (t)}
          <button
            onclick={() => (tab = t as Tab)}
            class:active={tab === t}
            class="px-3 py-1.5 rounded-lg text-sm transition capitalize"
          >
            {t}
            {#if t === 'installed'}
              <span class="ml-1 text-xs text-emos-text-3">{($local.data ?? []).length}</span>
            {:else if t === 'catalog' && $remote.data}
              <span class="ml-1 text-xs text-emos-text-3">{catalogFiltered.length}</span>
            {/if}
          </button>
        {/each}
      </div>
      <div class="relative">
        <Search size={14} class="absolute left-3 top-1/2 -translate-y-1/2 text-emos-text-3" />
        <input bind:value={q} placeholder="search" class="input pl-8 py-2 text-sm w-56" />
      </div>
      {#if tab === 'catalog'}
        {#if $conn.data}
          {#if $conn.data.online}
            <span class="pill pill-good" title="Connected to the Automatika catalog">
              <Cloud size={12} /> cloud
            </span>
          {:else}
            <span class="pill pill-warn" title="No internet — catalog and pulls are unavailable">
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
      {/if}
    </div>
  </div>

  {#if tab === 'installed'}
    {#if installedFiltered.length}
      <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
        {#each installedFiltered as r (r.name)}
          <RecipeCard
            recipe={r}
            state="installed"
            busy={!!activeRun}
            onRun={() => runRecipe(r.name)}
            onDelete={() => deleteRecipe(r.name)}
          />
        {/each}
      </div>
    {:else if q}
      <Empty title="No matches" description={`No installed recipes match "${q}".`} />
    {:else}
      <Empty
        icon={BookMarked}
        title="No recipes installed yet"
        description="Browse the catalog and pull your first recipe — most install in seconds."
      >
        {#snippet actions()}
          <button class="btn btn-primary" onclick={() => (tab = 'catalog')}>Open catalog</button>
        {/snippet}
      </Empty>
    {/if}
  {:else if isOffline()}
    <Empty
      icon={CloudOff}
      title="You're offline"
      description="Connect this robot to the internet to browse the catalog. Recipes already on this device still run."
    >
      {#snippet actions()}
        <button class="btn btn-ghost" onclick={() => $remote.refetch()}>Try again</button>
      {/snippet}
    </Empty>
  {:else if $remote.isLoading}
    <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
      {#each Array(6) as _, i (i)}
        <div class="surface h-44 shimmer"></div>
      {/each}
    </div>
  {:else if catalogFiltered.length}
    <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
      {#each catalogFiltered as r (r.name)}
        {@const job = pullJobByName.get(r.name)}
        {@const optimistic = pendingPulls.has(r.name)}
        {@const cardState = job
          ? (job.status === 'failed' ? 'failed' : 'pulling')
          : (optimistic ? 'pulling' : 'remote')}
        <RecipeCard
          recipe={r}
          state={cardState}
          {job}
          busy={cardState === 'pulling'}
          onPull={() => startPull(r.name)}
        />
      {/each}
    </div>
  {:else}
    <Empty title="Catalog is up to date" description="Every catalog recipe is already installed." />
  {/if}
</section>

<style>
  button.active {
    background: color-mix(in oklab, var(--color-emos-bg) 60%, transparent);
    color: var(--color-emos-text);
    box-shadow: 0 0 0 1px var(--color-emos-line);
  }
</style>
