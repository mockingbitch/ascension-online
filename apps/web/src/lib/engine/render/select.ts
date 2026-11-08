// Which renderer draws a scene (GĐ7). Pure and tiny: it ships in the initial /play bundle; the 3D code itself is only
// reached through the dynamic import in ./index.ts. 2D is the default; 3D is opt-in (URL `?renderer=3d` or Cài đặt)
// for the scenes listed here, and a remembered fallback (no WebGL2, context lost, too slow) turns it off.
import type { SceneId } from '../scene/types';
import type { RendererKind } from './types';

/** Scenes that have a 3D version (the spike: Tử Tiêu Cung only). */
export const RENDER3D_SCENES: readonly SceneId[] = ['tu_tieu_cung'];

export type Fallback3dReason = 'no-webgl2' | 'context-lost' | 'slow' | 'error';

export interface Fallback3d {
	reason: Fallback3dReason;
	/** epoch ms */
	at: number;
}

export const FALLBACK_REASONS: readonly Fallback3dReason[] = ['no-webgl2', 'context-lost', 'slow', 'error'];

export interface RendererPrefs {
	/** `?renderer=` from the URL (dev/test override): '2d' | '3d' | anything else = ignored */
	requested: string | null;
	/** the Cài đặt choice */
	choice: RendererKind;
	/** remembered fallback: blocks 3D until the player picks 3D again */
	fallback: Fallback3d | null;
}

/** Renderer the player asked for (URL wins over the setting), before per-scene support and fallbacks. */
export function wantedRenderer(p: Pick<RendererPrefs, 'requested' | 'choice'>): RendererKind {
	if (p.requested === '3d' || p.requested === '2d') return p.requested;
	return p.choice;
}

/** Renderer for one scene: 3D only when wanted, supported by the scene and not fallen back. */
export function rendererFor(scene: SceneId | null, p: RendererPrefs): RendererKind {
	if (!scene || !RENDER3D_SCENES.includes(scene) || p.fallback) return '2d';
	return wantedRenderer(p);
}

