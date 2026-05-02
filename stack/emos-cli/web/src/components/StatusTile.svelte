<script lang="ts">
  import type { Snippet, Component } from 'svelte';

  let {
    label,
    value,
    sub = undefined,
    icon: Icon = undefined,
    tone = 'neutral',
    children = undefined,
  }: {
    label: string;
    value: string | number;
    sub?: string;
    icon?: Component<any> | undefined;
    tone?: 'neutral' | 'good' | 'warn' | 'bad';
    children?: Snippet;
  } = $props();
</script>

<div class="surface p-5 flex flex-col gap-2 min-w-0 animate-rise">
  <div class="flex items-center gap-2 text-emos-text-3 text-xs uppercase tracking-wider">
    {#if Icon}<Icon size={14} />{/if}
    <span>{label}</span>
  </div>
  <div
    class="text-2xl font-medium tracking-tight truncate"
    class:tone-good={tone === 'good'}
    class:tone-warn={tone === 'warn'}
    class:tone-bad={tone === 'bad'}
  >
    {value}
  </div>
  {#if sub}<div class="text-xs text-emos-text-3 truncate">{sub}</div>{/if}
  {#if children}{@render children()}{/if}
</div>

<style>
  .tone-good { color: var(--color-emos-good); }
  .tone-warn { color: var(--color-emos-warn); }
  .tone-bad  { color: var(--color-emos-bad); }
</style>
