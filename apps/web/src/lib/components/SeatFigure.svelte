<!--
	A listener on a Tử Tiêu Cung cushion, drawn with their chosen look (decisions 2026-11-09, GĐ-A): the body's
	`characters/<body_id>[/<outfit_id>]/listener` art (lazy; outfit, then body), else a small still SVG figure of the look (robe colour, hair, pose).
	Without a look (older data) the shared `characters/listener` art or the plain silhouette. Display only.
-->
<script lang="ts">
	import { art } from '$lib/game/art';
	import { charArtFirst } from '$lib/game/charArt.svelte';
	import { cosmetics } from '$lib/game/cosmetics.svelte';
	import { bodyOf, figureOf, listenerSlots, normalizeLook } from '$lib/game/looks';
	import type { Aura, Look } from '$lib/game/types';
	import CultivatorFigure from './CultivatorFigure.svelte';

	const shared = art('characters/listener');

	let { look, aura }: { look?: Look; aura: Aura } = $props();
	const figure = $derived(figureOf(cosmetics.looks, look));
	// the outfit's listener art, else the body's (default outfit), else the SVG figure of the look
	const own = $derived(
		look ? charArtFirst(listenerSlots(cosmetics.looks ? normalizeLook(cosmetics.looks, look) : look, { defaultOutfit: bodyOf(cosmetics.looks, look.bodyId)?.defaultOutfit })) : undefined
	);
</script>

{#if look}
	{#if own}
		<img class="sitter" src={own.src} srcset={own.srcset} alt="" draggable="false" data-body={look.bodyId} />
	{:else}
		<span class="sitter figure" data-body={look.bodyId}><CultivatorFigure {aura} {figure} still class="size-full" /></span>
	{/if}
{:else if shared}
	<img class="sitter" src={shared.src} srcset={shared.srcset} alt="" draggable="false" />
{:else}
	<span class="sitter sitter-fallback" aria-hidden="true"></span>
{/if}

<style>
	.sitter {
		position: absolute;
		inset: 0;
		width: 100%;
		filter: drop-shadow(0 0 2px color-mix(in oklab, var(--color-mist-100) 35%, transparent));
	}
	.figure {
		inset: -12% -6% 6%;
		width: auto;
	}
	.sitter-fallback {
		inset: 25% 25% 10%;
		width: auto;
		border-radius: 50% 50% 35% 35%;
		background: var(--color-ink-800);
	}
	.sitter {
		transform-origin: 50% 100%;
	}
	:global(:root[data-fx='full']) .sitter,
	:global(:root[data-fx='lite'] .seat.is-mine) .sitter {
		animation: sit-breathe 5.2s var(--ease-in-out-sine) infinite;
		animation-delay: calc(var(--seat-i, 0) * -0.7s);
	}
	@keyframes sit-breathe {
		50% {
			transform: scale(1.01, 1.018);
		}
	}
	:global(.seat.is-mine) .sitter {
		filter: drop-shadow(0 0 5px var(--color-gold-300));
	}
</style>
