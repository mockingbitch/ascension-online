// 3D asset slots (spec §5): drop files into src/lib/assets/scenes/<scene id>/{model.glb, env.hdr} and
// src/lib/assets/characters/<id>.glb and the 3D renderer uses them on the next build; without them it draws the
// procedural hall. Like lib/game/art.ts the glob is resolved at build time, so a missing file is never requested.
// Loaders are separate dynamic imports: they cost nothing until a file exists. KTX2/Draco/meshopt decoders are not
// wired yet (next step when real art lands — render3d/README.md).
import { PMREMGenerator, type Group, type Scene, type Texture, type WebGLRenderer } from 'three';

const files = import.meta.glob('/src/lib/assets/{scenes,characters}/**/*.{glb,hdr}', {
	eager: true,
	query: '?url',
	import: 'default'
}) as Record<string, string>;

export function sceneAsset(scene: string, name: 'model.glb' | 'env.hdr'): string | undefined {
	return files[`/src/lib/assets/scenes/${scene}/${name}`];
}

export function characterAsset(id: string): string | undefined {
	return files[`/src/lib/assets/characters/${id}.glb`];
}

export const has3dAssets = Object.keys(files).length > 0;

export async function loadGlb(url: string): Promise<Group> {
	const { GLTFLoader } = await import('three/examples/jsm/loaders/GLTFLoader.js');
	const gltf = await new GLTFLoader().loadAsync(url);
	return gltf.scene;
}

/** Prefiltered environment from an equirect .hdr; the caller disposes the returned texture. */
export async function loadEnv(url: string, renderer: WebGLRenderer, scene: Scene): Promise<Texture> {
	const { HDRLoader } = await import('three/examples/jsm/loaders/HDRLoader.js');
	const hdr = await new HDRLoader().loadAsync(url);
	const pmrem = new PMREMGenerator(renderer);
	const env = pmrem.fromEquirectangular(hdr).texture;
	hdr.dispose();
	pmrem.dispose();
	scene.environment = env;
	return env;
}
