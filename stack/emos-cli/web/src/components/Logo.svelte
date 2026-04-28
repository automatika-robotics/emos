<script lang="ts">
  import { theme } from '$lib/theme';
  import emosOnDark from '../assets/emos-on-dark.png';
  import emosOnLight from '../assets/emos-on-light.png';

  // Caller picks display height; width is derived from the wordmark's
  // intrinsic 6489×2000 aspect ratio (≈3.245:1).
  let { height = 24 }: { height?: number } = $props();
  const ASPECT = 6489 / 2000;
  let width = $derived(Math.round(height * ASPECT));

  // The two PNG variants only differ in the colour of the "MOS"
  // letterforms — red "E" is shared, so the swap is purely about
  // contrast against the surface behind it.
  let src = $derived($theme === 'dark' ? emosOnDark : emosOnLight);
</script>

<img
  {src}
  alt="EMOS"
  width={width}
  height={height}
  draggable="false"
  loading="eager"
  decoding="async"
  class="select-none"
  style:height="{height}px"
  style:width="auto"
/>
