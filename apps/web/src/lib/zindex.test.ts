// Layer lint (UI polish U1): every z-index in the game UI comes from the --z-* scale in app.css. A literal number
// (`z-index: 40`, Tailwind `z-40` / `z-[60]` / `-z-10`) anywhere in src/lib or src/routes/(game) fails. Computed depth
// inside a scene view (`style:z-index={depth(y)}`) is data, not a layer, and is allowed.
import fs from 'node:fs';
import path from 'node:path';
import { describe, expect, it } from 'vitest';

const src = path.resolve(__dirname, '..');
const roots = ['lib', 'routes/(game)'].map((r) => path.join(src, r));

function* files(dir: string): Generator<string> {
	for (const e of fs.readdirSync(dir, { withFileTypes: true })) {
		const p = path.join(dir, e.name);
		if (e.isDirectory()) yield* files(p);
		else if (/\.(svelte|ts|css)$/.test(e.name) && !e.name.endsWith('.test.ts')) yield p;
	}
}

const bareCss = /z-index\s*:\s*-?\d/;
const bareClass = /(?:^|[\s"'`{])-?z-(?:\d|\[-?\d)/;

describe('z-index scale', () => {
	it('app.css defines the scale', () => {
		const css = fs.readFileSync(path.join(src, 'app.css'), 'utf8');
		for (const t of ['scene', 'scene-ui', 'stage', 'chat', 'bar', 'hud', 'wipe', 'mode', 'toast', 'boot']) expect(css).toMatch(new RegExp(`--z-${t}:\\s*\\d+;`));
	});
	it('no bare z-index numbers outside app.css', () => {
		const bad: string[] = [];
		for (const root of roots)
			for (const f of files(root)) {
				const lines = fs.readFileSync(f, 'utf8').split('\n');
				lines.forEach((l, i) => {
					if (bareCss.test(l) || bareClass.test(l)) bad.push(`${path.relative(src, f)}:${i + 1}: ${l.trim()}`);
				});
			}
		expect(bad).toEqual([]);
	});
	it('the lint itself catches both forms', () => {
		expect(bareCss.test('		z-index: 40;')).toBe(true);
		expect(bareCss.test('		z-index: var(--z-hud);')).toBe(false);
		expect(bareClass.test('<div class="fixed z-40 p-2">')).toBe(true);
		expect(bareClass.test('<div class="absolute -z-10">')).toBe(true);
		expect(bareClass.test('<div class="z-[60]">')).toBe(true);
		expect(bareClass.test('<div class="z-(--z-mode)">')).toBe(false);
	});
});
