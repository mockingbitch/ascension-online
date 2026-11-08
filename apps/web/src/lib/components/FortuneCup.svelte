<!--
	Gieo quẻ stick cup (spec §4.5). While the draw request is in flight the cup shakes (a loop — the result is not known
	yet); when the store emits `fortune` (after the server answered) one stick pops out and its slip flips to show the
	outcome name. Display only: the outcome is the server's; the odds stay in the table next to it.
	Full: shake + pop + flip · Lite: short shake, slip fades in · Off: static cup, slip shown at once.
-->
<script lang="ts">
	import { fxBus } from '$lib/engine/fx/bus';
	import { settings } from '$lib/engine/settings.svelte';

	let { shaking = false }: { shaking?: boolean } = $props();
	let slip = $state<{ name: string; lucky: boolean; n: number } | null>(null);
	let n = 0;

	$effect(() => fxBus.on('fortune', (e) => (slip = { name: e.name, lucky: e.lucky, n: ++n })));

	// pop the stick out of the cup, then flip the slip (WAAPI, transform/opacity)
	function reveal(el: HTMLElement) {
		const q = settings.fx;
		if (q === 'off') return;
		const stick = el.querySelector<HTMLElement>('.stick');
		const face = el.querySelector<HTMLElement>('.slip');
		const anims: Animation[] = [];
		if (q === 'full' && stick)
			anims.push(
				stick.animate(
					[
						{ transform: 'translate(-50%, 40px) rotate(0deg)', opacity: 0 },
						{ transform: 'translate(-50%, -18px) rotate(-8deg)', opacity: 1, offset: 0.55 },
						{ transform: 'translate(-50%, 0) rotate(0deg)', opacity: 1 }
					],
					{ duration: 520, easing: 'cubic-bezier(0.34, 1.56, 0.64, 1)', fill: 'both' }
				)
			);
		if (face)
			anims.push(
				face.animate(
					q === 'full'
						? [
								{ transform: 'rotateY(90deg)', opacity: 0 },
								{ transform: 'rotateY(90deg)', opacity: 1, offset: 0.01 },
								{ transform: 'rotateY(0deg)', opacity: 1 }
							]
						: [{ opacity: 0 }, { opacity: 1 }],
					{ duration: q === 'full' ? 420 : 240, delay: q === 'full' ? 480 : 0, easing: 'cubic-bezier(0.22, 1, 0.36, 1)', fill: 'both' }
				)
			);
		return () => anims.forEach((a) => a.cancel());
	}
</script>

<div class="cup-area" data-fortune-cup>
	<div class="cup" class:is-shaking={shaking} aria-hidden="true">
		<svg viewBox="0 0 120 120" class="size-full" fill="none" stroke-linecap="round">
			{#each [-18, -10, -3, 4, 11, 18] as dx, i (i)}
				<line x1={60 + dx} y1="58" x2={60 + dx * 1.35} y2={22 + (i % 3) * 5} stroke="var(--color-gold-500)" stroke-width="3.2" />
			{/each}
			<path d="M32 50h56l-7 58H39Z" fill="var(--color-ink-700)" stroke="var(--color-gold-500)" stroke-width="2" />
			<path d="M36 64h48M38 94h44" stroke="var(--color-gold-700)" stroke-width="1.5" />
			<circle cx="60" cy="79" r="7" stroke="var(--color-cinnabar-300)" stroke-width="1.6" />
		</svg>
	</div>
	{#if slip}
		{#key slip.n}
			<div class="reveal" {@attach reveal}>
				<span class="stick" aria-hidden="true"></span>
				<p class="slip" class:is-lucky={slip.lucky} data-fortune-slip>{slip.name}</p>
			</div>
		{/key}
	{/if}
</div>

<style>
	.cup-area {
		position: relative;
		display: flex;
		flex-direction: column;
		align-items: center;
		perspective: 600px;
	}
	.cup {
		width: 6.5rem;
		height: 6.5rem;
		transform-origin: 50% 90%;
	}
	:global(:root[data-fx='full']) .is-shaking {
		animation: shake 380ms var(--ease-in-out-sine) infinite;
	}
	:global(:root[data-fx='lite']) .is-shaking {
		animation: shake 380ms var(--ease-in-out-sine) 2;
	}
	@keyframes shake {
		25% {
			transform: rotate(-9deg) translateY(-2px);
		}
		75% {
			transform: rotate(9deg) translateY(-2px);
		}
	}
	.reveal {
		position: relative;
		display: flex;
		flex-direction: column;
		align-items: center;
		margin-top: -0.25rem;
	}
	.stick {
		position: absolute;
		left: 50%;
		bottom: 100%;
		width: 6px;
		height: 3.2rem;
		border-radius: 3px;
		background: linear-gradient(180deg, var(--color-cinnabar-300), var(--color-gold-500) 30%);
		transform: translate(-50%, 0);
	}
	.slip {
		min-width: 7rem;
		padding: 0.35rem 0.9rem;
		font-family: var(--font-display);
		font-size: 1.35rem;
		font-weight: 700;
		text-align: center;
		color: var(--color-ink-950);
		background: linear-gradient(180deg, var(--color-mist-100), var(--color-mist-300));
		border: 1px solid var(--color-gold-500);
		backface-visibility: hidden;
	}
	.slip.is-lucky {
		box-shadow: 0 0 16px -2px var(--color-gold-300);
	}
</style>
