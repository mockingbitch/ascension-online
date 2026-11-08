import { describe, expect, it } from 'vitest';
import { RendererFailure } from './render3d/failure';
import { rendererFor, wantedRenderer } from './select';

const none = { requested: null, choice: '2d' as const, fallback: null };

describe('renderer selection (GĐ7)', () => {
	it('2D by default, for every scene', () => {
		for (const s of ['world', 'dong_phu', 'tu_tieu_cung', null] as const) expect(rendererFor(s, none)).toBe('2d');
	});

	it('?renderer=3d or the setting turns 3D on — for Tử Tiêu Cung only', () => {
		for (const p of [{ ...none, requested: '3d' }, { ...none, choice: '3d' as const }]) {
			expect(rendererFor('tu_tieu_cung', p)).toBe('3d');
			expect(rendererFor('world', p)).toBe('2d');
			expect(rendererFor('dong_phu', p)).toBe('2d');
		}
	});

	it('the URL wins over the setting; unknown values are ignored', () => {
		expect(wantedRenderer({ requested: '2d', choice: '3d' })).toBe('2d');
		expect(wantedRenderer({ requested: '3d', choice: '2d' })).toBe('3d');
		expect(wantedRenderer({ requested: 'webgpu', choice: '3d' })).toBe('3d');
		expect(wantedRenderer({ requested: 'webgpu', choice: '2d' })).toBe('2d');
	});

	it('a remembered fallback keeps the scene 2D whatever was asked', () => {
		const fallback = { reason: 'no-webgl2' as const, at: 1 };
		expect(rendererFor('tu_tieu_cung', { requested: '3d', choice: '3d', fallback })).toBe('2d');
	});

	it('RendererFailure carries its reason', () => {
		const e = new RendererFailure('context-lost');
		expect(e).toBeInstanceOf(Error);
		expect(e.reason).toBe('context-lost');
	});
});
