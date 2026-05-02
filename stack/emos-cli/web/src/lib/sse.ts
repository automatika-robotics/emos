// EventSource wrapper that gates SSE on a single-use ticket rather than a
// long-lived bearer token in `?token=`. Tokens in URLs leak through
// referer headers, browser history, and reverse-proxy access logs; the
// ticket is short-lived and consumed on first use, so even if it shows
// up in a log it grants nothing.
//
// Reconnect lifecycle: native EventSource auto-reconnects on the same
// URL, which would 401 because the ticket is single-use. We intercept
// the error event, close the dead connection, mint a fresh ticket, and
// open a new EventSource against the same path.

import { api, ApiException, API_BASE } from './api';

export type SSEHandler = {
  onEvent?: (name: string, data: string) => void;
  onLog?: (line: string) => void;
  onStatus?: (data: unknown) => void;
  onEnd?: (data: unknown) => void;
  onOpen?: () => void;
  onError?: (err: Event) => void;
};

const RECONNECT_BACKOFF_MS = [500, 1000, 2000, 5000];

/**
 * Open an SSE stream against `path` (e.g. `/runs/{id}/logs`). Returns a
 * cancel function that closes the underlying EventSource and stops any
 * in-flight reconnect attempts.
 */
export function openSSE(path: string, handler: SSEHandler): () => void {
  let es: EventSource | null = null;
  let cancelled = false;
  let endedNormally = false;
  let retry = 0;

  async function connect(): Promise<void> {
    if (cancelled) return;
    let ticket: string;
    try {
      const resp = await api.authSSETicket();
      ticket = resp.ticket;
    } catch (err) {
      // No ticket → no stream. Surface to the handler as an error event;
      // ApiException carries status/code if the caller wants to inspect.
      if (cancelled) return;
      handler.onError?.(new CustomEvent('sse-ticket-error', { detail: err }) as unknown as Event);
      // Pairing has likely been revoked. Don't loop forever.
      if (err instanceof ApiException && err.status === 401) return;
      scheduleReconnect();
      return;
    }
    if (cancelled) return;

    const sep = path.includes('?') ? '&' : '?';
    const url = `${API_BASE}${path}${sep}ticket=${encodeURIComponent(ticket)}`;
    es = new EventSource(url);

    es.onopen = () => {
      retry = 0;
      handler.onOpen?.();
    };
    es.onerror = (e) => {
      handler.onError?.(e);
      if (cancelled || endedNormally) return;
      // EventSource will keep auto-reconnecting against the now-stale
      // URL; close it and mint a new ticket ourselves.
      es?.close();
      es = null;
      scheduleReconnect();
    };

    es.onmessage = (ev) => handler.onEvent?.('message', ev.data);
    es.addEventListener('log', (ev) => handler.onLog?.((ev as MessageEvent).data));
    es.addEventListener('status', (ev) => {
      try {
        handler.onStatus?.(JSON.parse((ev as MessageEvent).data));
      } catch {
        handler.onStatus?.((ev as MessageEvent).data);
      }
    });
    es.addEventListener('end', (ev) => {
      try {
        handler.onEnd?.(JSON.parse((ev as MessageEvent).data));
      } catch {
        handler.onEnd?.((ev as MessageEvent).data);
      }
      endedNormally = true;
      es?.close();
      es = null;
    });
  }

  function scheduleReconnect() {
    if (cancelled || endedNormally) return;
    const delay = RECONNECT_BACKOFF_MS[Math.min(retry, RECONNECT_BACKOFF_MS.length - 1)];
    retry++;
    setTimeout(() => {
      if (!cancelled && !endedNormally) connect();
    }, delay);
  }

  void connect();

  return () => {
    cancelled = true;
    es?.close();
    es = null;
  };
}
