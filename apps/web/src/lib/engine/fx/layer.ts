// Particle fields on the shared loop. A field = one pooled ParticleSystem + its canvas; it subscribes to the loop only
// while particles are alive or an ambient emitter wants to spawn — an idle game draws nothing.
//   fxLayer      the one full-screen effects canvas (FxLayer.svelte, above the UI): bursts, orbs flying to the HUD.
//                Screen space, so a 3D renderer can reuse it unchanged (positions come from anchorToScreen).
//   scene field  one per mounted scene (SceneLayer, inside the scene box, under the UI): ambient motes, petals, smoke.
//                Paused (and cleared) while the scene is covered, moving or the tab is hidden.
import { gameLoop } from '../loop';
import { ParticleSystem, type ParticlePreset, type PathSpec, type Quality } from './particles';

/** Called every frame while the field runs; spawns ambient particles (dt in ms). */
export type Emitter = (dtMs: number, ps: ParticleSystem) => void;

export class ParticleField {
	readonly particles: ParticleSystem;
	#emitters = new Set<Emitter>();
	#paused = false;
	#unsub: (() => void) | null = null;

	constructor(particles: ParticleSystem) {
		this.particles = particles;
	}

	get running() {
		return this.#unsub !== null;
	}
	get emitters() {
		return this.#emitters.size;
	}

	/** Emit `preset` at (x, y) in the field's coordinates. Returns how many were spawned (0 when effects are off). */
	burst(preset: ParticlePreset, x: number, y: number, count?: number): number {
		if (this.#paused) return 0;
		const n = this.particles.spawn(preset, x, y, count);
		this.#wake();
		return n;
	}

	/** Emit particles along a Bézier path (orbs to the HUD, motes converging). */
	path(preset: ParticlePreset, spec: PathSpec, count?: number): number {
		if (this.#paused) return 0;
		const n = this.particles.spawnPath(preset, spec, count);
		this.#wake();
		return n;
	}

	/** Register an ambient emitter; returns its removal. */
	addEmitter(fn: Emitter): () => void {
		this.#emitters.add(fn);
		this.#wake();
		return () => void this.#emitters.delete(fn);
	}

	/** Hold still and wipe (scene covered / in transition); resume restarts the emitters. */
	setPaused(paused: boolean) {
		if (paused === this.#paused) return;
		this.#paused = paused;
		if (paused) {
			this.particles.clear();
			this.#stop();
		} else this.#wake();
	}

	setQuality(q: Quality) {
		this.particles.setQuality(q);
		if (q === 'off') {
			this.particles.clear();
			this.#stop();
		} else this.#wake();
	}

	/** Remove emitters and particles (scene unmounted). */
	reset() {
		this.#emitters.clear();
		this.particles.clear();
		this.#stop();
	}

	#wants() {
		return !this.#paused && (this.particles.alive > 0 || (this.#emitters.size > 0 && this.particles.cap > 0));
	}

	#wake() {
		if (!this.#unsub && this.#wants()) this.#unsub = gameLoop().subscribe(this.#frame);
	}

	#stop() {
		this.#unsub?.();
		this.#unsub = null;
	}

	#frame = (dt: number) => {
		if (this.particles.cap > 0) for (const e of this.#emitters) e(dt, this.particles);
		this.particles.update(dt);
		this.particles.render();
		if (!this.#wants()) this.#stop();
	};
}

/** The full-screen effects field (FxLayer.svelte attaches its canvas). */
export const fxLayer = new ParticleField(new ParticleSystem('full'));
