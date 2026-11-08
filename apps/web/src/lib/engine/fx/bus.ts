// FxBus: typed pub/sub between the data stores (emit) and effects/audio (listen). Synchronous, no buffering: an
// event nobody listens to is dropped. A throwing listener never breaks the emitter or the other listeners.
import type { FxEvent, FxEventMap, FxEventType } from './events';

type Handler<K extends FxEventType> = (e: { type: K } & FxEventMap[K]) => void;
type AnyHandler = (e: FxEvent) => void;

export class FxBus {
	#handlers = new Map<FxEventType, Set<Handler<never>>>();
	#any = new Set<AnyHandler>();

	/** Listen to one event type; returns the unsubscribe. */
	on<K extends FxEventType>(type: K, fn: Handler<K>): () => void {
		let set = this.#handlers.get(type);
		if (!set) this.#handlers.set(type, (set = new Set()));
		set.add(fn as Handler<never>);
		return () => void set.delete(fn as Handler<never>);
	}

	/** Listen to every event (dev logging, audio router). */
	onAny(fn: AnyHandler): () => void {
		this.#any.add(fn);
		return () => void this.#any.delete(fn);
	}

	emit<K extends FxEventType>(type: K, payload: FxEventMap[K]): void {
		const e = { type, ...payload } as { type: K } & FxEventMap[K];
		for (const fn of this.#handlers.get(type) ?? []) this.#call(() => (fn as Handler<K>)(e));
		for (const fn of this.#any) this.#call(() => fn(e as FxEvent));
	}

	clear() {
		this.#handlers.clear();
		this.#any.clear();
	}

	#call(f: () => void) {
		try {
			f();
		} catch (err) {
			console.error('[fx] listener failed', err);
		}
	}
}

/** The app-wide bus. */
export const fxBus = new FxBus();

if (import.meta.env?.DEV && typeof window !== 'undefined') {
	// Dev only: show what the stores emit (nothing consumes these visually before GĐ4).
	fxBus.onAny((e) => console.debug('[fx]', e.type, e));
}
