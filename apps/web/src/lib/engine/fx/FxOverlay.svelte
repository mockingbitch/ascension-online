<!--
	DOM part of the FxLayer (lazy): "+X tu vi" floats, reward icons popping from their source and flying into the bag
	(rare = rarity aura + rotating rays), light rings, and the world-announcement scroll that unrolls mid-screen
	(2.5 s, skippable by click/tap or "Bỏ qua"). Web Animations on transform/opacity only. Off: floats show static
	and leave after their time; nothing moves (the director plans no flights/rings/scrolls when off).
-->
<script lang="ts">
	import Icon from '$lib/components/Icon.svelte';
	import { settings } from '../settings.svelte';
	import { fxOverlay, type OverlayItem } from './overlay.svelte';

	const SCROLL_MS = 2500;
	const FLOAT_MS = 1400;

	const q = $derived(settings.fx);
	const scroll = $derived(fxOverlay.scrolls[0]);

	function done(id: number, a: Animation | null, ms: number) {
		if (a) a.finished.then(() => fxOverlay.remove(id), () => fxOverlay.remove(id));
		else {
			const t = setTimeout(() => fxOverlay.remove(id), ms);
			return () => clearTimeout(t);
		}
	}

	/** One attachment per item: plays its animation, removes the item at the end. */
	function play(item: OverlayItem) {
		return (el: HTMLElement) => {
			const off = q === 'off';
			if (item.kind === 'float') {
				const a = off
					? null
					: el.animate(
							[
								{ opacity: 0, transform: 'translate(-50%, -30%) scale(0.85)' },
								{ opacity: 1, transform: 'translate(-50%, -110%) scale(1)', offset: 0.2 },
								{ opacity: 0, transform: 'translate(-50%, -260%) scale(1)' }
							],
							{ duration: q === 'lite' ? 900 : FLOAT_MS, easing: 'cubic-bezier(0.22, 1, 0.36, 1)', fill: 'forwards' }
						);
				return done(item.id, a, FLOAT_MS);
			}
			if (item.kind === 'ring') {
				const a = el.animate(
					[
						{ opacity: 0.9, transform: 'translate(-50%, -50%) scale(0.3)' },
						{ opacity: 0, transform: `translate(-50%, -50%) scale(${q === 'lite' ? 2.2 : 4.2})` }
					],
					{ duration: 700, easing: 'cubic-bezier(0.22, 1, 0.36, 1)', fill: 'forwards' }
				);
				return done(item.id, a, 0);
			}
			// fly: pop at the source (0.4 → 1.15 → 1), then a curved hop into the bag, shrinking at the end
			const dx = item.to.x - item.from.x;
			const dy = item.to.y - item.from.y;
			const lift = -Math.min(160, 60 + Math.abs(dx) * 0.25);
			const pop = 0.28;
			const a = el.animate(
				[
					{ opacity: 0, transform: 'translate(-50%, -50%) scale(0.4)' },
					{ opacity: 1, transform: 'translate(-50%, calc(-50% - 18px)) scale(1.15)', offset: pop * 0.6 },
					{ opacity: 1, transform: 'translate(-50%, calc(-50% - 14px)) scale(1)', offset: pop },
					{ opacity: 1, transform: `translate(calc(-50% + ${dx * 0.5}px), calc(-50% + ${dy * 0.5 + lift}px)) scale(0.85)`, offset: 0.68 },
					{ opacity: 0.2, transform: `translate(calc(-50% + ${dx}px), calc(-50% + ${dy}px)) scale(0.35)` }
				],
				{ duration: item.ms, delay: item.delay, easing: 'cubic-bezier(0.37, 0, 0.63, 1)', fill: 'both' }
			);
			return done(item.id, a, 0);
		};
	}

	// the announcement scroll: unroll (scaleX) in, hold, fade; any click skips
	function unroll(el: HTMLElement) {
		const lite = q === 'lite';
		const a = el.animate(
			lite
				? [{ opacity: 0 }, { opacity: 1, offset: 0.12 }, { opacity: 1, offset: 0.88 }, { opacity: 0 }]
				: [
						{ opacity: 0, transform: 'scaleX(0.06)' },
						{ opacity: 1, transform: 'scaleX(0.06)', offset: 0.06 },
						{ opacity: 1, transform: 'scaleX(1)', offset: 0.26 },
						{ opacity: 1, transform: 'scaleX(1)', offset: 0.88 },
						{ opacity: 0, transform: 'scaleX(1) translateY(-8px)' }
					],
			{ duration: SCROLL_MS, easing: 'cubic-bezier(0.22, 1, 0.36, 1)', fill: 'forwards' }
		);
		a.finished.then(() => fxOverlay.nextScroll(), () => {});
		return () => a.cancel();
	}
</script>

