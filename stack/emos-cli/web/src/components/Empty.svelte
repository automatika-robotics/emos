<script lang="ts">
  import type { Snippet, Component } from 'svelte';
  let {
    title,
    description = '',
    icon: Icon = undefined,
    actions = undefined,
  }: {
    title: string;
    description?: string;
    icon?: Component<any> | undefined;
    actions?: Snippet;
  } = $props();
</script>

<div class="surface px-8 py-12 flex flex-col items-center text-center gap-3 animate-rise">
  {#if Icon}
    <div class="rounded-full p-3 bg-emos-surface-2 text-emos-text-2">
      <Icon size={20} />
    </div>
  {/if}
  <h3 class="text-base font-medium">{title}</h3>
  {#if description}<p class="text-sm text-emos-text-3 max-w-md">{description}</p>{/if}
  {#if actions}<div class="mt-2 flex gap-2">{@render actions()}</div>{/if}
</div>
