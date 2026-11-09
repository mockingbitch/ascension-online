// fxBus event → sound cues (spec §4.10). Pure: the event (server values, as emitted) + presentation context → which
// sound ids to play, on which channel, and when (ms after the event). Nothing here decides an outcome: the
// breakthrough branch and the strike times come from the same timeline the visuals play (fx/breakthrough.ts), so the
// thunder lands on each strike and the result sound on the reveal. Unit-tested in sfxMap.test.ts.
import type { Rarity } from '$lib/game/types';
import { planBreakthrough } from '../fx/breakthrough';
import type { FxEvent } from '../fx/events';
import type { Quality } from '../fx/particles';
import type { ShotChannel } from './AudioManager';

export interface SfxCue {
	id: string;
	ch: ShotChannel;
	/** ms after the event */
	at: number;
}

/** UI input that is not a game event: a button press (delegated listener in the router). */
export type SfxInput = FxEvent | { type: 'press' };

export interface SfxContext {
	quality: Quality;
	reducedMotion: boolean;
	/** rarity of a catalogue item (undefined = unknown → treated as phàm) */
	rarityOf: (itemId: string) => Rarity | undefined;
	/** my character id: my own chat lines make no sound */
	selfId: string | null;
	/** effect tier of a stage's realm (realms.yaml visual.fx_tier) — picks the breakthrough cue of the tier */
	tierOf?: (stageId: string) => number | undefined;
}

export const RARITY_ORDER: Rarity[] = ['pham', 'linh', 'huyen', 'dia', 'thien', 'tien'];

const cue = (id: string, at = 0, ch: ShotChannel = 'sfx'): SfxCue => ({ id, ch, at });

/** The rarest of the rewarded items (currencies alone count as phàm); null = nothing was given. */
export function topRarity(items: { itemId?: string }[], currencies: unknown[], rarityOf: SfxContext['rarityOf']): Rarity | null {
	let best = -1;
	for (const it of items) best = Math.max(best, RARITY_ORDER.indexOf((it.itemId && rarityOf(it.itemId)) || 'pham'));
	if (best < 0 && currencies.length > 0) best = 0;
	return best < 0 ? null : RARITY_ORDER[best]!;
}

export function sfxFor(e: SfxInput, c: SfxContext): SfxCue[] {
	switch (e.type) {
		case 'press':
			return [cue('ui.click', 0, 'ui')];
		case 'window':
			return [cue(e.open ? 'ui.window_open' : 'ui.window_close', 0, 'ui')];
		case 'countdown':
			return [cue(e.n > 0 ? 'ui.countdown' : 'ui.countdown_go', 0, 'ui')];
		case 'chat':
			return e.message.characterId === c.selfId ? [] : [cue('sfx.chat', 0, 'ui')];
		case 'settled':
			return e.earnedMilli > 0 ? [cue('sfx.settle')] : [];
		case 'insight':
			return [cue('sfx.insight')];
		case 'stageUp':
		case 'worksLevelUp':
			return [cue('sfx.stage_up')];
		case 'reward': {
			const r = topRarity(e.items, e.currencies, c.rarityOf);
			return r ? [cue(`sfx.item_${r}`)] : [];
		}
		case 'announcement':
			// other players' breakthrough news sound only when its large banner asked (settings.others.sound)
			return e.kind === 'breakthrough' && !e.sound ? [] : [cue('sfx.announcement')];
		case 'trialResult':
			return [cue(e.verdict === 'pass' ? 'sfx.stage_up' : 'sfx.fail')];
		case 'fortune':
			return e.lucky ? [cue('sfx.item_linh')] : [];
		case 'breakthrough': {
			const plan = planBreakthrough({ ...e, fxTier: c.tierOf?.(e.to) }, c.quality, c.reducedMotion);
			const strikes = plan.steps.filter((s) => s.beat === 'strike');
			const out = strikes.map((s) => cue('sfx.thunder', s.at));
			const after = strikes.at(-1)?.at ?? -1;
			const success = e.result === 'success';
			// the reveal: the first beat after the strikes that shows the outcome (clouds clearing / pillar, or the crack),
			// else the static result. Success sounds grow with the tier: bell → khánh → cổ cầm → choir (sfx.bt_tier2..6)
			const reveal = plan.steps.find((s) => s.at > after && (success ? s.beat === 'clear' || s.beat === 'pillar' : s.beat === 'crack'));
			const at = reveal?.at ?? plan.totalMs;
			out.push(cue(success ? (plan.tier >= 2 ? `sfx.bt_tier${plan.tier}` : 'sfx.breakthrough') : 'sfx.fail', at));
			return out;
		}
		// the breakthrough start/stall/cancel, purchases (a reward follows), equips, item use and milestones (an
		// announcement carries them) stay silent
		default:
			return [];
	}
}

/** After a breakthrough, the stageUp the store also emits is part of the same moment: no second sound. */
export const STAGE_UP_QUIET_MS = 4000;
