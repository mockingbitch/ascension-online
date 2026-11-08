import { describe, expect, it } from 'vitest';
import { addBubbles, BUBBLE_MAX, BUBBLE_TTL_MS, expireBubbles, nextExpiry, type BubbleMsg } from './chatBubbles';

const msg = (k: number, mine = false): BubbleMsg => ({ key: `world:${k}`, name: `N${k}`, text: `t${k}`, stageId: 's', mine });

describe('floating chat bubbles', () => {
	it('keeps only the newest three and never duplicates', () => {
		let b = addBubbles([], [msg(1), msg(2)], 0);
		b = addBubbles(b, [msg(2), msg(3), msg(4)], 1000);
		expect(b.map((x) => x.key)).toEqual(['world:2', 'world:3', 'world:4']);
		expect(b.length).toBe(BUBBLE_MAX);
	});

	it('each bubble fades 8 s after it arrived', () => {
		let b = addBubbles([], [msg(1)], 0);
		b = addBubbles(b, [msg(2, true)], 3000);
		expect(nextExpiry(b, 3000)).toBe(BUBBLE_TTL_MS - 3000);
		b = expireBubbles(b, BUBBLE_TTL_MS);
		expect(b.map((x) => x.key)).toEqual(['world:2']);
		expect(b[0]!.mine).toBe(true);
		expect(expireBubbles(b, 3000 + BUBBLE_TTL_MS)).toEqual([]);
		expect(nextExpiry([], 0)).toBeNull();
	});
});
