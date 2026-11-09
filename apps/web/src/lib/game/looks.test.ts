import fs from 'node:fs';
import path from 'node:path';
import { describe, expect, it } from 'vitest';
import { avatarSlots, bodyDefaults, bodySlots, defaultLook, figureOf, isBodyLevel, listenerSlots, meditateSlots, normalizeLook, optionsFor, outfitOf, sameLook, withBody, withOutfit } from './looks';
import type { BodyDef, PathId } from './types';
import type { AppearanceCatalog } from './types';

const outfits = (b: string, parts: [string, string, string][]) => parts.map(([id, robePaletteId, hairId]) => ({ id: `${b}${id}`, name: id, robePaletteId, hairId }));
const cat: AppearanceCatalog = {
	settings: { bodyChangeCooldownSeconds: 86400 },
	bodies: [
		{ id: 'a', path: 'tien', sex: 'nam', name: 'A', temperament: '', traits: '', svgHair: 'bui_cao', defaultHair: 'h', defaultRobe: 'r', defaultOutfit: 'a1', defaultPose: 'p', pair: 'b', defaultFree: true,
			outfits: outfits('a', [['1', 'r', 'h'], ['2', 'r2', 'h2'], ['3', 'r3', 'h']]) },
		{ id: 'c', path: 'tien', sex: 'nu', name: 'C', temperament: '', traits: '', svgHair: 'van_cao', beard: false, defaultHair: 'h2', defaultRobe: 'r2', defaultOutfit: 'c1', defaultPose: 'p', pair: 'b', defaultFree: true,
			outfits: outfits('c', [['1', 'r2', 'h2'], ['2', 'r', 'h'], ['3', 'r3', 'h2']]) },
		{ id: 'b', path: 'vu', sex: 'nam', name: 'B', temperament: '', traits: '', svgHair: 'chom', beard: true, defaultHair: 'h', defaultRobe: 'r', defaultOutfit: 'b1', defaultPose: 'q', pair: 'a', defaultFree: true,
			outfits: outfits('b', [['1', 'r', 'h'], ['2', 'r3', 'h'], ['3', 'rv', 'h']]) }
	],
	hairAccessories: [
		{ id: 'h', name: 'H', paths: ['tien', 'vu'], defaultFree: true },
		{ id: 'h2', name: 'H2', paths: ['tien'], defaultFree: true }
	],
	robePalettes: [
		{ id: 'r', name: 'R', paths: ['tien', 'vu'], defaultFree: true },
		{ id: 'r2', name: 'R2', paths: ['tien'], defaultFree: true },
		{ id: 'r3', name: 'R3', paths: ['tien', 'vu'], defaultFree: true },
		{ id: 'rv', name: 'RV', paths: ['vu'], defaultFree: true }
	],
	poses: [
		{ id: 'p', name: 'P', path: 'tien', defaultFree: true },
		{ id: 'q', name: 'Q', path: 'vu', defaultFree: true }
	]
};

