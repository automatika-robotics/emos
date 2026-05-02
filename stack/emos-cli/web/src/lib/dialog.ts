// Imperative confirmation dialog. The dialog itself is a singleton mounted
// at the root of App.svelte; callers invoke `confirm({...})` and await a
// boolean. Cleaner than per-component <ConfirmDialog bind:open> wiring,
// and avoids polluting templates with state for a one-off interaction.
//
// Designed only for "are you sure?" yes/no flows. For richer prompts
// (text input, multi-step), build a dedicated component.

import { writable, type Readable } from 'svelte/store';

export type DialogIntent = 'destructive' | 'normal';

export interface DialogOptions {
  title: string;
  message?: string;
  confirmLabel?: string;
  cancelLabel?: string;
  intent?: DialogIntent;
}

interface DialogState extends Required<Omit<DialogOptions, 'message'>> {
  message: string;
  resolve: (ok: boolean) => void;
}

const _state = writable<DialogState | null>(null);
export const dialog: Readable<DialogState | null> = { subscribe: _state.subscribe };

export function confirm(opts: DialogOptions): Promise<boolean> {
  return new Promise((resolve) => {
    _state.set({
      title: opts.title,
      message: opts.message ?? '',
      confirmLabel: opts.confirmLabel ?? 'Confirm',
      cancelLabel: opts.cancelLabel ?? 'Cancel',
      intent: opts.intent ?? 'normal',
      resolve,
    });
  });
}

/** Internal — used by the dialog component to dispatch a result and clear. */
export function _resolveDialog(ok: boolean): void {
  _state.update((s) => {
    s?.resolve(ok);
    return null;
  });
}
