// Semantic game events for effects and sound (spec §3.1). Emitted by the domain stores AFTER a server response, with
// the values the server returned — listeners only present them and never call the API or derive rewards.
import type { BreakthroughKind, ChatMessage } from '$lib/game/types';

/** A reward line exactly as the server reported it. */
export interface RewardLine {
	itemId?: string;
	quantity?: number;
	currency?: string;
	amount?: number;
}

export type RewardSource = 'task' | 'vip_gift' | 'incense' | 'trial' | 'shop' | 'trade' | 'other';

export interface FxEventMap {
	/** settle / speed item / incense: tu vi the server credited (milli) */
	settled: {
		earnedMilli: number;
		source: 'settle' | 'speed_item' | 'incense';
		stagesAdvanced: string[];
		/** the server clipped this settle at today's ceiling (HUD bar shakes) */
		hitDailyCap?: boolean;
	};
	reward: { items: RewardLine[]; currencies: RewardLine[]; source: RewardSource; vipPoints?: number };
	/** a world announcement / story milestone arrived over the socket (text as the server sent it) */
	announcement: {
		id: string;
		text: string;
		kind: 'breakthrough' | 'milestone' | 'event' | 'faction';
		/** other players' news (kind breakthrough): its large banner asked for the cue (Cài đặt › thông báo) */
		sound?: boolean;
	};
	/** the attempt was sent (the mode opens and gathers qi until the answer); kind/tribulation from the server preview */
	breakthroughStart: { kind: BreakthroughKind; tribulation: boolean };
	/** the attempt's answer (server values): outcome, stages and the tu vi lost (0 = intact) */
	breakthrough: {
		result: 'success' | 'failure';
		kind: BreakthroughKind;
		tribulation: boolean;
		lossMilli: number;
		from: string;
		to: string;
		failStreak: number;
	};
	/** no answer in time / network error: the mode shows a soft error with retry (the request may still land) */
	breakthroughStalled: { message: string };
	/** the server refused the attempt (not ready, quote changed): the mode closes, the window explains */
	breakthroughCancelled: { reason?: string };
	stageUp: { from: string; to: string };
	insight: { earnedMilli: number; takeaway: string };
	milestone: { id: string };
	chat: { message: ChatMessage };
	trialResult: { verdict: 'pass' | 'fail' };
	purchase: { itemId: string };
	equip: { slot: string; id: string | null };
	/** an item was used (e.g. Tụ Linh Phù) — no reward attached */
	itemUsed: { itemId: string };
	/** Gieo quẻ: the server drew today's outcome */
	fortune: { outcomeId: string; name: string; lucky: boolean };
	/** faction works: a donation finished one or more build stages (server progress) */
	worksLevelUp: { factionId: string; unlocked: number };
	/** UI (sound only): a game window opened / closed over the scene (WindowHost) */
	window: { open: boolean; kind: string };
	/** UI (sound only): a mode's 3-2-1 countdown ticked (n = 3, 2, 1; 0 = "Bắt đầu") */
	countdown: { n: number };
}

export type FxEventType = keyof FxEventMap;
export type FxEvent = { [K in FxEventType]: { type: K } & FxEventMap[K] }[FxEventType];
