import { describe, expect, it } from 'vitest';
import { AUDIO_SLOTS, pickFiles, slotStatus } from './manifest';
import { PEAK, RECIPES, recipeLength, render, renderSynth } from './synth';

describe('synth placeholders', () => {
	it('every synth slot has a recipe, and every recipe is a declared one-shot slot', () => {
		const synthSlots = AUDIO_SLOTS.filter((s) => s.synth).map((s) => s.id);
		expect(Object.keys(RECIPES).sort()).toEqual([...synthSlots].sort());
		for (const s of AUDIO_SLOTS) if (s.synth) expect(s.loop).toBe(false);
	});

	it('renders each one: audible, peak ≤ −3 dBFS, within its length budget', () => {
		for (const [id, r] of Object.entries(RECIPES)) {
			const pcm = render(r, 8000);
			let peak = 0;
			for (const v of pcm) peak = Math.max(peak, Math.abs(v));
			expect(peak, id).toBeGreaterThan(0.5);
			expect(peak, id).toBeLessThanOrEqual(PEAK + 1e-6);
			expect(recipeLength(r), id).toBeLessThanOrEqual(2.1);
			expect(pcm.length).toBe(Math.ceil(recipeLength(r) * 8000));
		}
		expect(recipeLength(RECIPES['ui.click']!)).toBeLessThanOrEqual(0.12);
	});

	it('is deterministic and returns null for unknown ids', async () => {
		const a = render(RECIPES['sfx.thunder']!, 8000);
		const b = render(RECIPES['sfx.thunder']!, 8000);
		expect(a).toEqual(b);
		expect(await renderSynth('bgm.world', 8000)).toBeNull();
	});
});

describe('audio manifest', () => {
	it('prefers OGG, falls back to MP3', () => {
		const p = ['/src/lib/assets/audio/ui.click.mp3', '/src/lib/assets/audio/ui.click.ogg', '/src/lib/assets/audio/bgm/world.mp3'];
		expect([...pickFiles(p, true)]).toEqual([
			['bgm.world', '/src/lib/assets/audio/bgm/world.mp3'],
			['ui.click', '/src/lib/assets/audio/ui.click.ogg']
		]);
		expect(pickFiles(p, false).get('ui.click')).toBe('/src/lib/assets/audio/ui.click.mp3');
	});

	it('marks music/ambience missing until produced, SFX as synth placeholders, files win', () => {
		const st = slotStatus(new Set(['sfx.settle']));
		expect(st['bgm.world']).toBe('missing');
		expect(st['amb.cave']).toBe('missing');
		expect(st['ui.click']).toBe('synth');
		expect(st['sfx.settle']).toBe('file');
	});
});