describe('looks (display only)', () => {
	it('defaults to the first body of the path in its default outfit', () => {
		expect(defaultLook(cat, 'tien')).toEqual({ bodyId: 'a', outfitId: 'a1', hairId: 'h', robePaletteId: 'r', poseId: 'p' });
		expect(defaultLook(cat, 'vu')?.bodyId).toBe('b');
	});
	it('offers only bodies and poses of the path', () => {
		const o = optionsFor(cat, 'vu');
		expect(o.bodies.map((b) => b.id)).toEqual(['b']);
		expect(o.poses.map((x) => x.id)).toEqual(['q']);
	});
	it('switching body takes its default outfit and keeps the pose the path allows', () => {
		const tien = { bodyId: 'a', outfitId: 'a2', hairId: 'h2', robePaletteId: 'r2', poseId: 'p' };
		expect(withBody(cat, tien, 'c')).toEqual({ bodyId: 'c', outfitId: 'c1', hairId: 'h2', robePaletteId: 'r2', poseId: 'p' });
		expect(withBody(cat, tien, 'b')).toEqual({ bodyId: 'b', outfitId: 'b1', hairId: 'h', robePaletteId: 'r', poseId: 'q' });
		expect(withBody(cat, tien, 'nope')).toBe(tien);
	});
	it('an outfit brings its accessory and robe; unknown outfits change nothing', () => {
		const l = defaultLook(cat, 'tien')!;
		expect(withOutfit(cat, l, 'a3')).toEqual({ ...l, outfitId: 'a3', robePaletteId: 'r3', hairId: 'h' });
		expect(withOutfit(cat, l, 'b2')).toBe(l); // another body's outfit
	});
	it('finds the outfit by id, else by parts (old snapshots), else the default', () => {
		expect(outfitOf(cat, { bodyId: 'a', outfitId: 'a3', hairId: 'x', robePaletteId: 'x', poseId: 'p' })?.id).toBe('a3');
		expect(outfitOf(cat, { bodyId: 'a', hairId: 'h2', robePaletteId: 'r2', poseId: 'p' })?.id).toBe('a2');
		expect(outfitOf(cat, { bodyId: 'a', hairId: 'h2', robePaletteId: 'r', poseId: 'p' })?.id).toBe('a1');
		expect(normalizeLook(cat, { bodyId: 'a', hairId: 'h', robePaletteId: 'r3', poseId: 'p' })).toEqual({ bodyId: 'a', outfitId: 'a3', hairId: 'h', robePaletteId: 'r3', poseId: 'p' });
	});
	it('maps a look to the SVG figure with the outfit parts', () => {
		expect(figureOf(cat, { bodyId: 'b', outfitId: 'b3', hairId: 'h', robePaletteId: 'r', poseId: 'q' })).toEqual({
			sex: 'nam',
			path: 'vu',
			hair: 'chom',
			beard: true,
			acc: 'h',
			robe: 'rv',
			pose: 'q'
		});
		expect(figureOf(cat, { bodyId: 'gone', hairId: 'h', robePaletteId: 'r', poseId: 'p' })).toBeUndefined();
		expect(figureOf(undefined, { bodyId: 'a', hairId: 'h', robePaletteId: 'r', poseId: 'p' })).toBeUndefined();
	});
	it('names art slots: outfit folder → body folder (default outfit), exact for pickers', () => {
		const def = { bodyId: 'a', outfitId: 'a1', hairId: 'h', robePaletteId: 'r', poseId: 'p' };
		const alt = { ...def, outfitId: 'a2', hairId: 'h2', robePaletteId: 'r2' };
		const o = { defaultOutfit: 'a1' };
		expect(meditateSlots(def, o)).toEqual(['characters/a/meditate-p', 'characters/a/meditate']);
		expect(meditateSlots(alt, o)).toEqual(['characters/a/a2/meditate-p', 'characters/a/a2/meditate', 'characters/a/meditate-p', 'characters/a/meditate']);
		expect(meditateSlots(alt, { ...o, exact: true })).toEqual(['characters/a/a2/meditate-p', 'characters/a/a2/meditate']);
		expect(meditateSlots(def, { ...o, exact: true })).toEqual(['characters/a/meditate-p', 'characters/a/meditate']);
		expect(avatarSlots(alt, o)).toEqual(['characters/a/a2/avatar', 'characters/a/avatar']);
		expect(listenerSlots(alt, o)).toEqual(['characters/a/a2/listener', 'characters/a/listener']);
		expect(bodySlots(def, ['kim_dan', 'truc_co'], 2, o)).toEqual([
			'characters/a/body-kim_dan',
			'characters/a/body-b2',
			'characters/a/body-b1',
			'characters/a/body-truc_co'
		]);
		expect(bodySlots(alt, ['kim_dan'], 1, o)).toEqual(['characters/a/a2/body-kim_dan', 'characters/a/a2/body-b1', 'characters/a/body-kim_dan', 'characters/a/body-b1']);
		expect(isBodyLevel('a', 'characters/a/avatar')).toBe(true);
		expect(isBodyLevel('a', 'characters/a/a2/avatar')).toBe(false);
		expect(sameLook(def, { ...def })).toBe(true);
		expect(sameLook(def, alt)).toBe(false);
		expect(sameLook(def, { ...def, outfitId: undefined })).toBe(true);
	});
});

