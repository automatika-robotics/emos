// Tiny EventSource wrapper. We want bearer auth on SSE too, but EventSource
// can't set headers, so we accept the token as a `?token=` query param
// (the daemon's auth middleware picks both up).

import { getToken } from './auth';
import { API_BASE } from './api';

export type SSEHandler = {
  onEvent?: (name: string, data: string) => void;
  onLog?: (line: string) => void;
  onStatus?: (data: unknown) => void;
  onEnd?: (data: unknown) => void;
  onOpen?: () => void;
  onError?: (err: Event) => void;
};

export function openSSE(path: string, handler: SSEHandler): () => void {
  const tok = getToken();
  const sep = path.includes('?') ? '&' : '?';
  const url = `${API_BASE}${path}${tok ? `${sep}token=${encodeURIComponent(tok)}` : ''}`;
  const es = new EventSource(url);

  es.onopen = () => handler.onOpen?.();
  es.onerror = (e) => handler.onError?.(e);

  // Default `message` event — backend uses it for unnamed payloads.
  es.onmessage = (ev) => handler.onEvent?.('message', ev.data);

  // Named events used by the API. We attach individually (rather than a
  // broad listener) so each callback gets typed handling.
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
    es.close();
  });

  return () => es.close();
}
