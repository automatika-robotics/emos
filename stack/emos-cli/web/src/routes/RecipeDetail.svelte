<script lang="ts">
  import { link, navigate } from '$lib/router';
  import { Play, Trash2, ArrowLeft, Cable, Loader2 } from 'lucide-svelte';
  import { useRecipeDetail, useRuns, useStartRun, useDeleteRecipe, usePluginsInstalled } from '$lib/queries';
  import { pluginName, type ExtractedTopic } from '$lib/api';
  import { renderMarkdown } from '$lib/markdown';
  import { confirm as confirmDialog } from '$lib/dialog';
  import Empty from '$components/Empty.svelte';

  let { params }: { params?: { name?: string } } = $props();
  // App.svelte remounts this component when the route changes, so reading
  // params once at init is correct AND avoids re-creating the query
  // subscription on every $derived re-evaluation.
  // svelte-ignore state_referenced_locally
  const name = params?.name ?? '';
  const detail = useRecipeDetail(name);
  const runs = useRuns();
  const startRun = useStartRun();
  const remove = useDeleteRecipe();
  const plugins = usePluginsInstalled();

  // What a recipe's topics need in place to carry data.
  const topicNeeds = (topics: ExtractedTopic[]) => ({
    robot: topics.some((t) => t.use_plugin && !t.plugin_id),
    named: topics.some((t) => t.use_plugin && t.plugin_id),
    drivers: topics.some((t) => !t.use_plugin && t.is_sensor),
  });

  let activeRun = $derived(
    ($runs.data ?? []).find((r) => r.status === 'running' || r.status === 'preparing')
  );
  let canRun = $derived(!activeRun);

  async function run() {
    const res = await $startRun.mutateAsync({ recipe: name });
    navigate('/runs/' + res.id);
  }

  async function del() {
    const ok = await confirmDialog({
      title: `Remove ${name}?`,
      message: 'The recipe will be deleted from this device. You can pull it again from the catalog later.',
      confirmLabel: 'Remove',
      intent: 'destructive',
    });
    if (!ok) return;
    await $remove.mutateAsync(name);
    navigate('/recipes');
  }
</script>

<section class="space-y-6">
  <a use:link href="/recipes" class="inline-flex items-center gap-1 text-sm text-emos-text-3 hover:text-emos-text">
    <ArrowLeft size={14} /> Recipes
  </a>

  {#if $detail.isLoading}
    <div class="surface h-40 shimmer"></div>
  {:else if $detail.isError}
    <Empty title="Couldn't load recipe" description={String($detail.error)} />
  {:else if $detail.data}
    {@const r = $detail.data}
    <div class="surface p-6">
      <div class="flex items-start gap-4">
        <div class="min-w-0 flex-1">
          <h2 class="text-2xl font-semibold tracking-tight truncate">
            {r.display_name || r.name}
          </h2>
          <div class="text-xs text-emos-text-3 font-mono mt-1">{r.name}</div>
          {#if r.description}
            <div class="md-content mt-4 max-w-3xl">
              {@html renderMarkdown(r.description)}
            </div>
          {/if}
        </div>
        <div class="flex items-center gap-2">
          <button class="btn btn-primary" disabled={!canRun || $startRun.isPending} onclick={run}>
            {#if $startRun.isPending}<Loader2 size={14} class="animate-spin" />{:else}<Play size={14} />{/if}
            Run
          </button>
          <button class="btn btn-ghost" onclick={del} aria-label="remove">
            <Trash2 size={14} />
          </button>
        </div>
      </div>

      {#if !canRun}
        <div class="mt-3 text-xs text-emos-warn">
          Another recipe is already running. Stop it before starting a new one.
        </div>
      {/if}
    </div>

    <!-- Topics -->
    {#if r.topics?.length}
      {@const needs = topicNeeds(r.topics)}
      {@const installed = $plugins.data}
      <div class="surface p-6 space-y-3">
        <div class="flex items-center gap-2 text-sm font-medium">
          <Cable size={16} class="text-emos-accent" />
          Required topics
        </div>
        <div class="grid grid-cols-1 md:grid-cols-2 gap-2">
          {#each r.topics as t}
            <div class="flex items-center justify-between gap-2 px-3 py-2 rounded-lg bg-emos-bg-2 border border-emos-line/60">
              <div class="min-w-0">
                <div class="font-mono text-sm truncate">{t.name}</div>
                <div class="text-xs text-emos-text-3 truncate">{t.msg_type}</div>
              </div>
              {#if t.use_plugin}
                <span class="pill pill-good text-[0.7rem] truncate">
                  {t.plugin_id ? `plugin ${t.plugin_id}` : 'robot plugin'}
                </span>
              {:else if t.is_sensor}
                <span class="pill pill-good text-[0.7rem]">sensor</span>
              {:else}
                <span class="pill text-[0.7rem]">topic</span>
              {/if}
            </div>
          {/each}
        </div>
        <ul class="space-y-1 text-sm text-emos-text-3">
          {#if needs.robot && installed}
            {#if installed.robot}
              <li>Robot plugin topics come from the installed robot plugin, {pluginName(installed.robot)}.</li>
            {:else}
              <li class="text-emos-warn">
                Robot plugin topics need a robot plugin, and none is installed.
                <a use:link href="/plugins" class="underline">Install one</a>.
              </li>
            {/if}
          {/if}
          {#if needs.named && installed}
            {#if installed.sensors.length}
              <li>Topics via a named plugin come from the sensor plugin the recipe attaches with that id.</li>
            {:else}
              <li class="text-emos-warn">
                Topics via a named plugin need a sensor plugin, and none is installed.
                <a use:link href="/plugins" class="underline">Install one</a>.
              </li>
            {/if}
          {/if}
          {#if needs.drivers}
            <li>ROS sensor topics have to be published by a ROS driver for the sensor, or by a node the recipe starts.</li>
          {/if}
        </ul>
      </div>
    {/if}
  {/if}
</section>
