<script lang="ts">
  import { link } from '$lib/router';
  import { Activity } from 'lucide-svelte';
  import { useRuns } from '$lib/queries';
  import { formatDuration, relTime } from '$lib/time';
  import Empty from '$components/Empty.svelte';

  const runs = useRuns();
</script>

<section class="space-y-5">
  <div>
    <div class="text-xs uppercase tracking-wider text-emos-text-3">Activity</div>
    <h2 class="text-2xl font-semibold tracking-tight">Runs</h2>
    <p class="text-sm text-emos-text-3 mt-1">Recent recipe executions on this device.</p>
  </div>

  {#if $runs.isLoading}
    <div class="surface h-40 shimmer"></div>
  {:else if !($runs.data?.length)}
    <Empty
      icon={Activity}
      title="No runs yet"
      description="When you launch a recipe, it'll show up here with live logs and an exit status."
    />
  {:else}
    <div class="surface divide-y divide-emos-line/60 overflow-hidden">
      {#each $runs.data as r (r.id)}
        <a
          use:link
          href={'/runs/' + r.id}
          class="grid grid-cols-12 items-center gap-3 px-5 py-3 hover:bg-emos-surface-2/40 transition"
        >
          <span
            class="col-span-1 w-2.5 h-2.5 rounded-full"
            class:bg-good={r.status === 'finished'}
            class:bg-bad={r.status === 'failed'}
            class:bg-warn={r.status === 'canceled'}
            class:bg-accent={r.status === 'running' || r.status === 'preparing'}
            aria-hidden="true"
          ></span>
          <div class="col-span-5 min-w-0">
            <div class="text-sm font-medium truncate">{r.recipe}</div>
            <div class="text-xs text-emos-text-3 truncate font-mono">{r.id}</div>
          </div>
          <div class="col-span-2 text-xs text-emos-text-3 text-right md:text-left">
            {relTime(r.started_at)}
          </div>
          <div class="col-span-2 text-xs text-emos-text-3 hidden md:block">
            {formatDuration(r.started_at, r.finished_at)}
          </div>
          <div class="col-span-2 text-right">
            <span
              class="pill text-[0.7rem]"
              class:pill-good={r.status === 'finished'}
              class:pill-bad={r.status === 'failed'}
              class:pill-warn={r.status === 'canceled'}
            >
              {r.status}
            </span>
          </div>
        </a>
      {/each}
    </div>
  {/if}
</section>

<style>
  .bg-good { background: var(--color-emos-good); }
  .bg-bad { background: var(--color-emos-bad); }
  .bg-warn { background: var(--color-emos-warn); }
  .bg-accent { background: var(--color-emos-accent); }
</style>
