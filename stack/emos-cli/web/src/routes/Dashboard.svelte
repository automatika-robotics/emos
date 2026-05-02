<script lang="ts">
  import { link, navigate } from '$lib/router';
  import { Cpu, BookMarked, Activity, Sparkles, ArrowUpRight, Zap, Box, FolderOpen } from 'lucide-svelte';
  import { useInfo, useRecipesLocal, useRuns, useRobot, useCapabilities, useStartRun } from '$lib/queries';
  import { ApiException } from '$lib/api';
  import { confirm as confirmDialog } from '$lib/dialog';
  import { formatDuration, relTime } from '$lib/time';
  import StatusTile from '$components/StatusTile.svelte';
  import Empty from '$components/Empty.svelte';
  import RecipeCard from '$components/RecipeCard.svelte';

  const info = useInfo();
  const local = useRecipesLocal();
  const runs = useRuns();
  const robot = useRobot();
  const caps = useCapabilities();
  const startRun = useStartRun();

  let activeRun = $derived(($runs.data ?? []).find((r) => r.status === 'running' || r.status === 'preparing'));
  let recentRuns = $derived(($runs.data ?? []).slice(0, 4));
  let featured = $derived(($local.data ?? []).slice(0, 3));

  async function runRecipe(name: string) {
    try {
      const res = await $startRun.mutateAsync({ recipe: name });
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
</script>

<section class="space-y-6">
  <!-- Hero / identity -->
  <div class="flex items-end justify-between gap-4 mb-2">
    <div>
      <div class="text-xs uppercase tracking-wider text-emos-text-3">EMOS Console</div>
      <h2 class="text-2xl md:text-3xl font-semibold tracking-tight">
        {#if $robot.data?.name || $robot.data?.model}
          {$robot.data.name ?? $robot.data.model}
        {:else if $info.data?.name}
          {$info.data.name}
        {:else if $info.data}
          {$info.data.hostname}
        {:else}
          Welcome
        {/if}
      </h2>
      <div class="text-sm text-emos-text-3 mt-1">
        {#if $robot.data}
          {$robot.data.kinematics ?? 'robot'}{$robot.data.serial ? ` · ${$robot.data.serial}` : ''}
        {:else if $info.data}
          {$info.data.platform} · {$info.data.mode ?? 'unconfigured'} · ROS {$info.data.ros_distro ?? '—'}
        {/if}
      </div>
    </div>
    {#if $caps.data?.has_robot_identity === false && $robot.data === null}
      <span class="pill"><Sparkles size={12} /> generic device</span>
    {/if}
  </div>

  <!-- Status tiles -->
  <div class="grid grid-cols-2 md:grid-cols-4 gap-4">
    <StatusTile
      icon={Box}
      label="Install mode"
      value={$info.data?.mode ?? '—'}
      sub={$info.data?.installed ? 'configured' : 'not installed'}
    />
    <StatusTile icon={Cpu} label="ROS" value={$info.data?.ros_distro ?? '—'} sub={$info.data?.platform ?? ''} />
    <StatusTile
      icon={BookMarked}
      label="Recipes installed"
      value={$local.data?.length ?? 0}
      sub={($local.data?.length ?? 0) ? 'browse to run' : 'pull one to begin'}
    />
    <StatusTile
      icon={Activity}
      label="Activity"
      value={activeRun ? 'running' : `${($runs.data ?? []).length} runs`}
      tone={activeRun ? 'good' : 'neutral'}
      sub={activeRun ? formatDuration(activeRun.started_at) : 'idle'}
    />
  </div>

  <!-- Active run highlight -->
  {#if activeRun}
    <a
      use:link
      href={'/runs/' + activeRun.id}
      class="surface p-5 flex items-center gap-4 group transition hover:-translate-y-0.5"
    >
      <div class="rounded-xl p-3 bg-emos-surface-2 text-emos-accent">
        <Zap size={18} />
      </div>
      <div class="min-w-0 flex-1">
        <div class="text-xs text-emos-text-3 uppercase tracking-wider">Now running</div>
        <div class="text-base font-medium truncate">{activeRun.recipe}</div>
        <div class="text-xs text-emos-text-3">started {relTime(activeRun.started_at)} · {formatDuration(activeRun.started_at)}</div>
      </div>
      <ArrowUpRight class="text-emos-text-3 group-hover:text-emos-text transition" />
    </a>
  {/if}

  <!-- Featured installed recipes -->
  <div>
    <div class="flex items-center justify-between mb-3">
      <h3 class="text-sm uppercase tracking-wider text-emos-text-3">Quick run</h3>
      <a use:link href="/recipes" class="text-xs text-emos-text-2 hover:text-emos-text inline-flex items-center gap-1">
        all recipes <ArrowUpRight size={12} />
      </a>
    </div>
    {#if featured.length}
      <div class="grid grid-cols-1 md:grid-cols-3 gap-4">
        {#each featured as r}
          <RecipeCard
            recipe={r}
            state="installed"
            busy={!!activeRun}
            onRun={() => runRecipe(r.name)}
          />
        {/each}
      </div>
    {:else}
      <Empty
        icon={FolderOpen}
        title="No recipes yet"
        description="Pull one from the catalog to give your robot its first behaviour."
      >
        {#snippet actions()}
          <a use:link href="/recipes" class="btn btn-primary">
            Browse catalog <ArrowUpRight size={14} />
          </a>
        {/snippet}
      </Empty>
    {/if}
  </div>

  <!-- Recent runs -->
  {#if recentRuns.length}
    <div>
      <div class="flex items-center justify-between mb-3">
        <h3 class="text-sm uppercase tracking-wider text-emos-text-3">Recent runs</h3>
        <a use:link href="/runs" class="text-xs text-emos-text-2 hover:text-emos-text inline-flex items-center gap-1">
          all runs <ArrowUpRight size={12} />
        </a>
      </div>
      <div class="surface divide-y divide-emos-line/60 overflow-hidden">
        {#each recentRuns as r}
          <a
            use:link
            href={'/runs/' + r.id}
            class="flex items-center gap-4 px-5 py-3 hover:bg-emos-surface-2/40 transition"
          >
            <span
              class="w-2 h-2 rounded-full"
              class:bg-good={r.status === 'finished'}
              class:bg-bad={r.status === 'failed'}
              class:bg-warn={r.status === 'canceled'}
              class:bg-accent={r.status === 'running' || r.status === 'preparing'}
            ></span>
            <div class="min-w-0 flex-1">
              <div class="text-sm truncate">{r.recipe}</div>
              <div class="text-xs text-emos-text-3">
                {relTime(r.started_at)} · {formatDuration(r.started_at, r.finished_at)}
              </div>
            </div>
            <span class="pill text-[0.7rem]">{r.status}</span>
          </a>
        {/each}
      </div>
    </div>
  {/if}
</section>

<style>
  .bg-good { background: var(--color-emos-good); }
  .bg-bad { background: var(--color-emos-bad); }
  .bg-warn { background: var(--color-emos-warn); }
  .bg-accent { background: var(--color-emos-accent); }
</style>
