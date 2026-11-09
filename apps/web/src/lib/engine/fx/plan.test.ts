import { describe, expect, it } from 'vitest';
import type { FxEvent } from './events';
import { planFx } from './plan';

const settled = (earnedMilli: number, extra: Partial<Extract<FxEvent, { type: 'settled' }>> = {}): FxEvent => ({
	type: 'settled',
	earnedMilli,
	source: 'settle',
	stagesAdvanced: [],
	...extra
});

describe('fxBus → effect commands', () => {
	it('settle: orbs from the cultivator sized by the server value, with the float text', () => {
		const [c] = planFx(settled(12_345), 'full');
		expect(c).toMatchObject({ kind: 'orbs', from: 'cultivator', count: 16, float: '+12,3 tu vi' });
		expect(planFx(settled(12_345, { source: 'incense' }), 'full')[0]).toMatchObject({ from: 'pointer' });
		expect(planFx(settled(0), 'full')).toEqual([]);
	});
	it('insight: a gold ring from the button, then orbs from the seat', () => {
		const cs = planFx({ type: 'insight', earnedMilli: 5000, takeaway: 'x' }, 'full');
		expect(cs.map((c) => c.kind)).toEqual(['ring', 'orbs']);
		expect(cs[1]).toMatchObject({ from: 'seat' });
		expect(planFx({ type: 'insight', earnedMilli: 0, takeaway: 'x' }, 'full').map((c) => c.kind)).toEqual(['ring']);
	});
	it('reward: one flight per line (items → bag, currency → stones), rare by catalogue rarity, staggered', () => {
		const rarity = (id: string) => (id === 'kiem' ? 'thien' : 'pham');
		const cs = planFx(
			{ type: 'reward', items: [{ itemId: 'kiem', quantity: 1 }, { itemId: 'thao', quantity: 3 }], currencies: [{ currency: 'spirit_stone', amount: 50 }], source: 'trial' },
			'full',
			rarity
		);
		expect(cs).toMatchObject([
			{ kind: 'fly', to: 'bag', itemId: 'kiem', rare: true, delay: 0 },
			{ kind: 'fly', to: 'bag', itemId: 'thao', rare: false, quantity: 3 },
			{ kind: 'fly', to: 'stones', currency: 'spirit_stone', quantity: 50 }
		]);
		expect((cs[2] as { delay: number }).delay).toBeGreaterThan((cs[1] as { delay: number }).delay);
		// lite: fewer flights, no rays
		const lite = planFx({ type: 'reward', items: [{ itemId: 'kiem', quantity: 1 }, { itemId: 'a' }, { itemId: 'b' }], currencies: [], source: 'task' }, 'lite', rarity);
		expect(lite).toHaveLength(2);
		expect(lite[0]).toMatchObject({ rare: false });
	});
	it('announcement → scroll; works level up → fireworks; stageUp → one light stage-up command', () => {
		expect(planFx({ type: 'announcement', id: 'a:1', text: 'T', kind: 'milestone' }, 'full')).toEqual([{ kind: 'scroll', id: 'a:1', title: 'Thiên Đạo Sử', text: 'T' }]);
		expect(planFx({ type: 'worksLevelUp', factionId: 'f', unlocked: 2 }, 'full')).toEqual([{ kind: 'firework', count: 3 }]);
		expect(planFx({ type: 'worksLevelUp', factionId: 'f', unlocked: 2 }, 'lite')).toEqual([{ kind: 'firework', count: 1 }]);
		expect(planFx({ type: 'stageUp', from: 'a', to: 'b' }, 'full')).toEqual([{ kind: 'stageUp', text: 'Lên tầng: a → b', fxTier: 1, particles: 6, sweepMs: 700 }]);
	});
	it('effects off: no motion at all — only the static "+X tu vi" label stays', () => {
		const off = (e: FxEvent) => planFx(e, 'off', () => 'tien');
		expect(off(settled(9000))).toEqual([{ kind: 'float', text: '+9 tu vi', tone: 'jade' }]);
		expect(off({ type: 'insight', earnedMilli: 9000, takeaway: '' })).toEqual([{ kind: 'float', text: '+9 tu vi', tone: 'jade' }]);
		expect(off({ type: 'reward', items: [{ itemId: 'x', quantity: 1 }], currencies: [], source: 'shop' })).toEqual([]);
		expect(off({ type: 'announcement', id: '1', text: 't', kind: 'event' })).toEqual([]);
		expect(off({ type: 'worksLevelUp', factionId: 'f', unlocked: 1 })).toEqual([]);
		for (const e of [settled(9000), { type: 'insight', earnedMilli: 9000, takeaway: '' } as FxEvent])
			expect(off(e).some((c) => c.kind === 'orbs' || c.kind === 'ring' || c.kind === 'fly')).toBe(false);
	});
});
