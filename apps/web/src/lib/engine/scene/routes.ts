// Scenes are routes (spec §3.3): this table maps /play paths to the scene the SceneLayer shows, and plans the
// transition between two scenes for an effects quality (§4.2). Pure functions — the stage (stage.svelte.ts) runs them.
// Mode routes (the mini games, GĐ5) are full-screen modes over whatever scene is behind them, or the default scene when
// the game is opened on them directly. Menus are windows (shallow routing, lib/game/windowStack.ts), never routes.
import type { Quality } from '../fx/particles';
import type { SceneDef, SceneId } from './types';

export const DEFAULT_SCENE: SceneId = 'world';

/** Route of each scene that has one. */
export const sceneRoutes: Partial<Record<SceneId, string>> = {
	world: '/play',
	dong_phu: '/play/tu-luyen',
	tu_tieu_cung: '/play/tu-tieu-cung'
};

const byPath = new Map(Object.entries(sceneRoutes).map(([id, path]) => [path, id as SceneId]));

const normalise = (path: string) => (path.length > 1 && path.endsWith('/') ? path.slice(0, -1) : path);

/** Routes rendered as full-screen modes in the ModeLayer (HUD, action bar, chat and windows hidden). */
export const modeRoutes = ['/play/tam-ma', '/play/hoa-pham', '/play/bi-canh'] as const;

/** Is this path a full-screen mode route? */
export const isModePath = (path: string) => (modeRoutes as readonly string[]).includes(normalise(path));

/** Is this path inside the game shell (/play/**)? */
export const inGame = (path: string) => {
	const p = normalise(path);
	return p === '/play' || p.startsWith('/play/');
};

/** The scene a path IS (null for screens and paths outside /play). */
export const sceneOfPath = (path: string): SceneId | null => byPath.get(normalise(path)) ?? null;

/**
 * The scene to show for `path`: a scene route shows its scene; any other /play screen keeps the scene already shown
 * (or the default one on a deep link); outside /play there is no scene (null).
 */
export function resolveScene(path: string, shown: SceneId | null): SceneId | null {
	if (!inGame(path)) return null;
	return sceneOfPath(path) ?? shown ?? DEFAULT_SCENE;
}

export type TransitionKind = 'wipe' | 'fade' | 'instant';

export interface TransitionPlan {
	kind: TransitionKind;
	/** camera preset to zoom to before covering (world → a place), with its duration */
	zoom: string | null;
	zoomMs: number;
	/** cover in / reveal out; together ≈ --dur-scene (650 ms) for the ink wipe */
	coverMs: number;
	revealMs: number;
}

/** Durations (ms). The wipe totals --dur-scene; lite is a short plain fade; off swaps instantly. */
export const TIMING = {
	wipe: { cover: 325, reveal: 325, zoom: 420 },
	fade: { cover: 120, reveal: 140 }
} as const;

/** Camera preset that frames the hotspot leading to `toPath` in `def` (world → place zoom), if any. */
export function zoomPresetFor(def: SceneDef | null, toPath: string): string | null {
	if (!def?.camera) return null;
	const p = normalise(toPath);
	const h = def.hotspots.find((x) => x.action.kind === 'route' && normalise(x.action.href) === p);
	const id = h && `focus.${h.id}`;
	return id && def.camera.some((c) => c.id === id) ? id : null;
}

/**
 * How to go from the shown scene to `to`. `fromDef` is the def on screen (for the zoom toward the clicked hotspot).
 * Same scene → null (no transition: e.g. a screen over the world, or back to the world from a screen over it).
 */
export function planTransition(
	from: SceneId | null,
	to: SceneId | null,
	quality: Quality,
	opts: { fromDef?: SceneDef | null; toPath?: string } = {}
): TransitionPlan | null {
	if (!to || from === to) return null;
	if (!from || quality === 'off') return { kind: 'instant', zoom: null, zoomMs: 0, coverMs: 0, revealMs: 0 };
	if (quality === 'lite') return { kind: 'fade', zoom: null, zoomMs: 0, coverMs: TIMING.fade.cover, revealMs: TIMING.fade.reveal };
	const zoom = opts.toPath ? zoomPresetFor(opts.fromDef ?? null, opts.toPath) : null;
	return { kind: 'wipe', zoom, zoomMs: zoom ? TIMING.wipe.zoom : 0, coverMs: TIMING.wipe.cover, revealMs: TIMING.wipe.reveal };
}
