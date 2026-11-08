// Full-screen modes (spec §3.3 "Chế độ", §4.9): the pure state machine shared by the mini-game modes. A mode hides the
// HUD/action bar/chat/windows and runs: lobby (rules + start) → countdown 3-2-1 → play → waiting (the server judges)
// → result, with pause, a confirmed exit and a soft error (timeout / network) that can retry. Esc and Back go through
// one rule (`back`): pause first, then ask to confirm the exit; a confirmed exit during play GIVES UP (the board ends
// and its log is sent like any end — the server decides), anywhere else it leaves the mode.
// Presentation only: no game rule, timing of the simulation or input log lives here.
import type { Quality } from '../fx/particles';

export type ModePhase = 'lobby' | 'countdown' | 'play' | 'waiting' | 'result' | 'error';

export interface ModeState {
	phase: ModePhase;
	/** countdown / play are frozen (the board's ticks stop; Thiên đạo's clock does not) */
	paused: boolean;
	/** the "Rời đi?" confirmation is open */
	confirm: boolean;
	/** ms left in the 3-2-1 countdown */
	countdownLeft: number;
	/** ms the "Bắt đầu" banner still shows after the countdown */
	goLeft: number;
}

export type ModeEvent =
	| { type: 'start'; countdownMs: number }
	| { type: 'tick'; dt: number }
	| { type: 'pause' }
	| { type: 'resume' }
	/** Esc / browser Back / the mode's exit button */
	| { type: 'back' }
	| { type: 'confirmExit' }
	| { type: 'cancelExit' }
	/** the board finished; `local` = practice (nothing is sent, the result is local) */
	| { type: 'ended'; local: boolean }
	/** the server's verdict arrived */
	| { type: 'verdict' }
	/** no answer in time / network error: soft error with retry */
	| { type: 'fail' }
	| { type: 'retry' }
	/** back to the lobby (Chơi lại → the rules screen, or a refused submission) */
	| { type: 'reset' };

/** What the host must do after a transition. */
export type ModeEffect = 'exit' | 'giveUp' | null;

/** How long "Bắt đầu" stays up after the countdown (ms). */
export const GO_MS = 700;
/** Server answer timeout for a submission / breakthrough (ms); after it the mode shows a soft error with retry. */
export const RESPONSE_TIMEOUT_MS = 8000;

/** 3-2-1 for full and lite; effects off → no count, an instant "Bắt đầu". */
export const countdownMs = (q: Quality) => (q === 'off' ? 0 : 3000);

export const initialMode = (): ModeState => ({ phase: 'lobby', paused: false, confirm: false, countdownLeft: 0, goLeft: 0 });

const playing = (s: ModeState) => s.phase === 'countdown' || s.phase === 'play';

export function reduceMode(s: ModeState, e: ModeEvent): { state: ModeState; effect: ModeEffect } {
	const next = (patch: Partial<ModeState>, effect: ModeEffect = null) => ({ state: { ...s, ...patch }, effect });
	const same = { state: s, effect: null };
	switch (e.type) {
		case 'start': {
			const ms = Math.max(0, e.countdownMs);
			return ms > 0
				? next({ phase: 'countdown', paused: false, confirm: false, countdownLeft: ms, goLeft: 0 })
				: next({ phase: 'play', paused: false, confirm: false, countdownLeft: 0, goLeft: GO_MS });
		}
		case 'tick': {
			if (s.paused || s.confirm || !(e.dt > 0)) return same;
			if (s.phase === 'countdown') {
				const left = s.countdownLeft - e.dt;
				return left > 0 ? next({ countdownLeft: left }) : next({ phase: 'play', countdownLeft: 0, goLeft: GO_MS });
			}
			if (s.phase === 'play' && s.goLeft > 0) return next({ goLeft: Math.max(0, s.goLeft - e.dt) });
			return same;
		}
		case 'pause':
			return playing(s) && !s.paused ? next({ paused: true }) : same;
		case 'resume':
			return playing(s) && s.paused ? next({ paused: false, confirm: false }) : same;
		case 'back':
			if (s.phase === 'lobby' || s.phase === 'result') return next({}, 'exit');
			if (s.phase === 'waiting') return same; // the answer is on its way: nothing to leave yet
			if (s.confirm) return next({ confirm: false }); // second Back/Esc closes the question
			if (s.phase === 'error') return next({ confirm: true });
			return s.paused ? next({ confirm: true }) : next({ paused: true });
		case 'confirmExit':
			if (!s.confirm) return same;
			if (playing(s)) return next({ confirm: false, paused: false }, 'giveUp');
			return next({ confirm: false }, 'exit');
		case 'cancelExit':
			return s.confirm ? next({ confirm: false }) : same;
		case 'ended':
			return playing(s) ? next({ phase: e.local ? 'result' : 'waiting', paused: false, confirm: false, goLeft: 0 }) : same;
		case 'verdict':
			return s.phase === 'waiting' || s.phase === 'error' ? next({ phase: 'result', confirm: false }) : same;
		case 'fail':
			return s.phase === 'waiting' ? next({ phase: 'error' }) : same;
		case 'retry':
			return s.phase === 'error' ? next({ phase: 'waiting', confirm: false }) : same;
		case 'reset':
			return { state: initialMode(), effect: null };
	}
}

/** The countdown text: "3" / "2" / "1", then "Bắt đầu" briefly; null otherwise. */
export function countdownLabel(s: ModeState): string | null {
	if (s.phase === 'countdown') return String(Math.max(1, Math.ceil(s.countdownLeft / 1000)));
	if (s.phase === 'play' && s.goLeft > 0) return 'Bắt đầu';
	return null;
}

/** The board may tick and take input only while playing, not paused and not asking to confirm. */
export const boardLive = (s: ModeState) => s.phase === 'play' && !s.paused && !s.confirm;
