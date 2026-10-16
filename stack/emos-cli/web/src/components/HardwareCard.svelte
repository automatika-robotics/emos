<script lang="ts">
  import type { Snippet, Component, SvelteComponent } from 'svelte';
  import { renderMarkdown } from '$lib/markdown';

  // A piece of hardware on the System page: the robot, or a sensor plugin.
  let {
    title,
    subtitle = '',
    description = '',
    image = '',
    icon: Icon,
    large = false,
    footer,
    children,
  }: {
    title: string;
    subtitle?: string;
    description?: string;
    image?: string;
    // lucide-svelte icons are declared as (legacy) class components.
    icon: Component<any> | typeof SvelteComponent<any>;
    large?: boolean; // the robot's card; sensors use the smaller layout
    footer: string;
    children: Snippet; // the feeds, actions and events
  } = $props();

  // Pictures come from the support portal, which a robot on an offline
  // network cannot reach; a failed one falls back to the icon.
  let failedImage = $state('');
</script>

<div class="surface p-5 flex flex-col md:flex-row {large ? 'gap-6' : 'gap-5'}">
  <div class="shrink-0 {large ? 'md:w-56' : 'md:w-40'} flex items-start justify-center">
    {#if image && image !== failedImage}
      <img
        src={image}
        alt={title}
        class="{large ? 'max-h-48' : 'max-h-36'} w-full object-contain"
        loading="lazy"
        onerror={() => (failedImage = image)}
      />
    {:else}
      <div
        class="w-full {large ? 'h-36' : 'h-28'} rounded-xl bg-emos-surface-2 text-emos-accent flex items-center justify-center"
      >
        <Icon size={large ? 40 : 32} />
      </div>
    {/if}
  </div>
  <div class="min-w-0 flex-1 space-y-3">
    <div>
      <div class={large ? 'text-lg font-semibold' : 'font-semibold'}>{title}</div>
      {#if subtitle}<div class="text-sm text-emos-text-3">{subtitle}</div>{/if}
    </div>
    {#if description}
      <div class="md-content md-compact">{@html renderMarkdown(description)}</div>
    {/if}
    {@render children()}
    <div class="text-xs text-emos-text-3 pt-1">{footer}</div>
  </div>
</div>
