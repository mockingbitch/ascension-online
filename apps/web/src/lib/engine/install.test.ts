import { describe, expect, it } from 'vitest';
import { fullscreenApi } from './fullscreen.svelte';
import { installHintMode, isIos } from './install.svelte';

const env = { standalone: false, dismissed: false, mobile: true, hasPrompt: false, ios: false };

describe('install hint', () => {
	it('offers the native prompt on phones, iOS instructions otherwise, nothing when installed/dismissed/desktop', () => {
		expect(installHintMode({ ...env, hasPrompt: true })).toBe('prompt');
		expect(installHintMode({ ...env, ios: true })).toBe('ios');
		expect(installHintMode(env)).toBeNull();
		expect(installHintMode({ ...env, hasPrompt: true, dismissed: true })).toBeNull();
		expect(installHintMode({ ...env, ios: true, standalone: true })).toBeNull();
		expect(installHintMode({ ...env, hasPrompt: true, mobile: false })).toBeNull();
	});

	it('detects iPhone and iPadOS', () => {
		expect(isIos('Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X)', 5)).toBe(true);
		expect(isIos('Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)', 5)).toBe(true);
		expect(isIos('Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)', 0)).toBe(false);
		expect(isIos('Mozilla/5.0 (Linux; Android 14)', 5)).toBe(false);
	});
});

describe('fullscreen support', () => {
	const fn = () => {};
	it('standard API, webkit fallback, none on iPhone Safari', () => {
		expect(fullscreenApi({ fullscreenEnabled: true }, { requestFullscreen: async () => {} })).toBe('std');
		expect(fullscreenApi({ webkitFullscreenEnabled: true } as never, { webkitRequestFullscreen: fn } as never)).toBe('webkit');
		expect(fullscreenApi({ fullscreenEnabled: false }, {})).toBeNull();
		expect(fullscreenApi(undefined, undefined)).toBeNull();
	});
});
