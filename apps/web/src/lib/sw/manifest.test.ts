// The web app manifest and <meta theme-color> hold literal hex (they cannot read CSS variables): keep them equal to
// the --color-ink-950 token, and the manifest installable (name, start_url, display, 192/512 + maskable icons).
import fs from 'node:fs';
import path from 'node:path';
import { describe, expect, it } from 'vitest';

const web = path.resolve(__dirname, '../../..');
const read = (p: string) => fs.readFileSync(path.join(web, p), 'utf8');

describe('web app manifest', () => {
	const m = JSON.parse(read('static/manifest.webmanifest'));
	const token = /--color-ink-950:\s*(#[0-9a-f]{6})/i.exec(read('src/app.css'))?.[1]?.toLowerCase();

	it('is installable', () => {
		expect(m.name).toBe('Tiên Đạo Online');
		expect(m.short_name.length).toBeLessThanOrEqual(12);
		expect(m.start_url).toBe('/play');
		expect(m.scope).toBe('/');
		expect(['standalone', 'fullscreen']).toContain(m.display);
		const sizes = m.icons.map((i: { sizes: string; purpose: string }) => `${i.sizes}:${i.purpose}`);
		expect(sizes).toEqual(expect.arrayContaining(['192x192:any', '512x512:any', '512x512:maskable']));
		for (const i of m.icons) expect(fs.existsSync(path.join(web, 'static', i.src)), i.src).toBe(true);
	});

	it('uses the ink-950 token colour (manifest + meta theme-color)', () => {
		expect(token).toBeTruthy();
		expect(m.theme_color.toLowerCase()).toBe(token);
		expect(m.background_color.toLowerCase()).toBe(token);
		expect(read('src/app.html')).toContain(`<meta name="theme-color" content="${token}" />`);
		expect(read('src/app.html')).toContain('rel="manifest"');
	});
});
