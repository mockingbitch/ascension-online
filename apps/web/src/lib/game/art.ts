// Raster art slots. Drop files into src/lib/assets/art/<path>.{webp,avif,png} (optional `<path>@2x.<ext>` for
// retina) and the matching screen picks them up on the next build/dev reload — no code change. Anything missing
// falls back to the original SVG art, so the game never shows a broken image and never requests a 404.
// Expected files and their prompts: docs/design/art-prompts.md.
// Per-body character folders (characters/<body_id>/…) are NOT here: they load lazily via charArt.svelte.ts so the
// initial /play bundle carries neither their files nor their URLs (decisions 2026-11-09, GĐ-A).
const files = import.meta.glob(['/src/lib/assets/art/**/*.{webp,avif,png}', '!/src/lib/assets/art/characters/*/**', '!/src/lib/assets/art/fx/**'], {
	eager: true,
	query: '?url',
	import: 'default'
}) as Record<string, string>;

const EXT = ['avif', 'webp', 'png'];
const ROOT = '/src/lib/assets/art/';

export interface Art {
	src: string;
	srcset?: string;
}

/** The art at `path` (e.g. "world/sky", "items/tu_linh_phu"), or undefined when no file was added. */
export function art(path: string): Art | undefined {
	for (const ext of EXT) {
		const src = files[`${ROOT}${path}.${ext}`];
		if (!src) continue;
		const hi = files[`${ROOT}${path}@2x.${ext}`];
		return hi ? { src, srcset: `${src} 1x, ${hi} 2x` } : { src };
	}
	return undefined;
}

/** First available art among `paths` (fallback chains such as realm body art). */
export function artFirst(...paths: string[]): Art | undefined {
	for (const p of paths) {
		const a = art(p);
		if (a) return a;
	}
	return undefined;
}

/** Number of art files shipped (debug/diagnostics). */
export const artCount = Object.keys(files).filter((k) => !k.includes('@2x.')).length;
