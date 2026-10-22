<script lang="ts">
  import { link, navigate } from '$lib/router';
  import { Download, ArrowLeft, Loader2, CloudOff } from 'lucide-svelte';
  import { useRecipesRemote, useRecipesLocal, usePullRecipe, useJobs } from '$lib/queries';
  import { ApiException } from '$lib/api';
  import { renderMarkdown } from '$lib/markdown';
  import { confirm as confirmDialog } from '$lib/dialog';
  import Empty from '$components/Empty.svelte';

  let { params }: { params?: { name?: string } } = $props();
  // svelte-ignore state_referenced_locally
  const name = params?.name ?? '';
  const remote = useRecipesRemote();
  const local = useRecipesLocal();
  const jobs = useJobs();
  const pull = usePullRecipe();

  let recipe = $derived(($remote.data ?? []).find((r) => r.name === name));
  let installed = $derived(($local.data ?? []).some((r) => r.name === name));
  // The pull of this recipe, while it runs or just after it failed
  let job = $derived(
    ($jobs.data ?? []).find(
      (j) => j.kind === 'recipe_pull' && j.target === name &&
        (j.status === 'running' || (j.status === 'failed' && Date.now() - new Date(j.started_at).getTime() < 30000)),
    ),
  );
  let offline = $derived($remote.isError && $remote.error instanceof ApiException && $remote.error.code === 'offline');

  // Once the pull lands, the installed page takes over
  $effect(() => {
    if (installed) navigate('/recipes/' + encodeURIComponent(name));
  });

  async function get() {
    try {
      await $pull.mutateAsync(name);
    } catch (err) {
      await confirmDialog({
        title: 'Could not start pull',
        message: err instanceof ApiException ? err.message : String(err),
        confirmLabel: 'OK',
        cancelLabel: 'Dismiss',
      });
    }
  }
</script>

<section class="space-y-6">
  <a use:link href="/recipes" class="inline-flex items-center gap-1 text-sm text-emos-text-3 hover:text-emos-text">
    <ArrowLeft size={14} /> Recipes
  </a>

  {#if $remote.isLoading}
    <div class="surface h-40 shimmer"></div>
  {:else if offline}
    <Empty icon={CloudOff} title="You're offline" description="Connect this robot to the internet to browse the catalog." />
  {:else if !recipe}
    <Empty title="Not in the catalog" description={`There is no recipe named ${name} that this robot can use.`} />
  {:else}
    <div class="surface p-6">
      <div class="flex items-start gap-4">
        <div class="min-w-0 flex-1">
          <h2 class="text-2xl font-semibold tracking-tight truncate">{recipe.display_name || recipe.name}</h2>
          <div class="text-xs text-emos-text-3 font-mono mt-1">
            {recipe.name}<span class="ml-2 font-sans">· {recipe.version}</span>
          </div>
          {#if recipe.tags?.length}
            <div class="flex flex-wrap gap-1.5 mt-2">
              {#each recipe.tags as t}<span class="pill text-[0.7rem]">{t}</span>{/each}
            </div>
          {/if}
          {#if recipe.description}
            <div class="md-content mt-4 max-w-3xl">
              {@html renderMarkdown(recipe.description)}
            </div>
          {/if}
          {#if recipe.unlicensed}
            <div class="text-xs text-emos-text-3 mt-3">A version made for the {recipe.unlicensed} is available with a license.</div>
          {/if}
        </div>
        <div class="flex items-center gap-2">
          {#if job?.status === 'running'}
            <button class="btn btn-ghost" disabled><Loader2 size={14} class="animate-spin" /> downloading</button>
          {:else}
            <button class="btn btn-primary" disabled={$pull.isPending} onclick={get}>
              <Download size={14} /> Get
            </button>
          {/if}
        </div>
      </div>
      {#if job}
        <div class="mt-3 text-xs" class:text-emos-warn={job.status === 'failed'} class:text-emos-text-3={job.status !== 'failed'}>
          {job.message}
        </div>
      {/if}
      <p class="mt-4 text-xs text-emos-text-3">
        The topics this recipe needs show once it is installed on this device.
      </p>
    </div>
  {/if}
</section>
