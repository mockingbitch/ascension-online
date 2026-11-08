// What a scene view (the DOM overlay of a scene: plaques, characters, seats, bubbles, ambient) gets from the
// SceneLayer. Every position goes through the renderer (anchors / projected scene points), converted to px inside
// the scene box; `tick` re-renders positions after a resize or camera change. Views never assume 2D frame math.
import type { Component } from 'svelte';
import type { ParticleField } from '../fx/layer';
import type { Quality } from '../fx/particles';
import type { SceneRenderer, ScreenPoint } from '../render/types';
import type { StagePhase } from './stage.svelte';
import type { Anchor, SceneDef } from './types';

export class SceneViewCtx {
	/** bumps whenever anchors moved on screen */
	tick = $state(0);
	readonly renderer: SceneRenderer;
	readonly def: SceneDef;
	/** the scene's ambient particle field (box-local px, under the UI; paused while the scene is covered/moving) */
	readonly fx: ParticleField;
	#host: HTMLElement;
	#off: () => void;

	constructor(renderer: SceneRenderer, def: SceneDef, host: HTMLElement, fx: ParticleField) {
		this.renderer = renderer;
		this.def = def;
		this.fx = fx;
		this.#host = host;
		this.#off = renderer.onLayout(() => this.tick++);
	}

	/** Box-local px of a named anchor (null if unknown). Reactive on layout. */
	at(id: string): ScreenPoint | null {
		void this.tick;
		return this.#local(this.renderer.anchorToScreen(id));
	}

	/** Box-local px of a normalised scene point (server-sized layouts). Reactive on layout. */
	project(p: Anchor): ScreenPoint | null {
		void this.tick;
		return this.#local(this.renderer.projectPoint(p));
	}

	/** px per 1% of the scene width — overlays size themselves with `calc(var(--u) * n)`. */
	get unit(): number {
		void this.tick;
		return this.renderer.scale() / 100;
	}

	dispose() {
		this.#off();
		this.fx.reset();
	}

	#local(p: ScreenPoint | null): ScreenPoint | null {
		if (!p) return null;
		const b = this.#host.getBoundingClientRect();
		return { x: p.x - b.left, y: p.y - b.top };
	}
}

export interface SceneViewProps {
	scene: SceneViewCtx;
	quality: Quality;
	phase: StagePhase;
	/** a window or screen covers the scene: ambient loops pause */
	covered: boolean;
}

export type SceneView = Component<SceneViewProps>;

/** Run `spawn` every `everyMs` of loop time (an ambient emitter for ParticleField.addEmitter). */
export function every(everyMs: () => number, spawn: () => void): (dt: number) => void {
	let acc = 0;
	return (dt) => {
		const ms = everyMs();
		if (!(ms > 0)) return;
		acc += dt;
		// after a long stall spawn at most two, not a burst
		if (acc > ms * 2) acc = ms * 2;
		while (acc >= ms) {
			acc -= ms;
			spawn();
		}
	};
}
