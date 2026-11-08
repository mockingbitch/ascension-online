// Service-worker request classification (spec §4.11). Pure (unit-tested in routes.test.ts); src/service-worker.ts
// only executes the answer. The SW caches static assets ONLY:
//   immutable  /_app/immutable/* (content-hashed JS/CSS/art) → cache-first, never revalidated
//   static     other build/static files listed by $service-worker (icons, manifest, favicon) → cache-first
//   navigate   page navigations → network-only (the HTML is never cached: it may carry auth state); when the network
//              fails, the precached offline page is shown
//   bypass     everything else is not touched at all (no respondWith): /api/* (incl. the Google OAuth redirects
//              /api/v1/auth/google/*), /ws, SvelteKit data (__data.json), version.json, other origins, non-GET.

export type SwRoute = 'bypass' | 'immutable' | 'static' | 'navigate';

/** Path prefixes the SW must never answer (network straight from the page, no SW involvement). */
export const NEVER_TOUCH = ['/api/', '/api', '/ws', '/auth/', '/oauth', '/login/callback', '/metrics'];

export const OFFLINE_PATH = '/offline';

export interface SwRequest {
	url: string;
	method: string;
	/** Request.mode ('navigate' for page loads) */
	mode: string;
	/** Request.headers.get('upgrade') */
	upgrade?: string | null;
}

const neverTouch = (path: string) => NEVER_TOUCH.some((p) => (p.endsWith('/') ? path.startsWith(p) : path === p || path.startsWith(p + '/')));

export function classifyRequest(r: SwRequest, origin: string, assets: ReadonlySet<string>): SwRoute {
	if (r.method !== 'GET' || (r.upgrade && r.upgrade.toLowerCase() === 'websocket')) return 'bypass';
	let u: URL;
	try {
		u = new URL(r.url);
	} catch {
		return 'bypass';
	}
	if (u.origin !== origin) return 'bypass';
	const path = u.pathname;
	if (neverTouch(path)) return 'bypass';
	if (r.mode === 'navigate') return 'navigate';
	if (path.endsWith('/__data.json') || path.startsWith('/_app/version')) return 'bypass';
	if (u.search) return 'bypass'; // cached assets are addressed without a query
	if (path.startsWith('/_app/immutable/')) return 'immutable';
	if (assets.has(path)) return 'static';
	return 'bypass';
}

/** Files precached at install: code + styles + the small static set (art loads on demand, then is cached). */
export function precacheList(build: readonly string[], files: readonly string[]): string[] {
	const code = build.filter((p) => /\.(js|css)$/.test(p));
	const statics = files.filter((p) => /\.(svg|png|webmanifest|ico)$/.test(p));
	return [...new Set([...code, ...statics, OFFLINE_PATH])];
}

export const CACHE_PREFIX = 'tiendao-static-';
export const cacheName = (version: string) => CACHE_PREFIX + version;

/** Caches to delete on activate: ours from other versions (never foreign caches). */
export const staleCaches = (keys: readonly string[], version: string) => keys.filter((k) => k.startsWith(CACHE_PREFIX) && k !== cacheName(version));
