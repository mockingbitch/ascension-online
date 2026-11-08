import { describe, expect, it } from 'vitest';
import { cacheName, classifyRequest, OFFLINE_PATH, precacheList, staleCaches, type SwRequest } from './routes';

const O = 'https://tiendao.example';
const assets = new Set(['/favicon.svg', '/manifest.webmanifest', '/icons/icon-192.png', '/_app/immutable/entry/app.abc.js']);
const get = (path: string, mode = 'cors', extra: Partial<SwRequest> = {}) => classifyRequest({ url: O + path, method: 'GET', mode, ...extra }, O, assets);

describe('service worker routes', () => {
	it('never touches the API, the OAuth redirects, the socket or data requests', () => {
		expect(get('/api/v1/character')).toBe('bypass');
		expect(get('/api/v1/auth/google/start', 'navigate')).toBe('bypass');
		expect(get('/api/v1/auth/google/callback?code=x&state=y', 'navigate')).toBe('bypass');
		expect(get('/api/v1/auth/refresh')).toBe('bypass');
		expect(get('/api')).toBe('bypass');
		expect(get('/ws')).toBe('bypass');
		expect(get('/ws', 'websocket', { upgrade: 'websocket' })).toBe('bypass');
		expect(get('/play/__data.json')).toBe('bypass');
		expect(get('/_app/version.json')).toBe('bypass');
		expect(classifyRequest({ url: O + '/_app/immutable/entry/app.abc.js', method: 'POST', mode: 'cors' }, O, assets)).toBe('bypass');
		expect(get('/metrics')).toBe('bypass');
	});

	it('navigations are network-only (offline fallback), never cached', () => {
		expect(get('/play', 'navigate')).toBe('navigate');
		expect(get('/', 'navigate')).toBe('navigate');
		expect(get('/login', 'navigate')).toBe('navigate');
		expect(get('/play?win=settings', 'navigate')).toBe('navigate');
	});

	it('caches immutable build files and the listed static files only', () => {
		expect(get('/_app/immutable/chunks/x.123.js')).toBe('immutable');
		expect(get('/_app/immutable/assets/scene.abc.webp')).toBe('immutable');
		expect(get('/favicon.svg')).toBe('static');
		expect(get('/manifest.webmanifest')).toBe('static');
		expect(get('/icons/icon-192.png')).toBe('static');
		expect(get('/something-else.png')).toBe('bypass');
		expect(get('/favicon.svg?v=2')).toBe('bypass');
	});

	it('ignores other origins (Google Fonts, Google sign-in)', () => {
		expect(classifyRequest({ url: 'https://fonts.gstatic.com/s/x.woff2', method: 'GET', mode: 'cors' }, O, assets)).toBe('bypass');
		expect(classifyRequest({ url: 'https://accounts.google.com/o/oauth2/auth', method: 'GET', mode: 'navigate' }, O, assets)).toBe('bypass');
		expect(classifyRequest({ url: 'not a url', method: 'GET', mode: 'cors' }, O, assets)).toBe('bypass');
	});

	it('precaches code, styles, small statics and the offline page; cleans old versions only', () => {
		const list = precacheList(['/_app/immutable/a.js', '/_app/immutable/a.css', '/_app/immutable/big.webp', '/_app/version.json'], ['/favicon.svg', '/manifest.webmanifest', '/robots.txt']);
		expect(list).toEqual(['/_app/immutable/a.js', '/_app/immutable/a.css', '/favicon.svg', '/manifest.webmanifest', OFFLINE_PATH]);
		expect(staleCaches([cacheName('1'), cacheName('2'), 'someone-else'], '2')).toEqual([cacheName('1')]);
	});
});
