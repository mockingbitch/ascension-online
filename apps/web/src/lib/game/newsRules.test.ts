import { describe, expect, it } from 'vitest';
import { planFx } from '$lib/engine/fx/plan';
import { sfxFor } from '$lib/engine/audio/sfxMap';
import { BANNER_GAP_MS, BANNER_MAX_AGE_MS, chatLines, enqueue, nextBanner, prominence, wantsBanner, type Queued } from './newsRules';
import type { ChatMessage, WorldAnnouncement } from './types';

const a = (o: Partial<WorldAnnouncement>): WorldAnnouncement => ({
	id: 1,
	kind: 'realm',
	tier: 2,
	characterName: 'Lâm Thanh Vân',
	tribulation: false,
	text: 'Lâm Thanh Vân bước vào Trúc Cơ.',
	createdAt: '2026-11-09T05:00:00Z',
	characterId: 'c-other',
	...o
});

describe('world news prominence by tier (decisions 2026-11-09 #8)', () => {
	it('tier 2 → ticker; 3–4 → banner; 5–6 → major; first-to-realm and rank 1 → major; top-10 and digest → banner', () => {
		expect(prominence(a({ tier: 2 }))).toBe('ticker');
		expect(prominence(a({ tier: 3 }))).toBe('banner');
		expect(prominence(a({ kind: 'tribulation', tier: 4 }))).toBe('banner');
		expect(prominence(a({ kind: 'tribulation', tier: 5 }))).toBe('major');
		expect(prominence(a({ kind: 'tribulation', tier: 6 }))).toBe('major');
		expect(prominence(a({ kind: 'first_to_realm', tier: 2 }))).toBe('major');
		expect(prominence(a({ kind: 'rank_top10', tier: 0, rank: 1 }))).toBe('major');
		expect(prominence(a({ kind: 'rank_top10', tier: 0, rank: 7 }))).toBe('banner');
		expect(prominence(a({ kind: 'digest', tier: 4, count: 5 }))).toBe('banner');
	});
	it('never a banner for my own news, ticker-level news, or when other players\' banners are off', () => {
		expect(wantsBanner(a({ tier: 5, kind: 'tribulation' }), 'c-other', true)).toBeNull();
		expect(wantsBanner(a({ tier: 2 }), 'me', true)).toBeNull();
		expect(wantsBanner(a({ tier: 6, kind: 'tribulation' }), 'me', false)).toBeNull();
		expect(wantsBanner(a({ tier: 6, kind: 'tribulation' }), 'me', true)).toBe('major');
	});
});

describe('banner pacing on the client', () => {
	const q = (id: number, p: 'banner' | 'major', at: number): Queued => ({ item: a({ id }), prominence: p, at });
	it('keeps at most 3 queued, majors first, no duplicates', () => {
		let x: Queued[] = [];
		for (const [id, p] of [[1, 'banner'], [2, 'banner'], [3, 'major'], [4, 'banner'], [3, 'major']] as const) x = enqueue(x, q(id, p, id));
		expect(x.map((y) => y.item.id)).toEqual([3, 1, 2]);
	});
	it('waits for the gap after the last banner of that kind and drops news older than 30 s (ticker/list keep it)', () => {
		const last = { banner: 1000, major: -Infinity };
		expect(nextBanner([q(1, 'banner', 1000)], 2000, last)).toMatchObject({ show: null, waitMs: BANNER_GAP_MS.banner - 1000 });
		expect(nextBanner([q(1, 'banner', 1000)], 4000, last).show?.item.id).toBe(1);
		expect(nextBanner([q(1, 'major', 0)], BANNER_MAX_AGE_MS + 1, last)).toMatchObject({ show: null, rest: [] });
	});
});

describe('world chat system lines (client-side, never stored as chat)', () => {
	it('interleaves messages and news by time', () => {
		const m = (id: number, at: string) => ({ id, channel: 'world', characterId: 'x', name: 'A', stageId: 's', text: 'hi', sentAt: at }) as ChatMessage;
		const lines = chatLines([m(1, '2026-11-09T05:00:00Z'), m(2, '2026-11-09T05:02:00Z')], [a({ id: 9, createdAt: '2026-11-09T05:01:00Z' })]);
		expect(lines.map((l) => l.key)).toEqual(['m1', 'a9', 'm2']);
	});
});

describe('other players\' breakthrough news never opens the mid-screen scroll; sound only when its banner asks', () => {
	it('plan + sfx', () => {
		const e = { type: 'announcement', id: 'x', text: 't', kind: 'breakthrough' } as const;
		expect(planFx(e, 'full')).toEqual([]);
		expect(planFx({ type: 'announcement', id: 'y', text: 't', kind: 'milestone' }, 'full')[0]).toMatchObject({ kind: 'scroll' });
		const ctx = { quality: 'full' as const, reducedMotion: false, rarityOf: () => undefined, selfId: null };
		expect(sfxFor(e, ctx)).toEqual([]);
		expect(sfxFor({ ...e, sound: true }, ctx).map((c) => c.id)).toEqual(['sfx.announcement']);
	});
});
