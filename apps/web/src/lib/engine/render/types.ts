// SceneRenderer contract (spec §3.2): the only thing that knows whether a scene is drawn in 2D (DOM/CSS + Canvas2D)
// or, later, 3D. UI and FX ask it for screen positions of anchors and never assume pixels or percentages.
import type { Quality } from '../fx/particles';
import type { Anchor, SceneDef } from '../scene/types';
import type { Fallback3dReason } from './select';

export type RendererKind = '2d' | '3d';

export interface RendererOptions {
	quality: Quality;
	/** 3D: run the fps watchdog (fallback to 2D under 40 fps for 5 s); default true. `?fpsguard=0` for software-GL tests. */
	fpsGuard?: boolean;
}

export interface ScreenPoint {
	x: number;
	y: number;
}

export interface SceneRenderer {
	readonly kind: RendererKind;
	mount(el: HTMLElement, def: SceneDef, opts: RendererOptions): Promise<void>;
	unmount(): void;
	/** Viewport (client) coordinates of an anchor, or null for an unknown anchor / unmounted renderer. */
	anchorToScreen(id: string): ScreenPoint | null;
	/** Viewport coordinates of a normalised scene point that is not a named anchor (server-sized layouts such as the
	 * hall's seat rows: the def holds the rule, the server the count). anchorToScreen(id) = projectPoint(anchors[id]). */
	projectPoint(p: Anchor): ScreenPoint | null;
	/** Screen px per 1.0 of scene width at the current camera — overlays size themselves with it (2D: frame width ×
	 * zoom; 3D: at the reference depth). 0 while unmounted. */
	scale(): number;
	/** Called after anything that moves anchors on screen (resize, camera). Returns the unsubscribe. */
	onLayout(fn: () => void): () => void;
	setCamera(preset: string, ms?: number): Promise<void>;
	setQuality(q: Quality): void;
	pause(): void;
	resume(): void;
	/** 3D only: the renderer gave up at runtime (context lost, too slow) — the caller swaps the scene to 2D. */
	onFail?(fn: (reason: Fallback3dReason) => void): () => void;
}
