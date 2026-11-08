// Renderer factory behind the `renderer` flag. Only 2D exists; '3d' is reserved for the GĐ7 spike (render3d/README.md)
// and falls back to 2D. Loaded lazily so scenes don't weigh on the first /play load.
import type { RendererKind, SceneRenderer } from './types';

export async function createRenderer(_kind: RendererKind): Promise<SceneRenderer> {
	const { Render2D } = await import('./render2d/Render2D');
	return new Render2D();
}
