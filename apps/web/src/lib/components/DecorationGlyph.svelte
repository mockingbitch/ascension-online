<!-- SVG fallback for cave / faction decorations (64×64, currentColor + a soft fill). Placeholder art: a raster file at
     src/lib/assets/art/cosmetics/<slug>.webp replaces it (CosmeticArt). -->
<script lang="ts">
	import type { DecoShape } from '$lib/game/cosmetics.svelte';

	interface Props {
		shape: DecoShape;
		class?: string;
	}
	let { shape, class: cls = '' }: Props = $props();
</script>

<svg viewBox="0 0 64 64" class={cls} fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
	{#if shape === 'lantern'}
		<path d="M24 54h16M28 54V46h8v8M22 46h20M24 46l2-18h12l2 18M22 28h20l-10-10z" />
		<path d="M29 34h6v8h-6z" fill="currentColor" fill-opacity=".35" />
	{:else if shape === 'hanging'}
		<path d="M32 4v8M26 12h12" />
		<ellipse cx="32" cy="30" rx="13" ry="16" fill="currentColor" fill-opacity=".2" />
		<path d="M26 46h12M32 46v8M28 54l4 6 4-6M22 22c6 3 14 3 20 0M22 38c6-3 14-3 20 0" />
	{:else if shape === 'plant'}
		<path d="M22 46h20l-3 12H25z" fill="currentColor" fill-opacity=".2" />
		<path d="M32 46V22M32 34c-6-2-12-8-12-16 6 2 11 7 12 12M32 30c5-2 10-7 11-14-6 1-10 6-11 10M32 40c4-1 9-1 13 3" />
	{:else if shape === 'vase'}
		<path d="M27 30h10l-1 4c5 3 6 10 4 16-1 4-4 8-8 8s-7-4-8-8c-2-6-1-13 4-16z" fill="currentColor" fill-opacity=".2" />
		<path d="M32 30V14M32 22l-9-10M32 18l8-8M32 24l10-4" />
		<circle cx="23" cy="12" r="2.5" fill="currentColor" /><circle cx="40" cy="10" r="2.5" fill="currentColor" /><circle cx="42" cy="20" r="2.5" fill="currentColor" />
	{:else if shape === 'screen'}
		<path d="M8 14h16v38H8zM24 12h16v40H24zM40 14h16v38H40z" fill="currentColor" fill-opacity=".14" />
		<path d="M12 40c4-6 8-6 10-12M28 44c4-10 8-14 10-22M46 38l6-8M8 56v4M56 56v4" />
	{:else if shape === 'scroll'}
		<path d="M14 8h36M14 56h36M18 8v48h28V8" />
		<rect x="18" y="8" width="28" height="48" fill="currentColor" fill-opacity=".14" stroke="none" />
		<circle cx="38" cy="22" r="6" /><path d="M22 46c6-8 10-10 20-12" />
	{:else if shape === 'plaque'}
		<rect x="8" y="22" width="48" height="20" rx="2" fill="currentColor" fill-opacity=".18" />
		<path d="M14 32h8M28 32h8M42 32h8M20 22l12-10 12 10" />
	{:else if shape === 'cushion'}
		<ellipse cx="32" cy="44" rx="22" ry="9" fill="currentColor" fill-opacity=".22" />
		<path d="M10 44c0-6 10-10 22-10s22 4 22 10M18 40c8 3 20 3 28 0" />
	{:else if shape === 'banner'}
		<path d="M18 4v56" />
		<path d="M18 8h28l-6 10 6 10H18z" fill="currentColor" fill-opacity=".25" />
		<circle cx="18" cy="4" r="2" fill="currentColor" />
	{:else if shape === 'cauldron'}
		<path d="M14 26h36c0 14-8 22-18 22S14 40 14 26z" fill="currentColor" fill-opacity=".2" />
		<path d="M12 26h40M20 46l-4 10M44 46l4 10M32 48v8M20 26v-6h4M44 26v-6h-4M28 18c0-4 4-4 4-8M36 18c0-4-4-4-4-8" />
	{:else if shape === 'pavilion'}
		<path d="M6 24c10 0 18-6 26-14 8 8 16 14 26 14" />
		<path d="M12 24h40M16 24v28M48 24v28M10 52h44M24 34h16" />
		<path d="M16 24h32v6H16z" fill="currentColor" fill-opacity=".2" />
	{:else if shape === 'pond'}
		<ellipse cx="32" cy="44" rx="24" ry="10" fill="currentColor" fill-opacity=".2" />
		<path d="M24 42c2-4 6-6 8-2 2-4 6-2 8 2M32 40V28M32 28c-4-6 0-10 0-10s4 4 0 10" />
	{:else if shape === 'fan'}
		<path d="M32 50 10 26a30 30 0 0 1 44 0z" fill="currentColor" fill-opacity=".2" />
		<path d="M32 50 18 18M32 50V14M32 50l14-32M32 50v8" />
	{:else if shape === 'stove'}
		<path d="M16 30h32v20H16z" fill="currentColor" fill-opacity=".2" />
		<path d="M14 30h36M18 50l-2 8M46 50l2 8M24 40h16M28 22c0-4 4-4 4-8M36 24c0-4-3-4-3-7" />
		<circle cx="32" cy="40" r="4" fill="currentColor" fill-opacity=".5" />
	{:else if shape === 'pillar'}
		<path d="M24 10h16v44H24zM20 54h24M20 10h24" fill="currentColor" fill-opacity=".14" />
		<path d="M28 18h8M28 26h8M28 34h8" />
	{:else if shape === 'lion'}
		<path d="M18 54h28M22 54V36c0-8 4-14 10-14s10 6 10 14v18" fill="currentColor" fill-opacity=".18" />
		<circle cx="32" cy="22" r="9" /><path d="M28 22h1M35 22h1M30 27h4" />
	{:else}
		<path d="M8 20h48M12 14h40M14 20v38M50 20v38M14 30h36" />
		<path d="M24 30v8h16v-8" fill="currentColor" fill-opacity=".2" />
	{/if}
</svg>
