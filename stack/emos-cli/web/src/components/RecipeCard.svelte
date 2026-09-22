<script lang="ts">
  import { link } from '$lib/router';
  import { Play, Download, Trash2, Loader2, CheckCircle2, AlertTriangle } from 'lucide-svelte';
  import type { LocalRecipe, RemoteRecipe, Job } from '$lib/api';
  import { renderMarkdown } from '$lib/markdown';

  let {
    recipe,
    state = 'installed',
    job = undefined,
    onRun = undefined,
    onPull = undefined,
    onDelete = undefined,
    busy = false,
  }: {
    recipe: LocalRecipe | RemoteRecipe;
    state?: 'installed' | 'remote' | 'pulling' | 'pulled' | 'failed';
    job?: Job;
    onRun?: () => void;
    onPull?: () => void;
    onDelete?: () => void;
    busy?: boolean;
  } = $props();

  // Distinguish local recipes from catalog (remote) ones via `has_recipe_py`,
  // which the backend always serializes for LocalRecipe (no `omitempty`) and
  // never sends for RemoteRecipe.
  let local = $derived('has_recipe_py' in recipe ? (recipe as LocalRecipe) : null);
  // Both LocalRecipe and RemoteRecipe now use {name: slug, display_name: human}.
  let title = $derived(
    ('display_name' in recipe && recipe.display_name) || recipe.name
  );
  let description = $derived(
    recipe.description || (state === 'remote' ? 'In the catalog — pull to install on this device.' : ''),
  );
  // The version of the recipe: the installed one, or the one this robot gets from the catalog
  let version = $derived('version' in recipe ? recipe.version : undefined);
  let unlicensed = $derived(local ? undefined : (recipe as RemoteRecipe).unlicensed);

  // Tags: the catalog's for a remote recipe, the manifest's for an installed one.
  let tags = $derived.by(() => {
    const t = local ? (local.manifest as any)?.tags : (recipe as RemoteRecipe).tags;
    return Array.isArray(t) ? (t as string[]).slice(0, 3) : [];
  });
</script>

<article
  class="surface p-5 flex flex-col gap-3 transition-transform hover:-translate-y-0.5 animate-rise"
  class:state-pulling={state === 'pulling'}
  class:state-failed={state === 'failed'}
>
  <div class="flex items-start justify-between gap-3">
    <div class="min-w-0 flex-1">
      <h3 class="text-base font-medium truncate">{title}</h3>
      <div class="text-xs text-emos-text-3 truncate font-mono">
        {recipe.name}{#if version}<span class="ml-2 font-sans">· {version}</span>{/if}
      </div>
    </div>
    {#if state === 'installed'}
      <span class="pill pill-good"><CheckCircle2 size={12} /> installed</span>
    {:else if state === 'pulling'}
      <span class="pill"><Loader2 size={12} class="animate-spin" /> pulling</span>
    {:else if state === 'failed'}
      <span class="pill pill-bad"><AlertTriangle size={12} /> failed</span>
    {/if}
  </div>

  {#if description}
    <div class="md-content md-compact line-clamp-3">
      {@html renderMarkdown(description)}
    </div>
  {/if}

  {#if tags.length}
    <div class="flex flex-wrap gap-1.5 mt-1">
      {#each tags as t}<span class="pill text-[0.7rem]">{t}</span>{/each}
    </div>
  {/if}

  {#if unlicensed}
    <div class="text-xs text-emos-text-3">A version made for the {unlicensed} is available with a license.</div>
  {/if}

  {#if state === 'pulling' && job}
    <div class="h-1.5 rounded-full overflow-hidden bg-emos-bg-2 mt-1" aria-label="download progress">
      <div
        class="h-full bg-gradient-to-r from-emos-accent to-emos-accent-2 transition-[width] duration-500"
        style:width="{Math.max(8, Math.round(job.progress * 100))}%"
      ></div>
    </div>
    <div class="text-xs text-emos-text-3">{job.message}</div>
  {/if}

  <div class="flex items-center gap-2 mt-auto">
    {#if state === 'installed' && local}
      <button class="btn btn-primary" onclick={onRun} disabled={busy}>
        <Play size={14} /> Run
      </button>
      <a class="btn btn-ghost" use:link href={'/recipes/' + encodeURIComponent(local.name)}>Details</a>
      {#if onDelete}
        <button class="btn btn-ghost ml-auto" onclick={onDelete} aria-label="delete">
          <Trash2 size={14} />
        </button>
      {/if}
    {:else if state === 'remote'}
      <button class="btn btn-primary" onclick={onPull} disabled={busy}>
        <Download size={14} /> Get
      </button>
    {:else if state === 'pulling'}
      <button class="btn btn-ghost" disabled><Loader2 size={14} class="animate-spin" /> downloading</button>
    {/if}
  </div>
</article>

<style>
  .state-pulling { box-shadow: 0 0 0 1px color-mix(in oklab, var(--color-emos-accent) 35%, transparent); }
  .state-failed  { box-shadow: 0 0 0 1px color-mix(in oklab, var(--color-emos-bad) 35%, transparent); }

  .line-clamp-3 {
    display: -webkit-box;
    -webkit-line-clamp: 3;
    line-clamp: 3;
    -webkit-box-orient: vertical;
    overflow: hidden;
  }
</style>
