<!-- Round character portrait: bust crop of the cultivator art inside a double gold ring, tinted by realm aura. -->
<script lang="ts">
	import { art } from '$lib/game/art';
	import type { Aura } from '$lib/game/types';
	import CultivatorFigure from './CultivatorFigure.svelte';

	const avatar = art('characters/avatar'); // raster slot (docs/design/art-prompts.md §4), SVG otherwise

	interface Props {
		aura: Aura;
		size?: number; // px
	}
	let { aura, size = 80 }: Props = $props();
</script>

<span
	class="portrait relative block shrink-0 overflow-hidden rounded-full"
	style:width="{size}px"
	style:height="{size}px"
	style:--aura="var(--color-aura-{aura})"
>
	{#if avatar}
		<img src={avatar.src} srcset={avatar.srcset} alt="" class="size-full object-cover" draggable="false" />
	{:else}
		<CultivatorFigure {aura} crop="bust" class="size-full" />
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
</style>
