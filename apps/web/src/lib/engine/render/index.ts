// Renderer factory behind the `renderer` flag (settings.rendererFor(scene)). Both renderers are dynamic imports: 2D
// keeps scenes off the first /play load, and the 3D path (Three.js, GĐ7 spike) costs nothing unless `renderer=3d`.
import type { RendererKind, SceneRenderer } from './types';

export async function createRenderer(kind: RendererKind): Promise<SceneRenderer> {
	if (kind === '3d') {
		const { Render3D } = await import('./render3d/Render3D');
		return new Render3D();
	}
	const { Render2D } = await import('./render2d/Render2D');
	return new Render2D();
}
