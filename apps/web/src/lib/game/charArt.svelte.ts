// Lazy raster art for per-body slots (game/looks.ts). `charArt(path)` returns the art once known: undefined while the
// index/URL is loading or when no file exists — callers draw the SVG fallback meanwhile, so nothing waits and nothing
// 404s. The index module is fetched on first use only (never in the initial /play bundle).
import type { Art } from './art';

const EXT = ['avif', 'webp', 'png'];
const ROOT = '/src/lib/assets/art/';

const known = $state<Record<string, Art | null>>({});
const pending = new Set<string>();
let index: Promise<Record<string, () => Promise<string>>> | null = null;

async function resolve(path: string): Promise<Art | null> {
	index ??= import('./charArtIndex').then((m) => m.files);
	const files = await index;
	for (const ext of EXT) {
		const load = files[`${ROOT}${path}.${ext}`];
		if (!load) continue;
		const hi = files[`${ROOT}${path}@2x.${ext}`];
		const [src, src2] = await Promise.all([load(), hi ? hi() : Promise.resolve(undefined)]);
		return src2 ? { src, srcset: `${src} 1x, ${src2} 2x` } : { src };
	}
	return null;
}

/** Art at `path` (e.g. "characters/tien_nu_y_su/avatar"), or undefined (loading / not shipped). Reactive. */
export function charArt(path: string): Art | undefined {
	const v = known[path];
	if (v === undefined && !pending.has(path)) {
		pending.add(path);
		void resolve(path)
			.then((a) => (known[path] = a))
			.catch(() => (known[path] = null));
	}
	return v ?? undefined;
}

/** First art among `paths` (fallback chains), once every earlier slot is known to be missing. Reactive. */
export function charArtFirst(paths: string[]): Art | undefined {
	for (const p of paths) {
		const a = charArt(p);
		if (a) return a;
		if (known[p] === undefined) return undefined; // an earlier slot is still loading: don't show a later one yet
	}
	return undefined;
}

/** Like `charArtFirst`, plus which slot answered (e.g. to apply a body-level robe mask only to body-level art). */
export function charArtFirstSlot(paths: string[]): { art: Art; slot: string } | undefined {
	for (const p of paths) {
		const a = charArt(p);
		if (a) return { art: a, slot: p };
		if (known[p] === undefined) return undefined;
	}
	return undefined;
}