{#each fxOverlay.items as item (item.id)}
	{#if item.kind === 'float'}
		<span class="float" class:is-gold={item.tone === 'gold'} style:left="{item.at.x}px" style:top="{item.at.y}px" {@attach play(item)}>{item.text}</span>
	{:else if item.kind === 'ring'}
		<span class="ring" class:is-gold={item.tone === 'gold'} style:left="{item.at.x}px" style:top="{item.at.y}px" {@attach play(item)}></span>
	{:else}
		<span
			class="fly"
			class:is-rare={item.rare}
			style:left="{item.from.x}px"
			style:top="{item.from.y}px"
			style:--r={item.rarity ? `var(--color-rarity-${item.rarity})` : 'var(--color-gold-300)'}
			data-fx-fly={item.rare ? 'rare' : 'common'}
			{@attach play(item)}
		>
			{#if item.rare}<span class="rays"></span>{/if}
			<span class="chip">
				{#if item.art}<img src={item.art} alt="" draggable="false" />{:else}<Icon name={item.icon} size={22} />{/if}
			</span>
			<span class="sr-only">{item.label}</span>
		</span>
	{/if}
{/each}

{#if scroll}
	{#key scroll.id}
		<div class="scroll-wrap" data-fx-scroll>
			<button type="button" class="scroll" {@attach unroll} onclick={() => fxOverlay.nextScroll()} aria-label="Bỏ qua thông báo">
				<span class="rod l" aria-hidden="true"></span>
				<span class="paper">
					<span class="title">{scroll.title}</span>
					<span class="text">{scroll.text}</span>
					<span class="skip" aria-hidden="true">Chạm để bỏ qua</span>
				</span>
				<span class="rod r" aria-hidden="true"></span>
			</button>
		</div>
	{/key}
{/if}

<style>
	.float,
	.ring,
	.fly {
		position: fixed;
		pointer-events: none;
	}
	.float {
		transform: translate(-50%, -110%);
		font-family: var(--font-display);
		font-size: 1.5rem;
		font-weight: 700;
		white-space: nowrap;
		color: var(--color-jade-300);
		text-shadow:
			0 0 12px var(--color-jade-500),
			0 1px 2px var(--color-ink-950);
	}
	.float.is-gold {
		color: var(--color-gold-300);
		text-shadow: 0 0 12px var(--color-gold-500);
	}
	.ring {
		width: 5rem;
		height: 5rem;
		border-radius: 9999px;
		border: 2px solid var(--color-jade-300);
		box-shadow: 0 0 18px var(--color-jade-300);
		transform: translate(-50%, -50%);
	}
	.ring.is-gold {
		border-color: var(--color-gold-300);
		box-shadow: 0 0 18px var(--color-gold-300);
	}
	.fly {
		width: 3rem;
		height: 3rem;
		transform: translate(-50%, -50%);
		opacity: 0;
	}
	.chip {
		position: absolute;
		inset: 0;
		display: grid;
		place-items: center;
		border-radius: 9999px;
		color: var(--r);
		border: 1px solid color-mix(in oklab, var(--r) 60%, transparent);
		background: radial-gradient(circle at 50% 35%, var(--color-ink-700), var(--color-ink-950));
		box-shadow: 0 0 10px color-mix(in oklab, var(--r) 45%, transparent);
	}
	.chip img {
		width: 80%;
		height: 80%;
		object-fit: contain;
	}
	.is-rare .chip {
		box-shadow:
			0 0 0 2px color-mix(in oklab, var(--r) 70%, transparent),
			0 0 22px var(--r);
	}
	.rays {
		position: absolute;
		inset: -70%;
		border-radius: 9999px;
		background: repeating-conic-gradient(color-mix(in oklab, var(--r) 55%, transparent) 0 10deg, transparent 10deg 30deg);
		mask-image: radial-gradient(circle, var(--color-ink-950) 20%, transparent 68%);
		animation: rays 2.4s linear infinite;
	}
	@keyframes rays {
		to {
			transform: rotate(360deg);
		}
	}

	/* ── announcement scroll ── */
	.scroll-wrap {
		position: fixed;
		inset: 0;
		display: grid;
		place-items: center;
		padding: 1rem;
		pointer-events: none;
	}
	.scroll {
		pointer-events: auto;
		display: flex;
		align-items: stretch;
		width: min(40rem, 100%);
		transform-origin: 50% 50%;
		opacity: 0;
		cursor: pointer;
		filter: drop-shadow(0 12px 28px var(--color-ink-950));
	}
	.rod {
		flex: none;
		width: 0.9rem;
		border-radius: 9999px;
		background: linear-gradient(90deg, var(--color-gold-700), var(--color-gold-300), var(--color-gold-700));
	}
	.paper {
		display: flex;
		flex: 1;
		flex-direction: column;
		gap: 0.3rem;
		padding: 1rem 1.25rem;
		text-align: center;
		color: var(--color-ink-950);
		background:
			var(--grain),
			radial-gradient(ellipse at 50% 50%, var(--color-mist-100), color-mix(in oklab, var(--color-mist-300) 85%, var(--color-gold-300)));
	}
	.title {
		font-size: 0.72rem;
		font-weight: 700;
		letter-spacing: 0.18em;
		text-transform: uppercase;
		color: var(--color-cinnabar-500);
	}
	.text {
		font-family: var(--font-display);
		font-size: clamp(1.1rem, 3.4vw, 1.6rem);
		font-weight: 700;
		line-height: 1.3;
	}
	.skip {
		font-size: 0.68rem;
		color: var(--color-ink-700);
	}
</style>
