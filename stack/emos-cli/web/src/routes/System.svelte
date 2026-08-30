<script lang="ts">
  import { Wifi, WifiOff, RefreshCw, KeyRound, ArrowUpCircle, Bot, Radar } from 'lucide-svelte';
  import { useInfo, useCapabilities, useConnectivity, useRobot, usePluginsInstalled } from '$lib/queries';
  import type { InstalledPlugin } from '$lib/api';
  import { renderMarkdown } from '$lib/markdown';
  import { api } from '$lib/api';
  import { clearToken } from '$lib/auth';
  import { navigate } from '$lib/router';
  import { confirm as confirmDialog } from '$lib/dialog';
  import { onMount } from 'svelte';

  const info = useInfo();
  const caps = useCapabilities();
  const conn = useConnectivity();
  const robot = useRobot();
  const plugins = usePluginsInstalled();

  // Everything the System page shows for an installed plugin, robot or sensor:
  // what its describe() reports plus how it was installed.
  const names = (items: unknown, field: string): string[] =>
    ((items ?? []) as any[]).map((x) => x?.[field]).filter(Boolean);
  const view = (p: InstalledPlugin) => {
    const d = (p.describe as any) ?? {};
    return {
      slug: p.slug,
      name: (d.metadata?.name as string) ?? p.slug,
      vendor: d.metadata?.vendor as string | undefined,
      version: d.metadata?.version as string | undefined,
      description: d.metadata?.description as string | undefined,
      entry: p.entry_point,
      repo: p.repo,
      ref: p.ref,
      installed: p.installed_at ? new Date(p.installed_at).toLocaleDateString() : '',
      sources: p.sources ?? [],
      image: p.image_url,
      feeds: names(d.feedbacks, 'key'),
      actions: names(d.actions, 'name'),
      events: names(d.events, 'name'),
    };
  };
  let robotPlugin = $derived($plugins.data?.robot ? view($plugins.data.robot) : null);
  let sensorPlugins = $derived(($plugins.data?.sensors ?? []).map(view));

  // The robot's feeds, split into what it senses and the rest (status, battery, ...).
  let robotSensors = $derived(new Set($robot.data?.sensors ?? []));
  let robotOtherFeeds = $derived((robotPlugin?.feeds ?? []).filter((f) => !robotSensors.has(f)));

  let dashUrl = $state<string>('');
  onMount(() => {
    dashUrl = window.location.origin || '';
  });

  async function unpair() {
    const ok = await confirmDialog({
      title: 'Sign this browser out?',
      message: 'You\'ll need the device pairing code to sign back in.',
      confirmLabel: 'Sign out',
      intent: 'destructive',
    });
    if (!ok) return;
    clearToken();
    navigate('/pair');
  }
