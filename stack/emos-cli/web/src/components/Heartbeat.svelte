<script lang="ts">
  import { useInfo } from '$lib/queries';
  const info = useInfo();
  // Healthy = info query succeeded recently. Failure = red dot.
  let ok = $derived(!$info.isError && !!$info.data);
</script>

<span class="relative inline-flex h-2.5 w-2.5">
  <span
    class="absolute inset-0 rounded-full"
    class:bg-emos-good={ok}
    class:bg-emos-bad={!ok}
    class:animate-pulse-dot={ok}
  ></span>
  <span
    class="absolute inset-0 rounded-full opacity-40 blur-[3px]"
    class:bg-emos-good={ok}
    class:bg-emos-bad={!ok}
    class:animate-pulse-dot={ok}
  ></span>
</span>

<style>
  .bg-emos-good { background: var(--color-emos-good); }
  .bg-emos-bad  { background: var(--color-emos-bad); }
</style>