// The shipped catalogue and the web stay in step: every robe colour has a CSS token, every SVG hairstyle a drawing.
describe('appearances.yaml ↔ web', () => {
	const repo = path.resolve(__dirname, '../../../../..');
	const yaml = fs.readFileSync(path.join(repo, 'content/catalog/appearances.yaml'), 'utf8');
	const css = fs.readFileSync(path.resolve(__dirname, '../../app.css'), 'utf8');
	const figure = fs.readFileSync(path.resolve(__dirname, '../components/CultivatorFigure.svelte'), 'utf8');
	const section = (name: string) => yaml.slice(yaml.indexOf(`\n${name}:`) + 1).split(/\n(?=[a-z_]+:)/)[0]!;

	it('every robe palette has a --color-robe-<id> token', () => {
		const ids = [...section('robe_palettes').matchAll(/id:\s*([a-z][a-z0-9_]*)/g)].map((m) => m[1]!);
		expect(ids.length).toBeGreaterThan(0);
		for (const id of ids) expect(css, id).toMatch(new RegExp(`--color-robe-${id}:`));
	});
	it('every svg_hair, accessory and pose is drawn by CultivatorFigure', () => {
		const hairs = [...yaml.matchAll(/svg_hair:\s*([a-z_]+)/g)].map((m) => m[1]!);
		const accs = [...section('hair_accessories').matchAll(/id:\s*([a-z][a-z0-9_]*)/g)].map((m) => m[1]!).filter((id) => id !== 'khong');
		const poses = [...section('poses').matchAll(/id:\s*([a-z][a-z0-9_]*)/g)].map((m) => m[1]!);
		expect(hairs.length).toBeGreaterThanOrEqual(12);
		for (const id of [...new Set(hairs), ...accs]) expect(figure, id).toContain(`'${id}'`);
		for (const id of poses.filter((p) => p !== 'kiet_gia')) expect(figure, id).toContain(`'${id}'`);
	});
	// Minimal reader of the bodies section (no YAML dependency in the web app): id, path, sex, svg_hair, defaults, outfits.
	const shippedCatalog = (): AppearanceCatalog => {
		const bodies: BodyDef[] = [];
		for (const block of section('bodies').split(/\n  - id: /).slice(1)) {
			const field = (k: string) => block.match(new RegExp(`\\n    ${k}: (\\S+)`))?.[1] ?? '';
			const outs = block.split(/\n      - id: /).slice(1).map((o) => ({
				id: o.split('\n')[0]!.trim(),
				name: o.match(/name: (.+)/)?.[1] ?? '',
				robePaletteId: o.match(/robe_palette: (\S+)/)?.[1] ?? '',
				hairId: o.match(/hair_accessory: (\S+)/)?.[1] ?? ''
			}));
			const def = outs.find((o) => o.id === field('default_outfit'));
			bodies.push({
				id: block.split('\n')[0]!.trim(), path: field('path') as PathId, sex: field('sex') as BodyDef['sex'], name: field('name'),
				temperament: '', traits: '', svgHair: field('svg_hair') as BodyDef['svgHair'], beard: field('beard') === 'true',
				defaultHair: def?.hairId ?? '', defaultRobe: def?.robePaletteId ?? '', defaultOutfit: field('default_outfit'),
				defaultPose: field('default_pose'), pair: field('pair'), defaultFree: true, outfits: outs
			});
		}
		const opts = (name: string) => [...section(name).matchAll(/id:\s*([a-z][a-z0-9_]*),.*paths: \[([^\]]*)\]/g)].map((m) => ({ id: m[1]!, name: m[1]!, paths: m[2]!.split(',').map((x) => x.trim() as PathId), defaultFree: true }));
		const poses = [...section('poses').matchAll(/id:\s*([a-z][a-z0-9_]*),.*path: ([a-z]+)/g)].map((m) => ({ id: m[1]!, name: m[1]!, path: m[2]! as PathId, defaultFree: true }));
		return { settings: { bodyChangeCooldownSeconds: 86400 }, bodies, hairAccessories: opts('hair_accessories'), robePalettes: opts('robe_palettes'), poses };
	};
	const artRoot = path.resolve(__dirname, '../assets/art');
	const hasArt = (slot: string) => ['avif', 'webp', 'png'].some((e) => fs.existsSync(path.join(artRoot, `${slot}.${e}`)));

	// decision "Bộ trang phục vẽ sẵn": every outfit shows SOMETHING that is its own — raster art in the chain, or the
	// SVG figure in exactly the outfit's colours (robe token exists, accessory drawn); and the 3 outfits of a body
	// never draw the same SVG figure, so the picker preview changes on every choice.
	it('every outfit resolves to art or to its own SVG figure', () => {
		const sc = shippedCatalog();
		expect(sc.bodies).toHaveLength(12);
		for (const b of sc.bodies) {
			expect(b.outfits, b.id).toHaveLength(3);
			expect(b.outfits.map((o) => o.id), b.id).toContain(b.defaultOutfit);
			const figures = new Set<string>();
			for (const o of b.outfits) {
				expect(sc.hairAccessories.find((h) => h.id === o.hairId)?.paths, `${b.id}/${o.id} hair`).toContain(b.path);
				expect(sc.robePalettes.find((r) => r.id === o.robePaletteId)?.paths, `${b.id}/${o.id} robe`).toContain(b.path);
				const look = withOutfit(sc, bodyDefaults(b), o.id);
				expect(look.outfitId).toBe(o.id);
				const fig = figureOf(sc, look)!;
				expect(fig, `${b.id}/${o.id}`).toMatchObject({ acc: o.hairId, robe: o.robePaletteId });
				expect(css, `${o.id} robe token`).toMatch(new RegExp(`--color-robe-${fig.robe}:`));
				if (fig.acc !== 'khong') expect(figure, `${o.id} accessory`).toContain(`'${fig.acc}'`);
				figures.add(JSON.stringify(fig));
				const opts = { defaultOutfit: b.defaultOutfit };
				const art = [...avatarSlots(look, opts), ...meditateSlots(look, opts)].some(hasArt);
				// the default outfit's art is the body folder (shipped for all 12 bodies)
				if (o.id === b.defaultOutfit) expect(art, `${b.id} default outfit art`).toBe(true);
			}
			expect(figures.size, `${b.id}: outfits must look different`).toBe(3);
		}
	});
	it('12 free bodies: tiên 4 nam + 4 nữ, Vu 2 + 2 (decisions 2026-11-09 #2)', () => {
		const bodies = section('bodies');
		const count = (p: string, s: string) => [...bodies.matchAll(new RegExp(`path: ${p}\\n\\s+sex: ${s}\\n`, 'g'))].length;
		expect([count('tien', 'nam'), count('tien', 'nu'), count('vu', 'nam'), count('vu', 'nu')]).toEqual([4, 4, 2, 2]);
	});
});
