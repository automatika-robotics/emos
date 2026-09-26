<script lang="ts">
  import { BadgeCheck, LifeBuoy, Mail } from 'lucide-svelte';
  import { useLicense } from '$lib/queries';

  const license = useLicense();

  const day = (iso?: string) => (iso ? new Date(iso).toLocaleDateString() : '');
  const tier = (t?: string) => (t ? t[0].toUpperCase() + t.slice(1) : '');

  let rows = $derived(
    [
      ['licensed to', $license.data?.holder],
      ['robot', $license.data?.robot],
      ['tier', tier($license.data?.tier)],
      ['serial number', $license.data?.serial_number],
      ['activated', day($license.data?.activated_at)],
      ['verified', day($license.data?.verified_at)],
    ].filter(([, value]) => value),
  );
</script>

{#if $license.data}
  <div class="surface p-5 space-y-3">
    <div class="flex items-center gap-2 text-xs uppercase tracking-wider text-emos-text-3">
      License
      {#if $license.data.licensed}
        <span class="pill pill-good ml-auto normal-case tracking-normal"><BadgeCheck size={12} /> licensed</span>
      {/if}
    </div>

    {#if $license.data.licensed}
      <div class="grid grid-cols-[auto_1fr] gap-x-6 gap-y-1 text-sm">
        {#each rows as [label, value]}
          <div class="text-emos-text-3">{label}</div><div>{value}</div>
        {/each}
      </div>
      <a class="btn btn-ghost" href={$license.data.support_url} target="_blank" rel="noreferrer">
        <LifeBuoy size={14} /> Get support
      </a>
    {:else}
      <p class="text-sm text-emos-text-3 leading-relaxed">
        This install has no license. EMOS is free to use.
        A license brings professional support and recipes customised for your robots.
      </p>
      <a class="btn btn-ghost" href={`mailto:${$license.data.sales_email}`}>
        <Mail size={14} /> {$license.data.sales_email}
      </a>
    {/if}
  </div>
{/if}
