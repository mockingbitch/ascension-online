// The scene stage of the game shell (spec §3.1–3.3, §4.2). One instance per shell; it decides which scene the
// SceneLayer renders and runs scene transitions:
//   route change → leave(): [zoom toward the clicked hotspot] → cover (ink wipe / fade) → swap the scene
//   navigation done → arrive(): wait until the new scene is mounted → reveal.
// Routes stay the source of truth (scenes are routes, windows are shallow routing); the SceneManager mirrors them so
// Back semantics and "is the scene covered" have one owner. Presentation only.
import { untrack } from 'svelte';
import type { Quality } from '../fx/particles';
import type { SceneRenderer } from '../render/types';
import { SceneManager, type SceneWindow } from './manager.svelte';
import { planTransition, resolveScene, sceneOfPath, type TransitionKind, type TransitionPlan } from './routes';
import type { SceneDef, SceneId } from './types';

/** What draws the cover between two scenes (SceneWipe.svelte); resolves when the animation ends. */
export interface WipeDriver {
	cover(kind: TransitionKind, ms: number): Promise<void>;
	reveal(kind: TransitionKind, ms: number): Promise<void>;
}

export type StagePhase = 'idle' | 'zoom' | 'cover' | 'reveal';

/** Longest wait for a new scene to mount under the cover before revealing anyway (slow art, failed import). */
export const MOUNT_TIMEOUT_MS = 2500;

const sleep = (ms: number) => new Promise<void>((r) => setTimeout(r, ms));

export class SceneStage {
	readonly manager = new SceneManager();
	/** scene the SceneLayer renders (null until the first route is known) */
	shown = $state<SceneId | null>(null);
	phase = $state<StagePhase>('idle');
	/** a non-scene route (a mini-game mode) is drawn over the scene */
	screenOver = $state(false);
	/** local modal windows open outside the shallow-routed stack (confirmations, item detail, ngộ đạo reveal) */
	localWindows = $state(0);
	/** the page's SceneSlot (the box the scene fills), else the shell's stage cell */
	slot = $state<HTMLElement | null>(null);
	cell = $state<HTMLElement | null>(null);

	/** set by the SceneLayer once a scene is mounted */
	renderer: SceneRenderer | null = null;
	def: SceneDef | null = null;
	/** set by the SceneWipe */
	wipe: WipeDriver | null = null;

	#gen = 0;
	#plan: TransitionPlan | null = null;
	#mounted: SceneId | null = null;
	#waiters: { id: SceneId; done: () => void }[] = [];

	/** A window or a screen covers the scene: the renderer pauses parallax, ambient loops pause, the scene dims. */
	get covered() {
		return this.manager.windows.length > 0 || this.localWindows > 0 || this.screenOver;
	}

	/** The route is the truth: show its scene without a transition (first load, redirects, interrupted moves). */
	sync(path: string) {
		untrack(() => this.#sync(path));
	}

	#sync(path: string) {
		const s = resolveScene(path, this.shown);
		this.screenOver = !sceneOfPath(path);
		if (!s || s === this.shown) return;
		this.manager.go(s);
		this.#mounted = null;
		this.shown = s;
	}

	/** Mirror the shallow-routed window stack (page.state.wins) into the manager. */
	setWindows(ws: readonly SceneWindow[] | null | undefined) {
		// untracked: called from an effect on page.state; the manager reads its own stack while updating it
		untrack(() => this.manager.setStack(ws ?? []));
	}

	/**
	 * Before a navigation to `toPath` completes: zoom/cover and swap the scene. Returns a token for arrive(), or 0 when
	 * the move needs no transition (same scene, a screen over the current scene, leaving /play).
	 */
	async leave(toPath: string, quality: Quality): Promise<number> {
		const to = resolveScene(toPath, this.shown);
		const plan = planTransition(this.shown, to, quality, { fromDef: this.def, toPath });
		if (!plan || !to) return 0;
		const gen = ++this.#gen;
		this.#plan = plan;
		if (plan.zoom && this.renderer) {
			this.phase = 'zoom';
			await this.renderer.setCamera(plan.zoom, plan.zoomMs);
			if (gen !== this.#gen) return gen;
		}
		this.phase = 'cover';
		await this.wipe?.cover(plan.kind, plan.coverMs);
		if (gen !== this.#gen) return gen;
		this.manager.go(to);
		this.#mounted = null;
		this.shown = to;
		return gen;
	}

	/** After the navigation finished (or failed): reveal once the new scene is mounted. Stale tokens are ignored. */
	async arrive(token: number) {
		if (!token || token !== this.#gen || !this.#plan) return;
		const plan = this.#plan;
		const id = this.shown;
		if (id) await Promise.race([this.#whenMounted(id), sleep(MOUNT_TIMEOUT_MS)]);
		if (token !== this.#gen) return;
		this.phase = 'reveal';
		await this.wipe?.reveal(plan.kind, plan.revealMs);
		if (token !== this.#gen) return;
		this.#plan = null;
		this.phase = 'idle';
	}

	/** The SceneLayer finished mounting `id` (art decoded, view ready). */
	mounted(id: SceneId) {
		this.#mounted = id;
		const ready = this.#waiters.filter((w) => w.id === id);
		this.#waiters = this.#waiters.filter((w) => w.id !== id);
		for (const w of ready) w.done();
	}

	/** Resolves once `id` is mounted (boot screen, transitions). */
	whenMounted(id: SceneId): Promise<void> {
		return this.#whenMounted(id);
	}

	#whenMounted(id: SceneId): Promise<void> {
		if (this.#mounted === id) return Promise.resolve();
		return new Promise((done) => this.#waiters.push({ id, done }));
	}
}

/** The game shell's stage (one shell per page). */
export const sceneStage = new SceneStage();
