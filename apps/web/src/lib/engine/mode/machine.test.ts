import { describe, expect, it } from 'vitest';
import { boardLive, countdownLabel, countdownMs, GO_MS, initialMode, reduceMode, type ModeEvent, type ModeState } from './machine';

/** Apply events in order; returns the final state and every effect emitted. */
function run(events: ModeEvent[], from: ModeState = initialMode()) {
	let s = from;
	const effects: string[] = [];
	for (const e of events) {
		const r = reduceMode(s, e);
		s = r.state;
		if (r.effect) effects.push(r.effect);
	}
	return { s, effects };
}

describe('mode machine: countdown', () => {
	it('counts 3-2-1, then shows "Bắt đầu" briefly while play has started', () => {
		let { s } = run([{ type: 'start', countdownMs: 3000 }]);
		expect(s.phase).toBe('countdown');
		expect(countdownLabel(s)).toBe('3');
		expect(boardLive(s)).toBe(false);
		s = run([{ type: 'tick', dt: 1100 }], s).s;
		expect(countdownLabel(s)).toBe('2');
		s = run([{ type: 'tick', dt: 1000 }], s).s;
		expect(countdownLabel(s)).toBe('1');
		s = run([{ type: 'tick', dt: 1000 }], s).s;
		expect(s.phase).toBe('play');
		expect(countdownLabel(s)).toBe('Bắt đầu');
		expect(boardLive(s)).toBe(true);
		s = run([{ type: 'tick', dt: GO_MS }], s).s;
		expect(countdownLabel(s)).toBeNull();
	});
	it('effects off: no count, an instant "Bắt đầu" and the board is live at once', () => {
		expect(countdownMs('off')).toBe(0);
		expect(countdownMs('lite')).toBe(3000);
		const { s } = run([{ type: 'start', countdownMs: countdownMs('off') }]);
		expect(s.phase).toBe('play');
		expect(countdownLabel(s)).toBe('Bắt đầu');
		expect(boardLive(s)).toBe(true);
	});
	it('a paused countdown does not run', () => {
		const { s } = run([{ type: 'start', countdownMs: 3000 }, { type: 'pause' }, { type: 'tick', dt: 5000 }]);
		expect(s.phase).toBe('countdown');
		expect(s.countdownLeft).toBe(3000);
		const r = run([{ type: 'resume' }, { type: 'tick', dt: 3000 }], s).s;
		expect(r.phase).toBe('play');
	});
});

describe('mode machine: pause, Back order, exit confirmation', () => {
	const playing = run([{ type: 'start', countdownMs: 0 }]).s;

	it('Back during play: 1st pauses, 2nd asks to confirm, 3rd closes the question (still paused)', () => {
		let s = run([{ type: 'back' }], playing).s;
		expect(s).toMatchObject({ paused: true, confirm: false });
		expect(boardLive(s)).toBe(false);
		s = run([{ type: 'back' }], s).s;
		expect(s).toMatchObject({ paused: true, confirm: true });
		const r = run([{ type: 'back' }], s);
		expect(r.s).toMatchObject({ phase: 'play', paused: true, confirm: false });
		expect(r.effects).toEqual([]);
	});
	it('confirming the exit during play gives up (the board ends and is judged), never leaves directly', () => {
		const r = run([{ type: 'back' }, { type: 'back' }, { type: 'confirmExit' }], playing);
		expect(r.effects).toEqual(['giveUp']);
		expect(r.s.phase).toBe('play');
		const ended = run([{ type: 'ended', local: false }], r.s).s;
		expect(ended.phase).toBe('waiting');
	});
	it('Back during the countdown pauses it too', () => {
		const r = run([{ type: 'start', countdownMs: 3000 }, { type: 'back' }, { type: 'back' }, { type: 'confirmExit' }]);
		expect(r.effects).toEqual(['giveUp']);
	});
	it('cancel keeps the game paused; resume continues', () => {
		const s = run([{ type: 'back' }, { type: 'back' }, { type: 'cancelExit' }], playing).s;
		expect(s).toMatchObject({ paused: true, confirm: false });
		expect(boardLive(run([{ type: 'resume' }], s).s)).toBe(true);
	});
	it('the lobby and the result screen leave at once; waiting ignores Back', () => {
		expect(run([{ type: 'back' }]).effects).toEqual(['exit']);
		const waiting = run([{ type: 'ended', local: false }], playing).s;
		expect(run([{ type: 'back' }], waiting)).toMatchObject({ s: waiting, effects: [] });
		const result = run([{ type: 'verdict' }], waiting).s;
		expect(result.phase).toBe('result');
		expect(run([{ type: 'back' }], result).effects).toEqual(['exit']);
	});
	it('practice ends straight on the result (nothing is sent)', () => {
		expect(run([{ type: 'ended', local: true }], playing).s.phase).toBe('result');
	});
});

describe('mode machine: the result only after the server answers', () => {
	const waiting = run([{ type: 'start', countdownMs: 0 }, { type: 'ended', local: false }]).s;
	it('timeout → soft error → retry → waiting → verdict', () => {
		let s = run([{ type: 'fail' }], waiting).s;
		expect(s.phase).toBe('error');
		s = run([{ type: 'retry' }], s).s;
		expect(s.phase).toBe('waiting');
		s = run([{ type: 'verdict' }], s).s;
		expect(s.phase).toBe('result');
	});
	it('a late answer still lands while the error shows', () => {
		const s = run([{ type: 'fail' }, { type: 'verdict' }], waiting).s;
		expect(s.phase).toBe('result');
	});
	it('leaving from the error asks first (the unsent result would be lost)', () => {
		const r = run([{ type: 'fail' }, { type: 'back' }], waiting);
		expect(r.s.confirm).toBe(true);
		expect(run([{ type: 'confirmExit' }], r.s).effects).toEqual(['exit']);
	});
	it('a verdict never shows before the game ended', () => {
		const playing = run([{ type: 'start', countdownMs: 0 }]).s;
		expect(run([{ type: 'verdict' }], playing).s.phase).toBe('play');
		expect(run([{ type: 'verdict' }]).s.phase).toBe('lobby');
	});
	it('reset returns to the lobby', () => {
		expect(run([{ type: 'verdict' }, { type: 'reset' }], waiting).s).toEqual(initialMode());
	});
});
