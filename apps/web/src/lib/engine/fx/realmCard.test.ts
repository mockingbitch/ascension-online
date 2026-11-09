import { describe, expect, it } from 'vitest';
import { planBreakthrough } from './breakthrough';
import { planFx } from './plan';
import { CHI_TON_SEAL, hasChiTonSeal, realmCard } from './realmCard';

const input = {
	name: 'Vân Thanh Y',
	realmName: 'Thái Ất Kim Tiên',
	stageName: 'Sơ Kỳ Nhất Trọng',
	fxTier: 6,
	reachedAt: '2026-11-09T05:00:00Z',
	rank: 3,
	title: 'Sơ Tâm',
	faction: 'Tam Thanh',
	seal: false
};

describe('realm achievement card', () => {
	it('carries only display data — no email, no id, no token', () => {
		const m = realmCard(input);
		expect(Object.keys(m).sort()).toEqual(
			['date', 'extraMs', 'faction', 'fileName', 'fxTier', 'heading', 'name', 'particles', 'rank', 'realmName', 'seal', 'stageName', 'title'].sort()
		);
		expect(JSON.stringify(m)).not.toMatch(/@|[0-9a-f]{8}-[0-9a-f]{4}-/i);
		expect(m.rank).toBe('Thiên Bảng hạng 3');
		expect(m.fileName).toBe('tien-dao-thai-at-kim-tien.png');
		expect(realmCard({ ...input, rank: 0, reachedAt: null })).toMatchObject({ rank: 'Thiên Bảng: chưa xếp hạng', date: '' });
	});
	it('the chi_ton seal (Tiên Duyên 12, decision #6) is a static mark: no particles, no time, the same celebration', () => {
		const plain = realmCard(input);
		const sealed = realmCard({ ...input, seal: true });
		expect([sealed.particles, sealed.extraMs]).toEqual([0, 0]);
		expect({ ...sealed, seal: false }).toEqual(plain);
		// the celebration never sees the perk: same plan for every quality
		const e = { result: 'success', kind: 'major', tribulation: true, fxTier: 6 } as const;
		for (const q of ['full', 'lite', 'off'] as const) {
			const a = planBreakthrough(e, q);
			expect(planBreakthrough(e, q)).toEqual(a);
			const fx = { type: 'breakthrough', result: 'success', kind: 'major', tribulation: true, lossMilli: 0, from: 'a', to: 'b', failStreak: 0 } as const;
			expect(planFx(fx, q, undefined, false, { tierOf: () => 6 })).toEqual(planFx(fx, q, undefined, false, { tierOf: () => 6 }));
		}
		expect(hasChiTonSeal(['title:chi_ton', CHI_TON_SEAL])).toBe(true);
		expect(hasChiTonSeal(['title:vip_11'])).toBe(false);
		expect(hasChiTonSeal(undefined)).toBe(false);
	});
});
