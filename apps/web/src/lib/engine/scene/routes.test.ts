import { describe, expect, it } from 'vitest';
import { world } from './defs/world';
import { DEFAULT_SCENE, isModePath, planTransition, resolveScene, sceneOfPath, TIMING, zoomPresetFor } from './routes';

describe('scene routes', () => {
	it('mini games are mode routes over the scene (a deep link builds the default scene, no transition)', () => {
		expect(isModePath('/play/tam-ma')).toBe(true);
		expect(isModePath('/play/bi-canh/')).toBe(true);
		expect(isModePath('/play/tu-luyen')).toBe(false);
		expect(resolveScene('/play/hoa-pham', null)).toBe(DEFAULT_SCENE);
		expect(resolveScene('/play/hoa-pham', 'dong_phu')).toBe('dong_phu');
		expect(planTransition('dong_phu', resolveScene('/play/tam-ma', 'dong_phu'), 'full')).toBeNull();
	});

	it('maps the three scene routes', () => {
		expect(sceneOfPath('/play')).toBe('world');
		expect(sceneOfPath('/play/')).toBe('world');
		expect(sceneOfPath('/play/tu-luyen')).toBe('dong_phu');
		expect(sceneOfPath('/play/tu-tieu-cung')).toBe('tu_tieu_cung');
		expect(sceneOfPath('/play/tui-do')).toBeNull();
	});

	it('screens keep the scene behind them; a deep link gets the default scene; outside /play there is none', () => {
		expect(resolveScene('/play/nhan-vat', 'dong_phu')).toBe('dong_phu');
		expect(resolveScene('/play/nhan-vat', null)).toBe(DEFAULT_SCENE);
		expect(resolveScene('/play/tu-tieu-cung', 'world')).toBe('tu_tieu_cung');
		expect(resolveScene('/login', 'world')).toBeNull();
		expect(resolveScene('/playground', null)).toBeNull();
	});
});

describe('planTransition', () => {
	it('full: ink wipe of --dur-scene, zooming toward the clicked hotspot from the world', () => {
		const p = planTransition('world', 'tu_tieu_cung', 'full', { fromDef: world, toPath: '/play/tu-tieu-cung' })!;
		expect(p.kind).toBe('wipe');
		expect(p.zoom).toBe('focus.hall');
		expect(p.coverMs + p.revealMs).toBe(650);
		expect(p.zoomMs).toBe(TIMING.wipe.zoom);
	});
	it('no zoom between two places', () => {
		const p = planTransition('dong_phu', 'tu_tieu_cung', 'full', { toPath: '/play/tu-tieu-cung' })!;
		expect(p).toMatchObject({ kind: 'wipe', zoom: null, zoomMs: 0 });
	});
	it('lite: a shorter plain fade without zoom; off: instant', () => {
		const lite = planTransition('world', 'dong_phu', 'lite', { fromDef: world, toPath: '/play/tu-luyen' })!;
		expect(lite).toMatchObject({ kind: 'fade', zoom: null });
		expect(lite.coverMs + lite.revealMs).toBeLessThan(650);
		expect(planTransition('world', 'dong_phu', 'off')).toMatchObject({ kind: 'instant', coverMs: 0, revealMs: 0 });
	});
	it('same scene or no target: nothing to do', () => {
		expect(planTransition('world', 'world', 'full')).toBeNull();
		expect(planTransition('world', null, 'full')).toBeNull();
	});
	it('zoom presets only for route hotspots that have a camera', () => {
		expect(zoomPresetFor(world, '/play/tu-luyen')).toBe('focus.cultivate');
		expect(zoomPresetFor(world, '/play/xep-hang')).toBeNull();
		expect(zoomPresetFor(null, '/play/tu-luyen')).toBeNull();
	});
});
