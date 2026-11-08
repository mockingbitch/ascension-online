// Scene registry: each def is loaded on demand (dynamic import) so scenes never weigh on the first /play load.
import type { SceneDef, SceneId } from '../types';

export const sceneLoaders: Partial<Record<SceneId, () => Promise<SceneDef>>> = {
	world: () => import('./world').then((m) => m.world),
	tu_tieu_cung: () => import('./tu_tieu_cung').then((m) => m.tuTieuCung),
	dong_phu: () => import('./dong_phu').then((m) => m.dongPhu)
};

export async function loadSceneDef(id: SceneId): Promise<SceneDef | null> {
	return (await sceneLoaders[id]?.()) ?? null;
}
