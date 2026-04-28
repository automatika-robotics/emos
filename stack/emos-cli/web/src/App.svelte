<script lang="ts">
  import { QueryClient, QueryClientProvider } from '@tanstack/svelte-query';
  import { onMount } from 'svelte';
  import { api, ApiException } from '$lib/api';
  import { getToken } from '$lib/auth';
  import { navigate, path, useRoutes } from '$lib/router';
  import Sidebar from '$components/Sidebar.svelte';
  import Header from '$components/Header.svelte';
  import CommandPalette from '$components/CommandPalette.svelte';
  import ConfirmDialog from '$components/ConfirmDialog.svelte';
  import Dashboard from '$routes/Dashboard.svelte';
  import Recipes from '$routes/Recipes.svelte';
  import RecipeDetail from '$routes/RecipeDetail.svelte';
  import Runs from '$routes/Runs.svelte';
  import RunDetail from '$routes/RunDetail.svelte';
  import System from '$routes/System.svelte';
  import Pair from '$routes/Pair.svelte';

  const routes = {
    '/': Dashboard,
    '/recipes': Recipes,
    '/recipes/:name': RecipeDetail,
    '/runs': Runs,
    '/runs/:id': RunDetail,
    '/system': System,
    '/pair': Pair,
    '*': Dashboard,
  };
  const active = useRoutes(routes);

  const queryClient = new QueryClient({
    defaultOptions: {
      queries: {
        // Dashboards run for hours; refetch on focus would thrash. Background
        // intervals are configured per-query.
        refetchOnWindowFocus: false,
        retry: 1,
        staleTime: 1000,
      },
    },
  });

  let booted = $state(false);

  // On mount, decide whether to send the user to /pair.
  // Calls /info first (which is public) so we always have a banner ready
  // even before pairing.
  onMount(async () => {
    try {
      await api.info();
    } catch {
      // Daemon down. We'll show the standard surface anyway and let queries
      // surface the error inline.
    }
    if (!getToken()) {
      navigate('/pair');
      booted = true;
      return;
    }
    try {
      await api.authMe();
    } catch (err) {
      if (err instanceof ApiException && err.status === 401) {
        navigate('/pair');
      }
    }
    booted = true;
  });

  // Hide the sidebar on the pair screen — it's the one place users haven't
  // earned access yet.
  let showShell = $derived(!$path.startsWith('/pair'));
  let CurrentPage = $derived($active.component);
  let pageParams = $derived($active.params);

  // Force a fresh mount of the route component whenever the matched path
  // changes. Without this, navigating /recipes/foo → /recipes/bar would
  // re-run the component's setup (and its createQuery hooks) without
  // tearing down the previous query subscription, leaking it.
  let mountKey = $derived($path.split('?')[0]);
</script>

<QueryClientProvider client={queryClient}>
  {#if booted}
    {#if showShell}
      <div class="min-h-screen flex">
        <Sidebar />
        <main class="flex-1 min-w-0 flex flex-col">
          <Header />
          <div class="flex-1 px-8 py-6 max-w-7xl w-full mx-auto">
            {#key mountKey}
              <CurrentPage params={pageParams} />
            {/key}
          </div>
        </main>
        <CommandPalette />
      </div>
    {:else}
      {#key mountKey}
        <CurrentPage params={pageParams} />
      {/key}
    {/if}
    <ConfirmDialog />
  {/if}
</QueryClientProvider>
