import { describe, expect, it } from 'vitest';
import { Settings, SETTINGS_KEY } from './settings.svelte';

const memStorage = (init: Record<string, string> = {}) => {
	const m = new Map(Object.entries(init));
	return { getItem: (k: string) => m.get(k) ?? null, setItem: (k: string, v: string) => void m.set(k, v), m };
};

describe('Settings', () => {
	it('defaults effects to the reduced-motion preference', () => {
		expect(new Settings({ reducedMotion: () => true }).fx).toBe('off');
		expect(new Settings({ reducedMotion: () => false }).fx).toBe('full');
	});

	it('persists and restores choices', () => {
		const storage = memStorage();
		const a = new Settings({ storage });
		a.setFx('lite');
		a.setMuted(true);
		a.setVolume('bgm', 0.3);
		a.setVolume('sfx', 9);
		const b = new Settings({ storage, reducedMotion: () => true });
		expect(b.fx).toBe('lite');
		expect(b.audio.muted).toBe(true);
		expect(b.audio.volumes.bgm).toBe(0.3);
		expect(b.audio.volumes.sfx).toBe(1);
	});

	it('survives broken or throwing storage', () => {
		const bad = new Settings({ storage: memStorage({ [SETTINGS_KEY]: '{not json' }) });
		expect(bad.fx).toBe('full');
		const throwing = {
			getItem: () => {
				throw new Error('denied');
			},
			setItem: () => {
				throw new Error('denied');
			}
		};
		const s = new Settings({ storage: throwing });
		expect(() => s.setFx('off')).not.toThrow();
		expect(s.fx).toBe('off');
	});

	it('renderer: 2D by default, ?renderer=3d turns 3D on for Tử Tiêu Cung only', () => {
		expect(new Settings({}).rendererFor('tu_tieu_cung')).toBe('2d');
		const s = new Settings({ search: '?renderer=3d&fps=1' });
		expect(s.rendererRequested).toBe('3d');
		expect(s.renderer).toBe('3d');
		expect(s.rendererFor('tu_tieu_cung')).toBe('3d');
		expect(s.rendererFor('world')).toBe('2d');
		expect(s.fpsGuard).toBe(true);
		expect(new Settings({ search: '?renderer=3d&fpsguard=0' }).fpsGuard).toBe(false);
	});

	it('persists the renderer choice and a fallback; picking 3D again clears the fallback', () => {
		const storage = memStorage();
		const a = new Settings({ storage });
		a.setFx('lite');
		expect(JSON.parse(storage.m.get(SETTINGS_KEY)!)).not.toHaveProperty('renderer'); // 2D players' data unchanged
		a.setRenderer('3d');
		expect(new Settings({ storage }).rendererFor('tu_tieu_cung')).toBe('3d');
		a.recordFallback3d('slow', 1234);
		const b = new Settings({ storage });
		expect(b.rendererChoice).toBe('3d');
		expect(b.fallback3d).toEqual({ reason: 'slow', at: 1234 });
		expect(b.rendererFor('tu_tieu_cung')).toBe('2d');
		expect(new Settings({ storage, search: '?renderer=3d' }).rendererFor('tu_tieu_cung')).toBe('2d'); // the URL too
		b.setRenderer('3d');
		const c = new Settings({ storage });
		expect(c.fallback3d).toBeNull();
		expect(c.rendererFor('tu_tieu_cung')).toBe('3d');
		c.setRenderer('2d');
		expect(new Settings({ storage }).rendererFor('tu_tieu_cung')).toBe('2d');
	});

	it('ignores junk renderer fields in storage', () => {
		const junk = { fx: 'full', renderer: 'webgpu', fallback3d: { reason: 'nope', at: 1 } };
		const s = new Settings({ storage: memStorage({ [SETTINGS_KEY]: JSON.stringify(junk) }) });
		expect(s.rendererChoice).toBe('2d');
		expect(s.fallback3d).toBeNull();
	});
});
