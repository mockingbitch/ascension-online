// One mutation in flight at a time, with a soft timeout. A request that times out keeps running: the next `run()`
// (the player's "Thử lại") joins it instead of sending a second one, and `late()` lets the caller still apply its
// answer if it arrives after the timeout. Idempotency keys stay the caller's job (same key on retry).

export class TimeoutError extends Error {
	constructor() {
		super('timeout');
		this.name = 'TimeoutError';
	}
}

/** Resolve/reject like `p`, or reject with TimeoutError after `ms`. */
export function withTimeout<T>(p: Promise<T>, ms: number): Promise<T> {
	return new Promise<T>((resolve, reject) => {
		const t = setTimeout(() => reject(new TimeoutError()), ms);
		p.then(
			(v) => {
				clearTimeout(t);
				resolve(v);
			},
			(e) => {
				clearTimeout(t);
				reject(e);
			}
		);
	});
}

export class InFlight<T> {
	#p: Promise<T> | null = null;

	get pending() {
		return this.#p !== null;
	}

	/** Start `make()` unless a request is still in flight (then wait for that one), at most `ms`. */
	run(make: () => Promise<T>, ms: number): Promise<T> {
		let p = this.#p;
		if (!p) {
			p = make();
			this.#p = p;
			const done = () => {
				if (this.#p === p) this.#p = null;
			};
			p.then(done, done);
		}
		return withTimeout(p, ms);
	}

	/** After a timeout: call `fn` with the answer if the request still succeeds. */
	late(fn: (v: T) => void) {
		this.#p?.then(fn, () => {});
	}
}
