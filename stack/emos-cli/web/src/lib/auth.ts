// Token storage. The pair flow writes the token here, and the api client
// reads it for the Authorization header. Stored in localStorage so it
// survives reload — the daemon issues 90-day tokens.

const TOKEN_KEY = 'emos.token';

export function getToken(): string | null {
  try {
    return localStorage.getItem(TOKEN_KEY);
  } catch {
    return null;
  }
}

export function setToken(token: string): void {
  try {
    localStorage.setItem(TOKEN_KEY, token);
  } catch {
    /* private mode — nothing we can do */
  }
}

export function clearToken(): void {
  try {
    localStorage.removeItem(TOKEN_KEY);
  } catch {}
}
