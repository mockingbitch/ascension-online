/// <reference no-default-lib="true"/>
/// <reference lib="esnext" />
/// <reference lib="webworker" />
/// <reference types="@sveltejs/kit" />
// Service worker (spec §4.11, SvelteKit built-in, registered by Kit on every page). Caches static assets only — the
// rules live in lib/sw/routes.ts (pure, unit-tested): content-hashed /_app/immutable/* and the static files
// cache-first in one versioned cache (old versions deleted on activate); page navigations network-only with the
// precached /offline page as the fallback when the network is down; /api/* (incl. the Google OAuth redirects), /ws
// and SvelteKit data requests are never answered by the SW at all.
import { build, files, prerendered, version } from '$service-worker';
import { cacheName, classifyRequest, OFFLINE_PATH, precacheList, staleCaches } from '$lib/sw/routes';

const sw = self as unknown as ServiceWorkerGlobalScope;
const CACHE = cacheName(version);
const ASSETS = new Set([...build, ...files, ...prerendered]);

sw.addEventListener('install', (event) => {
	event.waitUntil(
		caches
			.open(CACHE)
			.then((c) => c.addAll(precacheList(build, files)))
			.then(() => sw.skipWaiting())
	);
});

sw.addEventListener('activate', (event) => {
	event.waitUntil(
		caches
			.keys()
			.then((keys) => Promise.all(staleCaches(keys, version).map((k) => caches.delete(k))))
			.then(() => sw.clients.claim())
	);
});

async function cacheFirst(req: Request): Promise<Response> {
	const cache = await caches.open(CACHE);
	const hit = await cache.match(req);
	if (hit) return hit;
	const res = await fetch(req);
	if (res.ok && res.type === 'basic') void cache.put(req, res.clone());
	return res;
}

async function navigate(req: Request): Promise<Response> {
	try {
		return await fetch(req);
	} catch {
		const offline = await caches.match(OFFLINE_PATH, { cacheName: CACHE });
		if (!offline) return Response.error();
		// Kit writes relative asset URLs (./_app/…): pin them to the root, the page may stand in for /play/tu-luyen
		const html = (await offline.text()).replace('<head>', '<head><base href="/" />');
		return new Response(html, { status: 200, headers: { 'content-type': 'text/html; charset=utf-8', 'cache-control': 'no-store' } });
	}
}

sw.addEventListener('fetch', (event) => {
	const req = event.request;
	const route = classifyRequest({ url: req.url, method: req.method, mode: req.mode, upgrade: req.headers.get('upgrade') }, sw.location.origin, ASSETS);
	if (route === 'bypass') return; // network, untouched
	event.respondWith(route === 'navigate' ? navigate(req) : cacheFirst(req));
});
