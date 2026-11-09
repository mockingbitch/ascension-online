<!--
	Persistent realm aura around the meditating figure (decisions 2026-11-09 D): it comes from the REALM the server says
	the character is in (realms.yaml visual.fx_tier + aura) — never from anything bought. Layers add up with the tier:
	1 mist · 2 jade ring under the figure · 3 the nascent soul's faint shadow behind · 4 small sparks of lightning ·
	5 auspicious clouds underfoot · 6 a star orbit and the faint pháp tướng (the player's own figure, huge, behind).
	DOM/SVG only (the motes are the scene field's job, budgeted by auraAmbient): loops at effects full, a slower subset
	at lite, a static end state at off / reduced motion. An optional raster overlay `fx/aura-<tier>` wins when shipped.
-->
<script lang="ts">
	import { charArt } from '$lib/game/charArt.svelte';
	import type { Figure } from '$lib/game/looks';
	import type { Aura } from '$lib/game/types';
	import CultivatorFigure from './CultivatorFigure.svelte';

	let { tier, aura, figure }: { tier: number; aura: Aura; figure?: Figure } = $props();
	const overlay = $derived(charArt(`fx/aura-${tier}`)); // lazy, never in the initial bundle
	const t = $derived(Math.max(1, Math.min(6, Math.round(tier))));
	const orbit = Array.from({ length: 10 }, (_, i) => i * 36);
</script>

<div class="realm-aura" data-aura-tier={t} style:--aura="var(--color-aura-{aura})" aria-hidden="true">
	{#if overlay}
		<img src={overlay.src} srcset={overlay.srcset} alt="" class="overlay" draggable="false" />
	{:else}
		{#if t >= 6}
			<div class="phap-tuong"><CultivatorFigure {aura} {figure} still class="size-full" /></div>
			<svg class="orbit" viewBox="0 0 200 200">
				{#each orbit as a, i (a)}<circle cx="100" cy="6" r={i % 3 ? 1.4 : 2.6} transform="rotate({a} 100 100)" />{/each}
			</svg>
		{/if}
		{#if t >= 3}
			<div class="nascent"><CultivatorFigure {aura} {figure} crop="bust" still class="size-full" /></div>
		{/if}
		{#if t >= 2}
			<span class="ring"></span>
		{/if}
		{#if t >= 4}
			<svg class="sparks" viewBox="0 0 200 200">
				<path d="M40 70 L52 82 L46 84 L58 98" />
				<path d="M160 64 L148 78 L155 80 L142 96" />
			</svg>
		{/if}
		{#if t >= 5}
			<svg class="clouds" viewBox="0 0 200 60">
				<path d="M20 40 q10 -16 24 -6 q8 -14 22 -4 q12 -10 22 4 q14 -6 16 8 Z" />
				<path d="M96 46 q10 -14 22 -4 q10 -12 22 0 q12 -8 20 6 q12 -2 14 8 Z" />
			</svg>
		{/if}
	{/if}
</div>

<style>
	.realm-aura {
		position: absolute;
		inset: -14% -10% -6%;
		pointer-events: none;
	}
	.overlay {
		position: absolute;
		inset: 0;
		width: 100%;
		height: 100%;
		object-fit: contain;
		opacity: 0.85;
	}
	.ring {
		position: absolute;
		left: 18%;
		right: 18%;
		bottom: 4%;
		height: 12%;
		border-radius: 50%;
		border: 2px solid color-mix(in oklab, var(--aura) 70%, transparent);
		box-shadow: 0 0 14px color-mix(in oklab, var(--aura) 55%, transparent);
	}
	.nascent {
		position: absolute;
		left: 38%;
		top: -6%;
		width: 24%;
		aspect-ratio: 1;
		opacity: 0.22;
		filter: blur(0.5px) drop-shadow(0 0 10px var(--aura));
	}
	.phap-tuong {
		position: absolute;
		inset: -30% -20% 10%;
		opacity: 0.1;
		filter: drop-shadow(0 0 24px var(--aura));
	}
	.orbit {
		position: absolute;
		inset: 2%;
		fill: color-mix(in oklab, var(--aura) 60%, var(--color-mist-100));
	}
	.sparks {
		position: absolute;
		inset: 0;
		fill: none;
		stroke: color-mix(in oklab, var(--color-spirit-cyan) 70%, var(--color-mist-100));
		stroke-width: 2;
		stroke-linejoin: bevel;
		opacity: 0.55;
	}
	.clouds {
		position: absolute;
		left: 4%;
		right: 4%;
		bottom: -2%;
		height: 18%;
		fill: color-mix(in oklab, var(--color-mist-100) 30%, transparent);
		stroke: color-mix(in oklab, var(--aura) 50%, transparent);
		stroke-width: 1;
	}
	/* motion only at full (lite: the slow orbit + breathing ring); off / reduced motion: the static state above */
	:global(:root[data-fx='full']) .ring,
	:global(:root[data-fx='lite']) .ring {
		animation: ring-breathe 4.5s var(--ease-in-out-sine) infinite;
	}
	:global(:root[data-fx='full']) .orbit,
	:global(:root[data-fx='lite']) .orbit {
		animation: orbit 40s linear infinite;
	}
	:global(:root[data-fx='full']) .sparks {
		animation: spark 3.2s steps(1, end) infinite;
	}
	:global(:root[data-fx='full']) .nascent {
		animation: nascent 6s var(--ease-in-out-sine) infinite;
	}
	@keyframes ring-breathe {
		50% {
			opacity: 0.55;
			transform: scale(1.05);
		}
	}
	@keyframes orbit {
		to {
			transform: rotate(360deg);
		}
	}
	@keyframes spark {
		0%,
		88% {
			opacity: 0.15;
		}
		90%,
		94% {
			opacity: 0.85;
		}
	}
	@keyframes nascent {
		50% {
			opacity: 0.32;
			transform: translateY(-4%);
		}
	}
</style>
