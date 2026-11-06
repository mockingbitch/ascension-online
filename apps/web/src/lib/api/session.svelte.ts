// Client session: the access token lives ONLY in memory (never localStorage). The refresh token is an
// HttpOnly cookie the browser sends to /api/v1/auth/refresh; JS cannot read it.
import type { SessionResponse, User } from '$lib/game/types';

export const session = $state<{ accessToken: string | null; user: User | null; expiresAt: number }>({
	accessToken: null,
	user: null,
	expiresAt: 0
});

let inflight: Promise<boolean> | null = null;

/** Exchange the refresh cookie for a new access token. Concurrent callers share one request. */
export function refreshSession(f: typeof fetch = fetch): Promise<boolean> {
	inflight ??= (async () => {
		try {
			const res = await f('/api/v1/auth/refresh', {
				method: 'POST',
				headers: { 'X-Requested-With': 'tiendao' }
			});
			if (!res.ok) {
				clearSession();
				return false;
			}
			const body = (await res.json()) as SessionResponse;
			session.accessToken = body.accessToken;
			session.user = body.user;
			session.expiresAt = Date.now() + body.expiresIn * 1000;
			return true;
		} catch {
			return false;
		} finally {
			inflight = null;
		}
	})();
	return inflight;
}

export function clearSession() {
	session.accessToken = null;
	session.user = null;
	session.expiresAt = 0;
}

/** Ensure there is a usable access token (refreshing ~30s before expiry). */
export async function ensureSession(f: typeof fetch = fetch): Promise<boolean> {
	if (session.accessToken && Date.now() < session.expiresAt - 30_000) return true;
	return refreshSession(f);
}

export function googleLoginUrl(returnTo = '/play'): string {
	return `/api/v1/auth/google/start?return=${encodeURIComponent(returnTo)}`;
}

export async function logout() {
	await fetch('/api/v1/auth/logout', { method: 'POST' }).catch(() => {});
	clearSession();
}
