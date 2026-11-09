// Per-body character art (characters/<body_id>/…, per-outfit characters/<body_id>/<outfit_id>/…, characters/acc/…, characters/guide/…): a LAZY glob, reached only
// through a dynamic import from charArt.svelte.ts, so neither the images nor their URL list are in the initial /play
// bundle (decisions 2026-11-09, GĐ-A). Each entry resolves the asset URL on demand.
// fx/aura-<tier> (persistent realm aura overlays) load the same way.
export const files = import.meta.glob(['/src/lib/assets/art/characters/*/*.{webp,avif,png}', '/src/lib/assets/art/characters/*/*/*.{webp,avif,png}', '/src/lib/assets/art/fx/*.{webp,avif,png}'], {
	query: '?url',
	import: 'default'
}) as Record<string, () => Promise<string>>;
