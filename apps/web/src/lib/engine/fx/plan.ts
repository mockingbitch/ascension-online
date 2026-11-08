// fxBus event → effect commands (spec §4.4–4.8). Pure: the director (director.ts) resolves positions and runs them.
// Every count/duration derives from the SERVER values in the event and the effects quality; 'off' keeps only static
// information (a "+X tu vi" label) — never motion. Breakthrough / lôi kiếp open the breakthrough MODE (GĐ5): a gather
// loop on `breakthroughStart`, the outcome timeline (breakthrough.ts) only on the server's `breakthrough` answer.
import { formatPoints } from '$lib/game/format';
import { planBreakthrough, planGather, type BtPlan, type GatherPlan } from './breakthrough';
import type { FxEvent, FxEventMap } from './events';
import type { Quality } from './particles';
import { isRare, motesForEarned, orbFlightMs, rewardFlights } from './presets';

/** Where an effect starts / ends; resolved to viewport points by the director. */
export type FxFrom = 'cultivator' | 'seat' | 'primary' | 'pointer';

export type FxCommand =
	| { kind: 'orbs'; from: FxFrom; count: number; ms: number; float: string }
	| { kind: 'float'; text: string; tone: 'jade' | 'gold' }
	| {
			kind: 'fly';
			to: 'bag' | 'stones';
			itemId?: string;
			currency?: string;
			quantity: number;
			rarity?: string;
			rare: boolean;
			delay: number;
			ms: number;
	  }
	| { kind: 'ring'; at: 'primary'; tone: 'jade' | 'gold' }
	| { kind: 'scroll'; id: string; title: string; text: string }
	| { kind: 'firework'; count: number }
	| { kind: 'mode'; mode: 'breakthrough'; step: 'gather'; gather: GatherPlan; tribulation: boolean }
	| { kind: 'mode'; mode: 'breakthrough'; step: 'outcome'; plan: BtPlan; result: FxEventMap['breakthrough'] }
	| { kind: 'mode'; mode: 'breakthrough'; step: 'stall'; message: string }
	| { kind: 'mode'; mode: 'breakthrough'; step: 'cancel' };

/** Gap between consecutive reward icons (ms). */
export const FLY_STAGGER_MS = 140;

const SCROLL_TITLE: Record<string, string> = {
	breakthrough: 'Thiên hạ chấn động',
	milestone: 'Thiên Đạo Sử',
	event: 'Thiên Đạo Sử',
	faction: 'Thế lực truyền tin'
};

export function planFx(
	e: FxEvent,
	q: Quality,
	rarityOf: (itemId: string) => string | undefined = () => undefined,
	reducedMotion = false
): FxCommand[] {
	switch (e.type) {
		// the mode always opens (even off: a static card) — information never depends on an animation
		case 'breakthroughStart':
			return [{ kind: 'mode', mode: 'breakthrough', step: 'gather', gather: planGather(e, q), tribulation: e.tribulation }];
		case 'breakthrough': {
			const { type: _t, ...result } = e;
			return [{ kind: 'mode', mode: 'breakthrough', step: 'outcome', plan: planBreakthrough(e, q, reducedMotion), result }];
		}
		case 'breakthroughStalled':
			return [{ kind: 'mode', mode: 'breakthrough', step: 'stall', message: e.message }];
		case 'breakthroughCancelled':
			return [{ kind: 'mode', mode: 'breakthrough', step: 'cancel' }];
		case 'settled':
		case 'insight': {
			const out: FxCommand[] = [];
			if (e.type === 'insight' && q !== 'off') out.push({ kind: 'ring', at: 'primary', tone: 'gold' });
			if (!(e.earnedMilli > 0)) return out;
			const float = `+${formatPoints(e.earnedMilli)} tu vi`;
			if (q === 'off') return [{ kind: 'float', text: float, tone: 'jade' }];
			const from: FxFrom = e.type === 'insight' ? 'seat' : e.source === 'incense' ? 'pointer' : 'cultivator';
			out.push({ kind: 'orbs', from, count: motesForEarned(e.earnedMilli, q), ms: orbFlightMs(e.earnedMilli, q), float });
			return out;
		}
		case 'reward': {
			const lines = [...e.items, ...e.currencies];
			const n = rewardFlights(lines.length, q);
			const ms = q === 'lite' ? 520 : 760;
			return lines.slice(0, n).map((l, i) => {
				const rarity = l.itemId ? rarityOf(l.itemId) : undefined;
				return {
					kind: 'fly',
					to: l.itemId ? 'bag' : 'stones',
					itemId: l.itemId,
					currency: l.currency,
					quantity: l.quantity ?? l.amount ?? 0,
					rarity,
					rare: q === 'full' && isRare(rarity),
					delay: i * FLY_STAGGER_MS,
					ms
				} satisfies FxCommand;
			});
		}
		case 'announcement':
			return q === 'off' ? [] : [{ kind: 'scroll', id: e.id, title: SCROLL_TITLE[e.kind] ?? 'Thiên hạ truyền tin', text: e.text }];
		case 'worksLevelUp':
			return q === 'off' ? [] : [{ kind: 'firework', count: q === 'lite' ? 1 : 3 }];
		default:
			return [];
	}
}
