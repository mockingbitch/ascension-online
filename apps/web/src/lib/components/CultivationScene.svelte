<!--
	Focal stage of the Cultivation Hall: the cultivator meditating over a rotating formation while qi motes
	converge on the pearl. `ready` (bottleneck reached) adds a gold pillar of light. Pure CSS/SVG, ~20 nodes animated.
-->
<script lang="ts">
	import type { Aura } from '$lib/game/types';
	import { art } from '$lib/game/art';
	import CultivatorFigure from './CultivatorFigure.svelte';
	import FormationCircle from './FormationCircle.svelte';

	interface Props {
		aura: Aura;
		ready?: boolean;
		/** Increment to play the failed-breakthrough backlash once. */
		backlash?: number;
	}
	let { aura, ready = false, backlash = 0 }: Props = $props();
	const meditate = art('characters/meditate'); // raster slot; includes its own rune array

	// Deterministic mote spawn ring (no Math.random → stable SSR/hydration, no layout jitter).
	const motes = Array.from({ length: 16 }, (_, i) => {
		const a = (i / 16) * Math.PI * 2 + (i % 2) * 0.3;
		const r = 150 + (i % 4) * 22;
		return { x: Math.cos(a) * r, y: Math.sin(a) * r * 0.7, delay: (i * 0.37) % 4.2, size: 3 + (i % 3) };
	});
</script>

{#key backlash}
<div class:shake={backlash > 0} class="scene relative mx-auto aspect-[4/3] w-full max-w-[480px] overflow-hidden" style:--aura="var(--color-aura-{aura})">
	<div class="halo absolute inset-0" aria-hidden="true"></div>
	{#if ready}<div class="pillar absolute inset-y-0 left-1/2" aria-hidden="true"></div>{/if}

	{#if meditate}
		<img src={meditate.src} srcset={meditate.srcset} alt="" class="art absolute inset-x-0 bottom-0 mx-auto h-[92%] object-contain" draggable="false" />
	{:else}
		<!-- formation on the floor, in perspective -->
		<div class="floor absolute left-1/2 w-[86%]" aria-hidden="true">
			<div class="spin"><FormationCircle color={ready ? 'var(--color-gold-300)' : 'var(--aura)'} class="size-full" /></div>
		</div>

		<CultivatorFigure {aura} class="figure absolute inset-x-0 bottom-[8%] mx-auto h-[62%]" />
	{/if}

	<!-- converging qi motes; anchor = the pearl -->
	<div class="anchor absolute" aria-hidden="true">
		{#each motes as m, i (i)}
			<span class="mote" style:--x="{m.x}px" style:--y="{m.y}px" style:--d="{m.delay}s" style:--s="{m.size}px"></span>
		{/each}
	</div>
	{#if backlash > 0}
		<div class="backlash absolute inset-0" aria-hidden="true"></div>
	{/if}
</div>
{/key}

<style>
	.art {
		filter: drop-shadow(0 0 18px color-mix(in oklab, var(--aura) 45%, transparent));
		animation: var(--animate-qi-breathe);
	}
	.halo {
		background:
			radial-gradient(circle at 50% 62%, color-mix(in oklab, var(--aura) 34%, transparent) 0, transparent 34%),
			radial-gradient(ellipse at 50% 82%, color-mix(in oklab, var(--aura) 18%, transparent) 0, transparent 55%);
		animation: var(--animate-qi-breathe);
	}
	.pillar {
		width: 34%;
		transform: translateX(-50%);
		background: linear-gradient(90deg, transparent, color-mix(in oklab, var(--color-gold-300) 22%, transparent), transparent);
		mask-image: linear-gradient(180deg, transparent, black 25%, black 75%, transparent);
		animation: pillar 2.8s ease-in-out infinite;
	}
	@keyframes pillar {
		0%,
		100% {
			opacity: 0.55;
			transform: translateX(-50%) scaleX(0.85);
		}
		50% {
			opacity: 1;
			transform: translateX(-50%) scaleX(1);
		}
	}
	.floor {
		bottom: -40%;
		aspect-ratio: 1;
		transform: translateX(-50%) scaleY(0.3);
		opacity: 0.75;
		filter: drop-shadow(0 0 6px var(--aura));
	}
	.spin {
		width: 100%;
		height: 100%;
		animation: spin 80s linear infinite;
	}
	@keyframes spin {
		to {
			transform: rotate(360deg);
		}
	}
	/* pearl sits at (200,236) of the figure's 320×300 viewBox crop starting at (40,40); figure = bottom 8%, h 62% */
	.anchor {
		left: 50%;
		top: calc(30% + 62% * (196 / 300));
	}
	.shake {
		animation: shake 420ms ease-out;
	}
	@keyframes shake {
		0%, 100% { transform: none; }
		20% { transform: translateX(-6px); }
		40% { transform: translateX(5px); }
		60% { transform: translateX(-3px); }
		80% { transform: translateX(2px); }
	}
	.backlash {
		pointer-events: none;
		background: radial-gradient(circle at 50% 70%, color-mix(in oklab, var(--color-cinnabar-500) 55%, transparent), transparent 55%);
		opacity: 0;
		animation: backlash 1.1s ease-out;
	}
	@keyframes backlash {
		0% { opacity: 0; transform: scale(0.6); }
		15% { opacity: 1; }
		100% { opacity: 0; transform: scale(1.4); }
	}
	.mote {
		position: absolute;
		width: var(--s);
		height: var(--s);
		margin: calc(var(--s) / -2);
		border-radius: 50%;
		background: var(--aura);
		box-shadow: 0 0 8px var(--aura);
		opacity: 0;
		animation: converge 4.2s cubic-bezier(0.55, 0, 0.8, 0.4) var(--d) infinite;
	}
	@keyframes converge {
		0% {
			transform: translate(var(--x), var(--y)) scale(1);
			opacity: 0;
		}
		25% {
			opacity: 0.9;
		}
		100% {
			transform: translate(0, 0) scale(0.2);
			opacity: 0;
		}
	}
</style>
