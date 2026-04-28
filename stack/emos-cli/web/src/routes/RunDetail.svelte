<script lang="ts">
  import { link } from '$lib/router';
  import { ArrowLeft, Square, Loader2 } from 'lucide-svelte';
  import { useRun, useCancelRun } from '$lib/queries';
  import { formatDuration, relTime } from '$lib/time';
  import LogConsole from '$components/LogConsole.svelte';

  let { params }: { params?: { id?: string } } = $props();
  // App.svelte remounts this component when the route changes; safe to
  // read params once at init and call the hook once.
  const id = params?.id ?? '';
  const run = useRun(id);
  const cancel = useCancelRun();
</script>

<section class="space-y-5 h-full flex flex-col">
  <div class="flex items-end justify-between gap-3">
    <div>
      <a use:link href="/runs" class="inline-flex items-center gap-1 text-sm text-emos-text-3 hover:text-emos-text">
        <ArrowLeft size={14} /> Runs
      </a>
      {#if $run.data}
        <h2 class="text-2xl font-semibold tracking-tight mt-1">{$run.data.recipe}</h2>
        <div class="text-xs text-emos-text-3 mt-1">
          started {relTime($run.data.started_at)} ·
          duration {formatDuration($run.data.started_at, $run.data.finished_at)} ·
          rmw {$run.data.rmw}
        </div>
      {/if}
    </div>
    <div class="flex items-center gap-2">
      {#if $run.data?.status === 'preparing'}
        <span class="pill">
          <Loader2 size={12} class="animate-spin" />
          preparing
        </span>
        <button class="btn btn-danger" onclick={() => $cancel.mutate(id)} disabled={$cancel.isPending}>
          {#if $cancel.isPending}<Loader2 size={14} class="animate-spin" />{:else}<Square size={14} />{/if}
          Cancel
        </button>
      {:else if $run.data?.status === 'running'}
        <span class="pill pill-good">
          <span class="w-1.5 h-1.5 rounded-full animate-pulse-dot" style:background="var(--color-emos-good)"></span>
          running
        </span>
        <button class="btn btn-danger" onclick={() => $cancel.mutate(id)} disabled={$cancel.isPending}>
          {#if $cancel.isPending}<Loader2 size={14} class="animate-spin" />{:else}<Square size={14} />{/if}
          Stop
        </button>
      {:else if $run.data}
        <span
          class="pill text-[0.75rem]"
          class:pill-bad={$run.data.status === 'failed'}
          class:pill-warn={$run.data.status === 'canceled'}
          class:pill-good={$run.data.status === 'finished'}
        >
          {$run.data.status} · exit {$run.data.exit_code}
        </span>
      {/if}
    </div>
  </div>

  {#if $run.data?.error}
    <div class="surface p-3 text-sm text-emos-bad">{$run.data.error}</div>
  {/if}

  <div class="flex-1 min-h-0">
    {#if id}
      <LogConsole path={`/runs/${id}/logs`} active={true} />
    {/if}
  </div>
</section>
