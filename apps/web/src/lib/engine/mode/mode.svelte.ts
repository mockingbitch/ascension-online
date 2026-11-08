// Runes wrapper around the pure mode machine (machine.ts) for one mounted mode: holds the state, runs the countdown /
// "Bắt đầu" banner on the shared loop (only while one is showing) and hands effects (exit / give up) to the host.
import { gameLoop } from '../loop';
import { initialMode, reduceMode, type ModeEvent, type ModeState } from './machine';

export interface ModeHost {
	/** leave the mode (back to the scene) */
	exit: () => void;
	/** confirmed exit during play: end the board now (its log is judged like any end) */
	giveUp: () => void;
}

export class ModeController {
	state = $state<ModeState>(initialMode());
	#host: ModeHost;
	#unsub: (() => void) | null = null;

	constructor(host: ModeHost) {
		this.#host = host;
	}

	get phase() {
		return this.state.phase;
	}
	get paused() {
		return this.state.paused;
	}

	send(e: ModeEvent) {
		const r = reduceMode(this.state, e);
		this.state = r.state;
		this.#syncTimer();
		if (r.effect === 'exit') this.#host.exit();
		else if (r.effect === 'giveUp') this.#host.giveUp();
	}

	/** Stop the timer (the mode unmounted). */
	dispose() {
		this.#unsub?.();
		this.#unsub = null;
	}

	#syncTimer() {
		const s = this.state;
		const wants = (s.phase === 'countdown' || (s.phase === 'play' && s.goLeft > 0)) && !s.paused && !s.confirm;
		if (wants && !this.#unsub) this.#unsub = gameLoop().subscribe((dt) => this.send({ type: 'tick', dt }));
		else if (!wants) this.dispose();
	}
}
