import { describe, expect, it, vi } from 'vitest';
import { world } from './defs/world';
import { SceneStage, type WipeDriver } from './stage.svelte';
import type { SceneRenderer } from '../render/types';

function setup() {
	const st = new SceneStage();
	const log: string[] = [];
	const wipe: WipeDriver = {
		cover: vi.fn(async (k, ms) => void log.push(`cover:${k}:${ms}`)),
		reveal: vi.fn(async (k, ms) => void log.push(`reveal:${k}:${ms}`))
	};
	const renderer = { setCamera: vi.fn(async (id: string) => void log.push(`camera:${id}`)) } as unknown as SceneRenderer;
	st.wipe = wipe;
	st.renderer = renderer;
	st.def = world;
	return { st, log, wipe, renderer };
}

describe('SceneStage', () => {
	it('deep link: the route scene (or the default one under a screen) is shown without a transition', () => {
		const { st, log } = setup();
		st.sync('/play/nhan-vat');
		expect(st.shown).toBe('world');
		expect(st.screenOver).toBe(true);
		expect(st.covered).toBe(true);
		st.sync('/play/tu-luyen');
		expect(st.shown).toBe('dong_phu');
		expect(st.covered).toBe(false);
		expect(log).toEqual([]);
	});

	it('world → place: zoom to the hotspot, cover, swap, then reveal only after the new scene mounted', async () => {
		const { st, log } = setup();
		st.sync('/play');
		st.mounted('world');
		const token = await st.leave('/play/tu-tieu-cung', 'full');
		expect(log).toEqual(['camera:focus.hall', 'cover:wipe:325']);
		expect(st.shown).toBe('tu_tieu_cung');
		expect(st.manager.current).toBe('tu_tieu_cung');
		expect(st.phase).toBe('cover');
		const arrived = st.arrive(token);
		await Promise.resolve();
		expect(st.phase).toBe('cover'); // still waiting for the mount
		st.mounted('tu_tieu_cung');
		await arrived;
		expect(log.at(-1)).toBe('reveal:wipe:325');
		expect(st.phase).toBe('idle');
	});

	it('a screen over the scene keeps the scene (no transition), and back to it is free', async () => {
		const { st, log } = setup();
		st.sync('/play/tu-luyen');
		expect(await st.leave('/play/nhan-vat', 'full')).toBe(0);
		st.sync('/play/nhan-vat');
		expect(st.shown).toBe('dong_phu');
		expect(await st.leave('/play/tu-luyen', 'full')).toBe(0);
		expect(log).toEqual([]);
	});

	it('fx off swaps instantly; lite fades without zoom', async () => {
		const { st, log } = setup();
		st.sync('/play');
		st.mounted('world');
		await st.arrive(await st.leave('/play/tu-luyen', 'lite').then((t) => (st.mounted('dong_phu'), t)));
		expect(log).toEqual(['cover:fade:120', 'reveal:fade:140']);
		log.length = 0;
		await st.arrive(await st.leave('/play', 'off').then((t) => (st.mounted('world'), t)));
		expect(log).toEqual(['cover:instant:0', 'reveal:instant:0']);
		expect(st.shown).toBe('world');
	});

	it('a window covers the scene without changing it; closing uncovers', () => {
		const { st } = setup();
		st.sync('/play');
		st.setWindows([{ kind: 'settings' }]);
		expect(st.covered).toBe(true);
		expect(st.shown).toBe('world');
		st.setWindows(null);
		expect(st.covered).toBe(false);
		// a local modal (confirmation over a page) also covers it
		st.localWindows = 1;
		expect(st.covered).toBe(true);
		st.localWindows = 0;
	});

	it('a newer navigation supersedes an older one (stale arrive is ignored)', async () => {
		const { st } = setup();
		st.sync('/play');
		st.mounted('world');
		const a = await st.leave('/play/tu-luyen', 'full');
		const b = await st.leave('/play/tu-tieu-cung', 'full');
		await st.arrive(a); // stale: no reveal
		expect(st.phase).toBe('cover');
		st.mounted('tu_tieu_cung');
		await st.arrive(b);
		expect(st.phase).toBe('idle');
		expect(st.shown).toBe('tu_tieu_cung');
	});
});
