<!--
	A character name styled by realm tier (decisions 2026-11-10, Tử Tiêu Cung): realms.yaml visual.fx_tier 1–6 escalates
	1 plain mist · 2 jade · 3 azure glow · 4 violet + a faint lightning shimmer · 5 animated gold gradient ·
	6 celestial prismatic + star glints; the Vu path uses its own ember → ancestral tokens (--color-rname-vu-*).
	Fairness rule (game/realmName.ts): the realm owns the name; a purchased chat aura (`nameFx`) is only a thin static
	underline in its colours — it never fills, glows or animates, so it can't look stronger than a higher realm.
	Motion: tiers 4–6 animate at effects full (lite: slower), static at off / reduced motion (the end state is the
	information: the name is fully readable, every colour stop ≥ 4.5:1 on ink).
-->
<script lang="ts">
	import { nameFxClass } from '$lib/game/cosmetics.svelte';
	import { auraAccent, realmNameStyle } from '$lib/game/realmName';
	import { game } from '$lib/game/store.svelte';
	import type { PathId } from '$lib/game/types';

	interface Props {
		name: string;
		/** stage id → tier + path from the catalogue (or pass tier/path directly) */
		stageId?: string;
		tier?: number;
		path?: PathId;
		/** chat aura cosmetic id (cosmetics.yaml chat_aura:*) — shown as a static underline accent only */
		nameFx?: string;
		class?: string;
	}
	let { name, stageId, tier, path, nameFx, class: cls = '' }: Props = $props();

	const realm = $derived(stageId ? game.catalogue?.realmOf(stageId) : undefined);
	const st = $derived(realmNameStyle(tier ?? realm?.visual.fxTier, path ?? realm?.path ?? 'tien'));
	const accent = $derived(auraAccent(nameFxClass(nameFx) || undefined));
	const v = (t: string | undefined) => (t ? `var(${t})` : undefined);
</script>

