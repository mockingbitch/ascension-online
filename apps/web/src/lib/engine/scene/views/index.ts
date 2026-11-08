// Scene views (DOM overlays), loaded on demand like the defs so no scene weighs on the first /play load.
import type { SceneView } from '../view.svelte';
import type { SceneId } from '../types';

export const sceneViews: Partial<Record<SceneId, () => Promise<SceneView>>> = {
	world: () => import('./WorldView.svelte').then((m) => m.default),
	dong_phu: () => import('./DongPhuView.svelte').then((m) => m.default),
	tu_tieu_cung: () => import('./HallView.svelte').then((m) => m.default)
};

export async function loadSceneView(id: SceneId): Promise<SceneView | null> {
	return (await sceneViews[id]?.()) ?? null;
}
