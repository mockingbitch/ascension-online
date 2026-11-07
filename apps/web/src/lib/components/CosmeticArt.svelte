<!-- Visual of one cosmetic or faction decoration: raster slot `cosmetics/<slug>` when shipped, SVG/CSS fallback
     otherwise (no 404s). Titles and chat effects render as text samples. Display only. -->
<script lang="ts">
	import { art } from '$lib/game/art';
	import { decoShape, frameStyle } from '$lib/game/cosmetics.svelte';
	import type { CosmeticKind, Rarity } from '$lib/game/types';
	import DecorationGlyph from './DecorationGlyph.svelte';
	import NameFx from './NameFx.svelte';

	interface Props {
		kind: CosmeticKind | 'faction_decoration';
		slug: string;
		rarity?: Rarity;
		/** sample text for titles / chat effects */
		label?: string;
		size?: number;
	}
	let { kind, slug, rarity = 'linh', label = '', size = 56 }: Props = $props();
	const a = $derived(art(`cosmetics/${slug}`));
</script>

<span class="cart" style:--r="var(--color-rarity-{rarity})" style:width="{size}px" style:height="{size}px">
	{#if a && kind !== 'title' && kind !== 'chat_aura'}
		<img src={a.src} srcset={a.srcset} alt="" class="size-full object-contain" draggable="false" />
	{:else if kind === 'avatar_frame'}
		<span class="disc"><span class="cos-frame frame-{frameStyle(`x:${slug}`)}"></span></span>
	{:else if kind === 'title'}
		<span class="title-seal font-display">{label.split(' ').slice(0, 2).map((w) => w[0]).join('')}</span>
	{:else if kind === 'chat_aura'}
		<NameFx fx={`chat_aura:${slug}`} class="text-sm font-semibold">Danh</NameFx>
	{:else}
		<DecorationGlyph shape={decoShape(slug)} class="size-[82%]" />
	{/if}
</span>

<style>
	.cart {
		position: relative;
		display: grid;
		place-items: center;
		flex-shrink: 0;
		color: var(--r);
	}
	.disc {
		position: relative;
		width: 74%;
		height: 74%;
		border-radius: 9999px;
		background: radial-gradient(circle at 50% 60%, var(--color-ink-700), var(--color-ink-950));
	}
	.title-seal {
		display: grid;
		place-items: center;
		width: 78%;
		height: 78%;
		border: 1px solid var(--r);
		border-radius: 0.35rem;
		font-size: 1.05rem;
		font-weight: 700;
		color: var(--r);
		background: color-mix(in oklab, var(--r) 12%, var(--color-ink-950));
	}
</style>
