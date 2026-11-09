// Vân Khê's tutorial store (decisions 2026-11-09 #1, GĐ-B). Display only: a step never grants anything and never
// calls a gameplay API. Triggers come from server facts — fxBus events the stores emit after a server answer, the
// server's breakthrough preview — or places the player opened (a window, a scene, a mini game lobby); see
// components/TutorialHost.svelte. Progress lives on the server (GET /tutorial, POST /tutorial/steps/{id}/done|skip)
// so it carries across devices; every write sends an Idempotency-Key and the answer replaces the local state.
import { untrack } from 'svelte';
import { api, ApiError, newIdempotencyKey } from '$lib/api/client';
import { help } from './help';
import { helpLines, stepFor } from './tutorialRules';
import type { HelpTopic } from './windowStack';
import type { TutorialScript, TutorialState, TutorialStep } from './types';

export interface ActiveGuide {
	step: TutorialStep;
	/** line index */
	i: number;
	/** opened by the player ("?"): not a stored step, focus moves to the panel */
	summoned: boolean;
}

class TutorialStore {
	script = $state.raw<TutorialScript | null>(null);
	state = $state.raw<TutorialState | null>(null);
	active = $state<ActiveGuide | null>(null);
	/** a full-screen moment (breakthrough celebration) is on: queued steps wait for it */
	blocked = $state(false);
	busy = $state(false);
	#queue: TutorialStep[] = [];
	/** steps shown in this page life (no repeat before the server's answer lands) */
	#seen = new Set<string>();
	#keys = new Map<string, string>();
	#scriptLoad: Promise<void> | null = null;
	/** triggers that happened before the script/progress arrived (e.g. a deep-linked window): replayed after load */
	#early: string[] = [];

	/** The public script (also used by /create before a character exists). Loads once per page life. */
	loadScript(): Promise<void> {
		this.#scriptLoad ??= api
			.tutorialScript()
			.then((s) => void (this.script = s))
			.catch(() => void (this.#scriptLoad = null));
		return this.#scriptLoad;
	}

	#load: Promise<void> | null = null;
	/** Script + the character's progress (the /play shell, once a character exists). Once per page life. */
	load(): Promise<void> {
		this.#load ??= this.#doLoad();
		return this.#load;
	}

	async #doLoad(): Promise<void> {
		await this.loadScript();
		try {
			this.state = await api.tutorial();
		} catch {
			/* no progress → no automatic step this session (the "?" summon still works) */
		}
		const early = this.#early;
		this.#early = [];
		for (const t of early) this.#trigger(t);
		untrack(() => this.#pump());
	}

	/** A trigger happened (see TutorialHost). Shows its step once, if not seen anywhere and not muted. */
	trigger(t: string) {
		// called from effects: never subscribe the caller to the tutorial state
		untrack(() => this.#trigger(t));
	}

	#trigger(t: string) {
		if (!this.script || !this.state) {
			if (!this.#early.includes(t)) this.#early.push(t);
			return;
		}
		const s = stepFor(this.script, t, this.state, this.#seen);
		if (!s || this.#queue.some((q) => q.id === s.id) || this.active?.step.id === s.id) return;
		this.#queue.push(s);
		this.#pump();
	}

	/** "?" — the player asks Vân Khê about a screen: she reads its help points (not stored). */
	summon(topic: HelpTopic) {
		const h = help[topic];
		this.active = { step: { id: `help:${topic}`, flow: 'play', trigger: 'help', title: h.title, lines: helpLines(h.points) }, i: 0, summoned: true };
	}

	setBlocked(b: boolean) {
		untrack(() => {
			this.blocked = b;
			if (!b) this.#pump();
		});
	}

	#pump() {
		if (this.active || this.blocked || !this.#queue.length) return;
		const s = this.#queue.shift()!;
		this.#seen.add(s.id);
		this.active = { step: s, i: 0, summoned: false };
	}

	next() {
		const a = this.active;
		if (!a) return;
		if (a.i + 1 < a.step.lines.length) this.active = { ...a, i: a.i + 1 };
		else this.#close('done');
	}

	prev() {
		const a = this.active;
		if (a && a.i > 0) this.active = { ...a, i: a.i - 1 };
	}

	/** Dismiss the step on screen (stored as skipped). */
	skip() {
		this.#close('skip');
	}

	#close(how: 'done' | 'skip') {
		const a = this.active;
		this.active = null;
		if (a && !a.summoned && a.step.flow === 'play') void this.#mark(a.step.id, how);
		this.#pump();
	}

	async #mark(id: string, how: 'done' | 'skip') {
		const key = this.#keys.get(id) ?? newIdempotencyKey();
		this.#keys.set(id, key);
		try {
			const r = await api.tutorialMark(id, how, key);
			this.state = r.state;
			this.#keys.delete(id);
		} catch (e) {
			// 4xx: the server answered and wrote nothing new — drop the key; 5xx/network: it stays for the next close
			if (e instanceof ApiError && e.status < 500) this.#keys.delete(id);
		}
	}

	/** "Không nhắc lại": no step on her own any more (the "?" summon still works). */
	async muteAll() {
		this.active = null;
		this.#queue = [];
		this.busy = true;
		try {
			this.state = await api.tutorialSkipAll(newIdempotencyKey());
		} catch {
			/* the next load shows the server's flag */
		} finally {
			this.busy = false;
		}
	}

	/** Cài đặt › "Xem lại hướng dẫn": forget seen steps and un-mute. */
	async reset() {
		this.busy = true;
		try {
			this.state = await api.tutorialReset(newIdempotencyKey());
			this.#seen.clear();
		} finally {
			this.busy = false;
		}
	}
}

export const tutorial = new TutorialStore();
