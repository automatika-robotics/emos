<script lang="ts">
  import { Wifi, WifiOff, RefreshCw, KeyRound, Loader2 } from 'lucide-svelte';
  import { useInfo, useCapabilities, useConnectivity, useRobot } from '$lib/queries';
  import { api } from '$lib/api';
  import { clearToken } from '$lib/auth';
  import { navigate } from '$lib/router';
  import { onMount } from 'svelte';

  const info = useInfo();
  const caps = useCapabilities();
  const conn = useConnectivity();
  const robot = useRobot();

  let qrSvg = $state<string>('');
  // Tiny QR generator: use the kazuhikoarase implementation? Easier — do it
  // server-side via `emos serve --qr` would require a network call. We'll
  // show the URL and rely on the daemon's terminal QR for first-touch.
  let dashUrl = $state<string>('');
  onMount(() => {
    dashUrl = window.location.origin || '';
  });

  function unpair() {
    if (!confirm('Sign this browser out of the dashboard?')) return;
    clearToken();
    navigate('/pair');
  }
</script>

<section class="space-y-6">
  <div>
    <div class="text-xs uppercase tracking-wider text-emos-text-3">Device</div>
    <h2 class="text-2xl font-semibold tracking-tight">System</h2>
  </div>

  <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
    <div class="surface p-5 space-y-2">
      <div class="text-xs uppercase tracking-wider text-emos-text-3">EMOS</div>
      <div class="grid grid-cols-2 gap-y-1 text-sm">
        <div class="text-emos-text-3">version</div><div class="font-mono">{$info.data?.version ?? '—'}</div>
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
      <div class="text-xs uppercase tracking-wider text-emos-text-3">Robot</div>
      {#if $robot.data}
        <div class="grid grid-cols-2 gap-y-1 text-sm">
          {#if $robot.data.name}<div class="text-emos-text-3">name</div><div>{$robot.data.name}</div>{/if}
          {#if $robot.data.model}<div class="text-emos-text-3">model</div><div>{$robot.data.model}</div>{/if}
          {#if $robot.data.serial}<div class="text-emos-text-3">serial</div><div class="font-mono">{$robot.data.serial}</div>{/if}
          {#if $robot.data.kinematics}<div class="text-emos-text-3">kinematics</div><div>{$robot.data.kinematics}</div>{/if}
          {#if $robot.data.plugin}<div class="text-emos-text-3">plugin</div><div class="font-mono">{$robot.data.plugin}</div>{/if}
          <div class="text-emos-text-3">source</div><div>{$robot.data.source}</div>
        </div>
      {:else}
        <p class="text-sm text-emos-text-3">
          No robot identity is exposed by this device. Generic dashboard.
          Licensed deployments will populate this card automatically.
        </p>
      {/if}
    </div>

    <div class="surface p-5 space-y-2">
      <div class="text-xs uppercase tracking-wider text-emos-text-3">Capabilities</div>
      <div class="grid grid-cols-2 gap-y-1 text-sm">
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
