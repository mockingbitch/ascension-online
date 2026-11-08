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

	it('reads ?renderer= but stays on 2D', () => {
		const s = new Settings({ search: '?renderer=3d&fps=1' });
		expect(s.rendererRequested).toBe('3d');
		expect(s.renderer).toBe('2d');
	});
});
