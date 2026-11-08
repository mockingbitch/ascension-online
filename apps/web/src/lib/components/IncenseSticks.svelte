<!--
	Dâng hương (spec §4.5): three incense sticks in a burner, lit when today's offering is made (server state:
	streak.offeredToday), and the streak as flame counters. When the store reports a fresh offering (`settled` with
	source incense, after the server answered) the sticks light one after another and smoke rises from the tips.
	Full: staggered light-up + smoke particles + flickering flames · Lite: light-up, little smoke · Off: static state.
	The streak count is always also in text.
-->
<script lang="ts">
	import { fxBus } from '$lib/engine/fx/bus';
	import { fxLayer } from '$lib/engine/fx/layer';
	import { fxOverlay } from '$lib/engine/fx/overlay.svelte';
	import { smoke } from '$lib/engine/fx/presets';
	import { settings } from '$lib/engine/settings.svelte';

	let { lit = false, streak = 0 }: { lit?: boolean; streak?: number } = $props();

	let root = $state<HTMLElement>();
	let lighting = $state(0);
	const flames = $derived(Math.min(7, Math.max(0, streak)));

	$effect(() => {
		const timers: ReturnType<typeof setTimeout>[] = [];
		const off = fxBus.on('settled', (e) => {
			if (e.source !== 'incense' || settings.fx === 'off') return;
			lighting++;
			const tips = [...(root?.querySelectorAll('.tip') ?? [])];
			const puffs = settings.fx === 'full' ? 3 : 1;
			tips.forEach((tip, i) => {
				for (let k = 0; k < puffs; k++)
					timers.push(
						setTimeout(
							() => {
								if (!tip.isConnected) return;
								const r = tip.getBoundingClientRect();
								fxOverlay.raised++;
								fxLayer.burst(smoke, r.left + r.width / 2, r.top, settings.fx === 'full' ? 3 : 2);
							},
							i * 320 + k * 450 + 200
						)
					);
			});
		});
		return () => {
			off();
			timers.forEach(clearTimeout);
		};
	});
</script>

<div bind:this={root} class="incense" data-incense data-lit={lit}>
	{#key lighting}
		<div class="burner" class:is-lit={lit || lighting > 0} class:is-lighting={lighting > 0} aria-hidden="true">
			{#each [0, 1, 2] as i (i)}
				<span class="stick" style:--i={i}><span class="tip"></span></span>
			{/each}
			<span class="bowl"></span>
		</div>
	{/key}
	{#if flames > 0}
		<span class="flames" role="img" aria-label="Chuỗi {streak} ngày dâng hương">
			{#each Array(flames) as _, i (i)}
				<svg viewBox="0 0 12 16" class="flame" style:--d="{(i * 0.23) % 0.9}s" aria-hidden="true">
					<path d="M6 1c1 3 4 5 4 9a4 4 0 0 1-8 0c0-2 1-3 2-4 0 2 1 3 2 3 0-3-1-5 0-8Z" fill="currentColor" />
				</svg>
			{/each}
			{#if streak > 7}<span class="more tabular-nums">×{streak}</span>{/if}
		</span>
	{/if}
</div>

<style>
	.incense {
		display: inline-flex;
		align-items: flex-end;
		gap: 0.6rem;
	}
	.burner {
		position: relative;
		width: 2.6rem;
		height: 2.6rem;
	}
	.bowl {
		position: absolute;
		left: 0;
		right: 0;
		bottom: 0;
		height: 0.8rem;
		border-radius: 0 0 50% 50%;
		background: linear-gradient(180deg, var(--color-gold-500), var(--color-gold-700));
	}
	.stick {
		position: absolute;
		bottom: 0.6rem;
		left: calc(0.75rem + var(--i) * 0.5rem);
		width: 2px;
		height: 1.7rem;
		background: var(--color-cinnabar-500);
		transform-origin: 50% 100%;
		transform: rotate(calc((var(--i) - 1) * 9deg));
	}
	.tip {
		position: absolute;
		top: -2px;
		left: -2px;
		width: 6px;
		height: 6px;
		border-radius: 9999px;
		background: var(--color-gold-300);
		box-shadow: 0 0 6px 1px var(--color-cinnabar-300);
		opacity: 0;
	}
	.is-lit .tip {
		opacity: 1;
	}
	.is-lighting .tip {
		animation: ignite 500ms var(--ease-qi) calc(var(--i) * 320ms) both;
	}
	@keyframes ignite {
		from {
			opacity: 0;
			transform: scale(0.2);
		}
		60% {
			opacity: 1;
			transform: scale(1.6);
		}
	}
	.flames {
		display: inline-flex;
		align-items: flex-end;
		gap: 1px;
		color: var(--color-gold-300);
	}
	.flame {
		width: 0.7rem;
		height: 0.95rem;
		transform-origin: 50% 100%;
	}
	:global(:root[data-fx='full']) .flame {
		animation: flicker 1.3s var(--ease-in-out-sine) var(--d) infinite alternate;
	}
	@keyframes flicker {
		to {
			transform: scale(0.88, 1.08);
			opacity: 0.8;
		}
	}
	.more {
		margin-left: 0.2rem;
		font-size: 0.72rem;
		font-weight: 700;
	}
</style>
