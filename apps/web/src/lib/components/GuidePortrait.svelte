<!--
	Vân Khê's half-body portrait for one expression: raster art slot `characters/guide/<expr>` (~768×1024, alpha, lazy —
	not in the initial /play bundle), else the SVG GuideFigure. An optional `characters/guide/blink` overlay blinks at
	effects full only.
-->
<script lang="ts">
	import { charArt } from '$lib/game/charArt.svelte';
	import type { GuideExpr } from '$lib/game/types';
	import GuideFigure from './GuideFigure.svelte';

	let { expr, class: cls = '' }: { expr: GuideExpr; class?: string } = $props();
	const pic = $derived(charArt(`characters/guide/${expr}`));
	const blink = $derived(pic && (expr === 'neutral' || expr === 'point') ? charArt('characters/guide/blink') : undefined);
</script>

<span class="portrait relative block {cls}" data-guide-expr={expr}>
	{#if pic}
		<img src={pic.src} srcset={pic.srcset} alt="" class="art size-full object-contain object-bottom" draggable="false" />
		{#if blink}<img src={blink.src} srcset={blink.srcset} alt="" class="art blink absolute inset-0 size-full object-contain object-bottom" draggable="false" />{/if}
	{:else}
		<GuideFigure {expr} class="size-full" />
	{/if}
</span>

<style>
	.blink {
		opacity: 0;
	}
	:global(:root[data-fx='full']) .blink {
		animation: blink-img 5.5s steps(1, end) infinite;
	}
	:global(:root[data-fx='full']) .art:not(.blink),
	:global(:root[data-fx='lite']) .art:not(.blink) {
		transform-origin: 50% 100%;
		animation: breathe 4.2s var(--ease-in-out-sine) infinite;
	}
	@keyframes blink-img {
		0%,
		96% {
			opacity: 0;
		}
		97%,
		99% {
			opacity: 1;
		}
	}
	@keyframes breathe {
		0%,
		100% {
			transform: scale(1);
		}
		50% {
			transform: scale(1.01, 1.016);
		}
	}
</style>