</script>
{#snippet pills(label: string, items: string[], tone: string = '')}
  {#if items.length}
    <div class="text-sm">
      <span class="text-emos-text-3">{label}: </span>
      {#each items as it (it)}<span class="pill mr-1 {tone}">{it}</span>{/each}
    </div>
  {/if}
{/snippet}

<section class="space-y-6">
  <div>
    <div class="text-xs uppercase tracking-wider text-emos-text-3">Device</div>
    <h2 class="text-2xl font-semibold tracking-tight">System</h2>
  </div>

  <!-- The robot -->
  <div class="text-xs uppercase tracking-wider text-emos-text-3 flex items-center gap-1"><Bot size={12} /> Robot</div>
  {#if $robot.data}
    <div class="surface p-5 flex flex-col md:flex-row gap-6">
      <div class="shrink-0 md:w-56 flex items-start justify-center">
        {#if $robot.data.image_url}
          <img
            src={$robot.data.image_url}
            alt={$robot.data.model ?? $robot.data.name ?? 'robot'}
            class="max-h-48 w-full object-contain"
            loading="lazy"
          />
        {:else}
          <div class="w-full h-36 rounded-xl bg-emos-surface-2 text-emos-accent flex items-center justify-center">
            <Bot size={40} />
          </div>
        {/if}
      </div>
      <div class="min-w-0 flex-1 space-y-3">
        <div>
          <div class="text-lg font-semibold">{$robot.data.model ?? $robot.data.name ?? 'Robot'}</div>
          <div class="text-sm text-emos-text-3">
            {$robot.data.vendor ?? ''}{$robot.data.serial ? ` · ${$robot.data.serial}` : ''}{$robot.data.kinematics ? ` · ${$robot.data.kinematics}` : ''}
          </div>
        </div>
        {#if robotPlugin?.description}
          <div class="md-content md-compact">{@html renderMarkdown(robotPlugin.description)}</div>
        {/if}
        {#if robotPlugin}
          {@render pills('sensors', $robot.data.sensors ?? [], 'pill-good')}
          {@render pills('other feeds', robotOtherFeeds)}
          {@render pills('actions', robotPlugin.actions)}
          {@render pills('events', robotPlugin.events)}
        {:else}
          {@render pills('sensors', $robot.data.sensors ?? [], 'pill-good')}
          {@render pills('actions', $robot.data.actions ?? [])}
          {@render pills('events', $robot.data.events ?? [])}
        {/if}
        <div class="text-xs text-emos-text-3 pt-1">
          {robotPlugin ? 'Described by the installed robot plugin.' : `Described by the robot ${$robot.data.source}.`}
        </div>
      </div>
    </div>
  {:else}
    <div class="surface p-5 text-sm text-emos-text-3">
      No robot identity is exposed by this device. Generic dashboard.
      Install a robot plugin, or a licensed deployment will populate this automatically.
    </div>
  {/if}

  <!-- Its sensors -->
  {#if sensorPlugins.length}
    <div class="text-xs uppercase tracking-wider text-emos-text-3 flex items-center gap-1"><Radar size={12} /> Sensors</div>
    <div class="grid grid-cols-1 xl:grid-cols-2 gap-4">
      {#each sensorPlugins as s (s.slug)}
        <div class="surface p-5 flex flex-col md:flex-row gap-5">
          <div class="shrink-0 md:w-40 flex items-start justify-center">
            {#if s.image}
              <img src={s.image} alt={s.name} class="max-h-36 w-full object-contain" loading="lazy" />
            {:else}
              <div class="w-full h-28 rounded-xl bg-emos-surface-2 text-emos-accent flex items-center justify-center">
                <Radar size={32} />
              </div>
            {/if}
          </div>
          <div class="min-w-0 flex-1 space-y-3">
            <div>
              <div class="font-semibold">{s.name}</div>
              {#if s.vendor}<div class="text-sm text-emos-text-3">{s.vendor}</div>{/if}
            </div>
            {#if s.description}
              <div class="md-content md-compact">{@html renderMarkdown(s.description)}</div>
            {/if}
            {@render pills('feeds', s.feeds, 'pill-good')}
            {@render pills('actions', s.actions)}
            {@render pills('events', s.events)}
            <div class="text-xs text-emos-text-3 pt-1">Described by the installed sensor plugin.</div>
          </div>
        </div>
      {/each}
    </div>
  {/if}

  <!-- The device it runs on -->
  <div class="text-xs uppercase tracking-wider text-emos-text-3">Device</div>
  <div class="grid grid-cols-1 md:grid-cols-3 gap-4">
    <div class="surface p-5 space-y-2">
      <div class="text-xs uppercase tracking-wider text-emos-text-3">EMOS</div>
      <div class="grid grid-cols-[auto_1fr] gap-x-6 gap-y-1 text-sm">
        <div class="text-emos-text-3">version</div>
        <div class="font-mono flex items-center gap-2">
          <span>{$info.data?.version ?? '—'}</span>
          {#if $info.data?.update_available && $info.data?.latest_version}
            <a
              href={`https://github.com/automatika-robotics/emos/releases/tag/v${$info.data.latest_version}`}
              target="_blank"
              rel="noopener"
              class="pill pill-good text-[0.7rem]"
              title="A newer release of EMOS is available"
            >
              <ArrowUpCircle size={12} /> v{$info.data.latest_version}
            </a>
          {/if}
        </div>
        <div class="text-emos-text-3">uptime</div><div class="font-mono">{$info.data?.uptime ?? '—'}</div>
        <div class="text-emos-text-3">install</div><div>{$info.data?.mode ?? '—'}</div>
        <div class="text-emos-text-3">ros</div><div>{$info.data?.ros_distro ?? '—'}</div>
        <div class="text-emos-text-3">platform</div><div class="font-mono">{$info.data?.platform ?? '—'}</div>
        <div class="text-emos-text-3">hostname</div><div class="font-mono">{$info.data?.hostname ?? '—'}</div>
        <div class="text-emos-text-3">recipes dir</div><div class="font-mono truncate" title={$info.data?.recipes_dir ?? ''}>{$info.data?.recipes_dir ?? '—'}</div>
        <div class="text-emos-text-3">logs dir</div><div class="font-mono truncate" title={$info.data?.logs_dir ?? ''}>{$info.data?.logs_dir ?? '—'}</div>
      </div>
    </div>

    <div class="surface p-5 space-y-2">
      <div class="text-xs uppercase tracking-wider text-emos-text-3">Capabilities</div>
      <div class="grid grid-cols-[auto_1fr] gap-x-6 gap-y-1 text-sm">
        <div class="text-emos-text-3">docker</div><div>{$caps.data?.docker_available ? 'yes' : 'no'}</div>
        <div class="text-emos-text-3">pixi</div><div>{$caps.data?.pixi_available ? 'yes' : 'no'}</div>
        <div class="text-emos-text-3">pull recipes</div><div>{$caps.data?.can_pull_recipes ? 'yes' : 'no'}</div>
        <div class="text-emos-text-3">run recipes</div><div>{$caps.data?.can_run_recipes ? 'yes' : 'no'}</div>
        <div class="text-emos-text-3">robot identity</div><div>{$caps.data?.has_robot_identity ? 'yes' : 'no'}</div>
      </div>
    </div>

    <div class="surface p-5 space-y-2">
      <div class="flex items-center gap-2 text-xs uppercase tracking-wider text-emos-text-3">
        Connectivity
        <button
          class="ml-auto text-emos-text-3 hover:text-emos-text"
          onclick={async () => { await api.connectivity(true); $conn.refetch(); }}
          aria-label="re-probe"
        >
          <RefreshCw size={12} />
        </button>
      </div>
      <div class="flex items-center gap-2 text-sm">
        {#if $conn.data?.online}
          <span class="pill pill-good"><Wifi size={12} /> online</span>
        {:else}
          <span class="pill pill-warn"><WifiOff size={12} /> offline</span>
        {/if}
        <span class="text-emos-text-3 truncate font-mono">{$conn.data?.target ?? '—'}</span>
      </div>
      <p class="text-xs text-emos-text-3 leading-relaxed mt-1">
        Dashboard, run, logs, and local recipes always work offline.
        Browsing the catalog and pulling new recipes need internet.
      </p>
    </div>
  </div>

  <div class="surface p-5 space-y-3">
    <div class="text-xs uppercase tracking-wider text-emos-text-3">This browser</div>
    <p class="text-sm text-emos-text-3">
      Open the dashboard from another device on the same network using:
    </p>
    <code class="kbd text-sm">{dashUrl}</code>
    <div class="pt-2">
      <button class="btn btn-ghost" onclick={unpair}>
        <KeyRound size={14} /> Sign out / re-pair
      </button>
    </div>
  </div>
</section>
