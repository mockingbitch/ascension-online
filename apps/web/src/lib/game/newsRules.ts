// World news presentation (decisions 2026-11-09 #8 #12, GĐ-D) — pure, unit-tested: how loud an announcement is, the
// banner pacing on the client, and the client-side system lines of the world chat. Display only.
import type { ChatMessage, WorldAnnouncement } from './types';

/** ticker = the HUD strip only · banner = a sliding banner · major = the large banner with sound. */
export type Prominence = 'ticker' | 'banner' | 'major';

/** Tier 2 → ticker; 3–4, top-10 and digests → banner; 5–6, first-to-realm and rank 1 → major. */
export function prominence(a: Pick<WorldAnnouncement, 'kind' | 'tier' | 'rank'>): Prominence {
	switch (a.kind) {
		case 'first_to_realm':
			return 'major';
		case 'rank_top10':
			return a.rank === 1 ? 'major' : 'banner';
		case 'digest':
			return 'banner';
		default:
			return a.tier >= 5 ? 'major' : a.tier >= 3 ? 'banner' : 'ticker';
	}
}

export const BANNER_MS: Record<Exclude<Prominence, 'ticker'>, number> = { banner: 4000, major: 5500 };
/** minimum gap between two banners of a kind, and how old news may be to still get a banner */
export const BANNER_GAP_MS: Record<Exclude<Prominence, 'ticker'>, number> = { banner: 3000, major: 6000 };
export const BANNER_MAX_AGE_MS = 30_000;
export const BANNER_QUEUE = 3;

export interface Queued {
	item: WorldAnnouncement;
	prominence: Exclude<Prominence, 'ticker'>;
	/** when it arrived (client clock, display pacing only) */
	at: number;
}

/**
 * Whether a live announcement should become a banner: not my own news (my celebration already played), not when the
 * player turned other players' banners off, not a ticker-level one. Pure.
 */
export function wantsBanner(a: WorldAnnouncement, selfId: string | null, bannersOn: boolean): Exclude<Prominence, 'ticker'> | null {
	const p = prominence(a);
	if (p === 'ticker' || !bannersOn || (selfId && a.characterId === selfId)) return null;
	return p;
}

/** Add to the banner queue: keeps the newest BANNER_QUEUE, majors first. Pure. */
export function enqueue(q: readonly Queued[], x: Queued): Queued[] {
	if (q.some((y) => y.item.id === x.item.id)) return [...q];
	const next = [...q, x].sort((a, b) => (a.prominence === b.prominence ? a.at - b.at : a.prominence === 'major' ? -1 : 1));
	return next.slice(0, BANNER_QUEUE);
}

/**
 * Next banner to show now, or null: drops queued news older than BANNER_MAX_AGE_MS (it stays in the ticker / list)
 * and waits for the gap after the last banner of that prominence. Pure.
 */
export function nextBanner(q: readonly Queued[], now: number, lastShown: Record<'banner' | 'major', number>): { show: Queued | null; rest: Queued[]; waitMs: number } {
	const rest = q.filter((x) => now - x.at <= BANNER_MAX_AGE_MS);
	const head = rest[0];
	if (!head) return { show: null, rest, waitMs: 0 };
	const wait = lastShown[head.prominence] + BANNER_GAP_MS[head.prominence] - now;
	if (wait > 0) return { show: null, rest, waitMs: wait };
	return { show: head, rest: rest.slice(1), waitMs: 0 };
}

/** One line of the world chat: a player's message, or a client-side system line for a world announcement. */
export type ChatLine = { kind: 'msg'; key: string; at: number; msg: ChatMessage } | { kind: 'news'; key: string; at: number; news: WorldAnnouncement };

/** World chat + announcements in time order (announcements are shown, never stored as chat). Pure. */
export function chatLines(messages: readonly ChatMessage[], news: readonly WorldAnnouncement[], keep = 200): ChatLine[] {
	const lines: ChatLine[] = [
		...messages.map((m) => ({ kind: 'msg' as const, key: `m${m.id}`, at: Date.parse(m.sentAt) || 0, msg: m })),
		...news.map((n) => ({ kind: 'news' as const, key: `a${n.id}`, at: Date.parse(n.createdAt) || 0, news: n }))
	];
	return lines.sort((a, b) => a.at - b.at || (a.kind === b.kind ? 0 : a.kind === 'news' ? 1 : -1)).slice(-keep);
}
