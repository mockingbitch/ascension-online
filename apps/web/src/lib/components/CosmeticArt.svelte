<!-- Visual of one cosmetic or faction decoration: raster slot `cosmetics/<slug>` when shipped, SVG/CSS fallback
     otherwise (no 404s). Titles and chat effects render as text samples. Display only. -->
<script lang="ts">
	import { art } from '$lib/game/art';
	import { decoShape, frameStyle } from '$lib/game/cosmetics.svelte';
	import type { CosmeticKind, Rarity } from '$lib/game/types';
	import DecorationGlyph from './DecorationGlyph.svelte';
	import FrameRing from './FrameRing.svelte';
	import TitlePlaque from './TitlePlaque.svelte';
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
		<span class="disc"><FrameRing style={frameStyle(`x:${slug}`)} /></span>
	{:else if kind === 'title'}
		<TitlePlaque name={label.split(' ').slice(0, 2).map((w) => w[0]).join('')} {rarity} seal />
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

</style>
