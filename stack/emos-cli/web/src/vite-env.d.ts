/// <reference types="svelte" />
/// <reference types="vite/client" />

// Vite emits hashed asset URLs for these binary imports; the value is a string.
declare module '*.png' {
  const src: string;
  export default src;
}
declare module '*.jpg' {
  const src: string;
  export default src;
}
declare module '*.svg' {
  const src: string;
  export default src;
}
