import { describe, expect, it } from 'vitest';
import { BOOT_MAX_MS, bootDone, bootMinMs, bootProgress, needsTapToEnter } from './boot';

describe('boot screen timing', () => {
	it('waits for every step, but not past the hard limit', () => {
		expect(bootDone(2, 3, 1000, 'full')).toBe(false);
		expect(bootDone(3, 3, 1000, 'full')).toBe(true);
		expect(bootDone(0, 3, BOOT_MAX_MS, 'full')).toBe(true);
	});
	it('a cached load stays a moment (no flash) — none when effects are off — and well under 2 s', () => {
		expect(bootDone(3, 3, 100, 'full')).toBe(false);
		expect(bootDone(3, 3, 0, 'off')).toBe(true);
		for (const q of ['full', 'lite', 'off'] as const) expect(bootMinMs(q)).toBeLessThan(2000);
	});
	it('progress is a clamped ratio', () => {
		expect(bootProgress(0, 3)).toBe(0);
		expect(bootProgress(3, 3)).toBe(1);
		expect(bootProgress(5, 3)).toBe(1);
		expect(bootProgress(0, 0)).toBe(1);
	});
});

describe('Chạm để vào', () => {
	it('only gates the boot for real scene music/ambience, sound on and audio locked', () => {
		expect(needsTapToEnter({ loopFile: true, muted: false, unlocked: false })).toBe(true);
		expect(needsTapToEnter({ loopFile: false, muted: false, unlocked: false })).toBe(false); // synth SFX only (today)
		expect(needsTapToEnter({ loopFile: true, muted: true, unlocked: false })).toBe(false); // never block a muted player
		expect(needsTapToEnter({ loopFile: true, muted: false, unlocked: true })).toBe(false);
	});
});
