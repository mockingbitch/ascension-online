<!-- Round character portrait: bust crop of the cultivator art inside a double gold ring, tinted by realm aura.
     `frameId` = equipped avatar_frame cosmetic (looks only): raster `cosmetics/<slug>` overlay, CSS ring otherwise. -->
<script lang="ts">
	import { art } from '$lib/game/art';
	import { frameStyle, slugOf } from '$lib/game/cosmetics.svelte';
	import type { Aura } from '$lib/game/types';
	import CultivatorFigure from './CultivatorFigure.svelte';

	const avatar = art('characters/avatar'); // raster slot (docs/design/art-prompts.md §4), SVG otherwise

	interface Props {
		aura: Aura;
		size?: number; // px
		frameId?: string;
	}
	let { aura, size = 80, frameId }: Props = $props();
	const frameArt = $derived(frameId ? art(`cosmetics/${slugOf(frameId)}`) : undefined);
</script>

<span class="relative inline-block shrink-0" style:width="{size}px" style:height="{size}px">
	<span
		class="portrait relative block size-full overflow-hidden rounded-full"
		class:framed={!!frameId}
		style:--aura="var(--color-aura-{aura})"
	>
		{#if avatar}
			<img src={avatar.src} srcset={avatar.srcset} alt="" class="size-full object-cover" draggable="false" />
		{:else}
			<CultivatorFigure {aura} crop="bust" class="size-full" />
		{/if}
	</span>
	{#if frameId}
		{#if frameArt}
			<img src={frameArt.src} srcset={frameArt.srcset} alt="" class="frame-art" draggable="false" />
		{:else}
			<span class="cos-frame frame-{frameStyle(frameId)}" aria-hidden="true"></span>
		{/if}
	{/if}
</span>

<style>
	.portrait {
		background: radial-gradient(circle at 50% 60%, color-mix(in oklab, var(--aura) 35%, var(--color-ink-800)), var(--color-ink-950));
		box-shadow:
			0 0 0 1px var(--color-gold-500),
			0 0 0 4px var(--color-ink-950),
			0 0 0 5px color-mix(in oklab, var(--color-gold-500) 45%, transparent),
			0 0 18px color-mix(in oklab, var(--aura) 40%, transparent);
	}
	.portrait.framed {
		box-shadow: 0 0 18px color-mix(in oklab, var(--aura) 40%, transparent);
	}
	.frame-art {
		position: absolute;
		inset: -14%;
		width: 128%;
		height: 128%;
		max-width: none;
		pointer-events: none;
	}
</style>
