<script lang="ts">
  import { ArrowDownToLine, Search } from 'lucide-svelte';
  import { openSSE } from '$lib/sse';

  // ANSI to HTML — small, dependency-free.
  // Handles the SGR subset that ROS / Python tooling actually emits.
  const ANSI_RE = /\x1b\[([0-9;]*)m/g;
  const PALETTE: Record<string, string> = {
    '30': 'color: #6b7280', '31': 'color: #f87171', '32': 'color: #4ade80',
    '33': 'color: #facc15', '34': 'color: #60a5fa', '35': 'color: #c084fc',
    '36': 'color: #22d3ee', '37': 'color: #e5e7eb', '90': 'color: #6b7280',
    '91': 'color: #fb7185', '92': 'color: #86efac', '93': 'color: #fde047',
    '94': 'color: #93c5fd', '95': 'color: #d8b4fe', '96': 'color: #67e8f9',
    '1': 'font-weight: bold',
  };
  function ansiToHtml(s: string): string {
    let out = '', open = 0;
    let last = 0;
    for (const m of s.matchAll(ANSI_RE)) {
      out += escape(s.slice(last, m.index));
      const codes = m[1].split(';').filter(Boolean);
      if (codes.length === 0 || codes.includes('0')) {
        out += '</span>'.repeat(open); open = 0;
      } else {
        const styles = codes.map((c) => PALETTE[c] ?? '').filter(Boolean).join(';');
        if (styles) { out += `<span style="${styles}">`; open++; }
      }
      last = m.index + m[0].length;
    }
    out += escape(s.slice(last)) + '</span>'.repeat(open);
    return out;
  }
  function escape(s: string): string {
    return s.replace(/[&<>]/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;' }[c]!));
  }

  let { path, active = true }: { path: string; active?: boolean } = $props();

  let lines = $state<string[]>([]);
  let level = $state<'all' | 'info' | 'warn' | 'error'>('all');
  let q = $state('');
  let follow = $state(true);
  let connected = $state(false);
  let scroller: HTMLDivElement | undefined;

  function levelOf(line: string): 'info' | 'warn' | 'error' {
    const u = line.toUpperCase();
    if (u.includes('ERROR') || u.includes('CRITICAL') || u.includes('FATAL')) return 'error';
    if (u.includes('WARN')) return 'warn';
    return 'info';
  }
  let visible = $derived.by(() => {
    let out = lines;
    if (level !== 'all') out = out.filter((l) => levelOf(l) === level);
    if (q.trim()) {
      let rx: RegExp;
      try { rx = new RegExp(q.trim(), 'i'); } catch { rx = new RegExp(q.trim().replace(/[.*+?^${}()|[\]\\]/g, '\\$&'), 'i'); }
      out = out.filter((l) => rx.test(l));
    }
    return out.slice(-2000);
  });

  $effect(() => {
    if (follow && scroller) {
      // After visible recomputes, pin to bottom.
      queueMicrotask(() => { if (scroller) scroller.scrollTop = scroller.scrollHeight; });
    }
    // intentional dep: visible
    void visible;
  });

  function onScroll() {
    if (!scroller) return;
    const atBottom = scroller.scrollHeight - scroller.scrollTop - scroller.clientHeight < 24;
    follow = atBottom;
  }

  // Re-establish the SSE connection whenever `path` or `active` changes.
  // Previous version wired this in onMount and silently ignored prop changes.
  $effect(() => {
    if (!path || !active) return;
    // Reset visible buffer when switching streams so the user doesn't see
    // log lines from a previous run jumbled with the new one.
    lines = [];
    const dispose = openSSE(path, {
      onOpen: () => { connected = true; },
      onError: () => { connected = false; },
      onLog: (line) => { lines = [...lines, line]; },
      onEnd: () => { connected = false; },
    });
    return dispose;
  });
</script>

<div class="surface flex flex-col h-full min-h-[24rem]">
  <div class="flex items-center gap-2 px-4 py-2.5 border-b border-emos-line/60">
    <span class="pill" class:pill-good={connected} class:pill-bad={!connected}>
      <span class="w-1.5 h-1.5 rounded-full" style:background={connected ? 'var(--color-emos-good)' : 'var(--color-emos-bad)'}></span>
      {connected ? 'streaming' : 'closed'}
    </span>
    <div class="flex items-center gap-1.5 ml-2">
      {#each ['all', 'info', 'warn', 'error'] as l}
        <button
          class="text-xs px-2 py-1 rounded-md transition"
          class:active={level === l}
          onclick={() => (level = l as any)}
        >{l}</button>
      {/each}
    </div>
    <div class="ml-auto flex items-center gap-2">
      <div class="relative">
        <Search size={12} class="absolute left-2 top-1/2 -translate-y-1/2 text-emos-text-3" />
        <input
          bind:value={q}
          placeholder="filter"
          class="text-xs bg-emos-bg-2 border border-emos-line rounded-md pl-7 pr-2 py-1 w-40"
        />
      </div>
      <button class="btn btn-ghost text-xs px-2 py-1" onclick={() => { follow = true; if (scroller) scroller.scrollTop = scroller.scrollHeight; }}>
        <ArrowDownToLine size={12} /> tail
      </button>
    </div>
  </div>
  <div
    bind:this={scroller}
    onscroll={onScroll}
    class="flex-1 overflow-y-auto px-4 py-3 font-mono text-[0.78rem] leading-[1.55rem]"
  >
    {#if visible.length === 0}
      <div class="text-emos-text-3 italic">No log lines yet…</div>
    {/if}
    {#each visible as line, i (i)}
      <div class="whitespace-pre-wrap break-all" data-level={levelOf(line)}>
        {@html ansiToHtml(line)}
      </div>
    {/each}
  </div>
</div>

<style>
  button.active {
    background: color-mix(in oklab, var(--color-emos-surface-2) 90%, transparent);
    color: var(--color-emos-text);
  }
  div[data-level='warn'] { color: var(--color-emos-warn); }
  div[data-level='error'] { color: var(--color-emos-bad); }
</style>
