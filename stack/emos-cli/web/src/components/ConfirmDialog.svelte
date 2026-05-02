<script lang="ts">
  import { onMount } from 'svelte';
  import { AlertTriangle } from 'lucide-svelte';
  import { dialog, _resolveDialog } from '$lib/dialog';

  // Mounted once at the root. Reads $dialog and renders the modal whenever
  // a caller has invoked `confirm(...)` and is awaiting a response.

  function cancel() { _resolveDialog(false); }
  function ok() { _resolveDialog(true); }

  function onKeydown(e: KeyboardEvent) {
    if (!$dialog) return;
    if (e.key === 'Escape') { e.preventDefault(); cancel(); }
    else if (e.key === 'Enter') { e.preventDefault(); ok(); }
  }

  onMount(() => {
    window.addEventListener('keydown', onKeydown);
    return () => window.removeEventListener('keydown', onKeydown);
  });
</script>

{#if $dialog}
  <div
    class="fixed inset-0 z-[60] flex items-center justify-center p-6 bg-black/55 backdrop-blur-sm"
    role="dialog"
    aria-modal="true"
    aria-labelledby="emos-dialog-title"
    onclick={cancel}
    onkeydown={onKeydown}
    tabindex="-1"
  >
    <div
      class="surface w-full max-w-md p-6 animate-rise"
      onclick={(e) => e.stopPropagation()}
      role="presentation"
    >
      <div class="flex items-start gap-3">
        {#if $dialog.intent === 'destructive'}
          <div class="rounded-full p-2 shrink-0 mt-0.5"
               style:background="color-mix(in oklab, var(--color-emos-bad) 15%, transparent)"
               style:color="var(--color-emos-bad)">
            <AlertTriangle size={18} />
          </div>
        {/if}
        <div class="min-w-0 flex-1">
          <h2 id="emos-dialog-title" class="text-base font-semibold tracking-tight">
            {$dialog.title}
          </h2>
          {#if $dialog.message}
            <p class="text-sm text-emos-text-2 mt-2 leading-relaxed">{$dialog.message}</p>
          {/if}
        </div>
      </div>

      <div class="flex items-center justify-end gap-2 mt-6">
        <button class="btn btn-ghost" onclick={cancel}>{$dialog.cancelLabel}</button>
        <button
          class="btn"
          class:btn-danger={$dialog.intent === 'destructive'}
          class:btn-primary={$dialog.intent !== 'destructive'}
          onclick={ok}
        >
          {$dialog.confirmLabel}
        </button>
      </div>
    </div>
  </div>
{/if}
