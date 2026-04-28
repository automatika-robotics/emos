<script lang="ts">
  import { onMount } from 'svelte';
  import { get } from 'svelte/store';
  import { navigate, query } from '$lib/router';
  import { ShieldCheck, Loader2 } from 'lucide-svelte';
  import { api, ApiException } from '$lib/api';
  import { setToken } from '$lib/auth';
  import Logo from '$components/Logo.svelte';

  let digits = $state(['', '', '', '', '', '']);
  let busy = $state(false);
  let error = $state<string | null>(null);
  let inputs: HTMLInputElement[] = $state([]);

  // QR-flow auto-pair: if the URL is /#/pair?code=NNNNNN, pre-fill and submit.
  // Triggered on mount (not on every keystroke) so a stale ?code= in
  // history doesn't replay after the user manually edits a digit.
  onMount(() => {
    const q = get(query);
    const code = q.code;
    if (code && /^\d{6}$/.test(code)) {
      digits = code.split('');
      // micro-tick so the input fields render with values before submit
      queueMicrotask(submit);
    }
  });

  function focusAt(i: number) {
    queueMicrotask(() => inputs[i]?.focus());
  }

  function onInput(i: number, e: Event) {
    const t = e.target as HTMLInputElement;
    const v = t.value.replace(/\D/g, '').slice(-1);
    digits[i] = v;
    if (v && i < 5) focusAt(i + 1);
    if (digits.every((d) => d.length === 1)) submit();
  }

  function onPaste(e: ClipboardEvent) {
    const text = (e.clipboardData?.getData('text') ?? '').replace(/\D/g, '');
    if (text.length >= 6) {
      e.preventDefault();
      digits = text.slice(0, 6).split('');
      submit();
    }
  }

  function onKey(i: number, e: KeyboardEvent) {
    if (e.key === 'Backspace' && !digits[i] && i > 0) focusAt(i - 1);
    if (e.key === 'ArrowLeft' && i > 0) focusAt(i - 1);
    if (e.key === 'ArrowRight' && i < 5) focusAt(i + 1);
  }

  async function submit() {
    if (busy) return;
    const code = digits.join('');
    if (code.length !== 6) return;
    busy = true;
    error = null;
    try {
      const res = await api.authPair(code, 'web');
      setToken(res.token);
      navigate('/');
    } catch (err) {
      digits = ['', '', '', '', '', ''];
      focusAt(0);
      if (err instanceof ApiException) error = err.message;
      else error = String(err);
    } finally {
      busy = false;
    }
  }
</script>

<div class="min-h-screen flex items-center justify-center px-6 py-12">
  <div class="surface w-full max-w-md p-8 animate-rise">
    <div class="flex items-center gap-3 mb-6">
      <Logo size={36} />
      <div>
        <div class="text-base font-semibold tracking-tight">Pair this browser</div>
        <div class="text-xs text-emos-text-3">One-time setup for this device</div>
      </div>
    </div>

    <p class="text-sm text-emos-text-2 mb-6 leading-relaxed">
      Enter the six-digit pairing code shown when EMOS started.
      It was printed in the terminal and saved to
      <code class="kbd">~/.config/emos/pairing.txt</code>.
    </p>

    <div class="flex justify-between gap-2 mb-4" onpaste={onPaste}>
      {#each digits as d, i}
        <input
          bind:this={inputs[i]}
          inputmode="numeric"
          maxlength="1"
          autocomplete="one-time-code"
          value={d}
          oninput={(e) => onInput(i, e)}
          onkeydown={(e) => onKey(i, e)}
          class="w-12 h-14 text-center text-2xl font-mono tracking-tight rounded-xl bg-emos-bg-2 border border-emos-line focus:outline-2 focus:outline-emos-accent transition"
          aria-label={`Digit ${i + 1}`}
        />
      {/each}
    </div>

    {#if error}
      <div class="text-xs text-emos-bad mt-2 mb-2">{error}</div>
    {/if}

    <button
      class="btn btn-primary w-full"
      onclick={submit}
      disabled={busy || digits.some((d) => !d)}
    >
      {#if busy}<Loader2 size={16} class="animate-spin" />{:else}<ShieldCheck size={16} />{/if}
      {busy ? 'Verifying…' : 'Pair'}
    </button>

    <p class="text-xs text-emos-text-3 mt-6 leading-relaxed">
      Can't find the code? Run <code class="kbd">emos config rotate-pairing</code> on the robot to issue a fresh one.
    </p>
  </div>
</div>