<span
	class={['rname', cls]}
	data-tier={st.tier}
	data-path={st.path}
	data-effect={st.effect}
	data-accent={accent ? '' : undefined}
	style:--rn-a={v(st.stops[0])}
	style:--rn-b={v(st.stops[1] ?? st.stops[0])}
	style:--rn-c={v(st.stops[2] ?? st.stops[1] ?? st.stops[0])}
	style:--rn-glow="{st.glow}px"
	style:--rn-acc-a={accent ? v(accent.stops[0]) : undefined}
	style:--rn-acc-b={accent ? v(accent.stops[1]) : undefined}
	style:font-weight={st.weight}
	><span class="txt">{name}</span>{#if st.glints}<span class="glints" aria-hidden="true"><i></i><i></i><i></i></span>{/if}</span
>

<style>
	.rname {
		position: relative;
		display: inline-block;
		max-width: 100%;
		white-space: nowrap;
		vertical-align: baseline;
	}
	.txt {
		color: var(--rn-a);
	}
	/* 1 plain mist: the faintest ink halo only for legibility over the painting */
	[data-tier='1'] .txt {
		text-shadow: 0 1px 2px var(--color-ink-950);
	}
	/* 2–3: a glow in the tier colour (3 wider) */
	[data-effect='glow'] .txt {
		text-shadow:
			0 1px 2px var(--color-ink-950),
			0 0 var(--rn-glow) color-mix(in oklab, var(--rn-a) 55%, transparent);
	}
	/* 4–6: gradient text (every stop is readable on ink) + a drop-shadow glow */
	[data-effect='shimmer'] .txt,
	[data-effect='flow'] .txt,
	[data-effect='prismatic'] .txt {
		-webkit-background-clip: text;
		background-clip: text;
		color: transparent;
		filter: drop-shadow(0 0 calc(var(--rn-glow) / 2) color-mix(in oklab, var(--rn-a) 60%, transparent))
			drop-shadow(0 1px 1px var(--color-ink-950));
	}
	/* 4: violet with a narrow lightning band (static: resting across the name; full: it sweeps now and then) */
	[data-effect='shimmer'] .txt {
		background-image: linear-gradient(100deg, var(--rn-a) 0 42%, var(--rn-b) 50%, var(--rn-a) 58% 100%);
		background-size: 260% 100%;
		background-position: 38% 0;
	}
	/* 5: gold gradient flowing */
	[data-effect='flow'] .txt {
		background-image: linear-gradient(90deg, var(--rn-a), var(--rn-b), var(--rn-c), var(--rn-a));
		background-size: 200% 100%;
	}
	/* 6: celestial prismatic, wider and brighter, with star glints */
	[data-effect='prismatic'] .txt {
		background-image: linear-gradient(90deg, var(--rn-a), var(--rn-b), var(--rn-c), var(--rn-a), var(--rn-b));
		background-size: 250% 100%;
		letter-spacing: 0.02em;
		filter: drop-shadow(0 0 calc(var(--rn-glow) / 2) color-mix(in oklab, var(--rn-b) 65%, transparent))
			drop-shadow(0 0 var(--rn-glow) color-mix(in oklab, var(--rn-c) 40%, transparent)) drop-shadow(0 1px 1px var(--color-ink-950));
	}
	/* 6: a soft celestial halo behind the name */
	[data-effect='prismatic']::before {
		content: '';
		position: absolute;
		inset: -0.15em -0.35em;
		border-radius: 9999px;
		background: radial-gradient(ellipse at 50% 55%, color-mix(in oklab, var(--rn-b) 22%, transparent), transparent 70%);
		pointer-events: none;
	}
	.glints {
		position: absolute;
		inset: -0.4em -0.5em;
		pointer-events: none;
	}
	.glints i {
		position: absolute;
		width: 0.48em;
		aspect-ratio: 1;
		background: var(--rn-a);
		clip-path: polygon(50% 0, 60% 40%, 100% 50%, 60% 60%, 50% 100%, 40% 60%, 0 50%, 40% 40%);
		filter: drop-shadow(0 0 2px var(--rn-b));
	}
	.glints i:nth-child(1) {
		left: 0;
		top: 0;
	}
	.glints i:nth-child(2) {
		right: 10%;
		top: -8%;
		width: 0.32em;
	}
	.glints i:nth-child(3) {
		right: 0;
		bottom: 4%;
	}
	/* chat aura accent: a thin static underline in the aura's colours — never animated, never glowing */
	[data-accent] .txt {
		text-decoration: none;
	}
	[data-accent]::after {
		content: '';
		position: absolute;
		left: 6%;
		right: 6%;
		bottom: -0.12em;
		height: 1.5px;
		border-radius: 1px;
		background: linear-gradient(90deg, var(--rn-acc-a), var(--rn-acc-b));
		opacity: 0.75;
	}

	/* motion: only at effects full / lite; off and reduced motion keep the static end state above */
	:global(:root[data-fx='full']) [data-effect='shimmer'] .txt {
		animation: rn-sweep 5.5s var(--ease-in-out-sine) infinite;
	}
	:global(:root[data-fx='full']) [data-effect='flow'] .txt {
		animation: rn-flow 5s linear infinite;
	}
	:global(:root[data-fx='full']) [data-effect='prismatic'] .txt {
		animation: rn-flow 7s linear infinite;
	}
	:global(:root[data-fx='lite']) [data-effect='flow'] .txt,
	:global(:root[data-fx='lite']) [data-effect='prismatic'] .txt {
		animation: rn-flow 12s linear infinite;
	}
	:global(:root[data-fx='full']) .glints i {
		animation: rn-twinkle 2.6s var(--ease-in-out-sine) infinite;
	}
	:global(:root[data-fx='full']) .glints i:nth-child(2) {
		animation-delay: -0.9s;
	}
	:global(:root[data-fx='full']) .glints i:nth-child(3) {
		animation-delay: -1.7s;
	}
	@keyframes rn-sweep {
		0%,
		70% {
			background-position: 100% 0;
		}
		100% {
			background-position: 0 0;
		}
	}
	@keyframes rn-flow {
		to {
			background-position: -200% 0;
		}
	}
	@keyframes rn-twinkle {
		50% {
			opacity: 0.25;
			transform: scale(0.6);
		}
	}
</style>
