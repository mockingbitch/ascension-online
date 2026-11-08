<!--
	Scene transition cover (spec §4.2): two banks of ink cloud close over the stage, the scene swaps underneath, then
	they part again — ≈ --dur-scene in total. Web Animations on transform/opacity only (compositor), no rAF.
	Lite = a short plain ink fade; off = an instant swap. Registers itself as the stage's WipeDriver.
-->
<script lang="ts">
	import type { TransitionKind } from './routes';
	import type { SceneStage } from './stage.svelte';

	let { stage }: { stage: SceneStage } = $props();

	let left = $state<HTMLDivElement>();
	let right = $state<HTMLDivElement>();
	let veil = $state<HTMLDivElement>();
	let active = $state(false);

	const easing = () => getComputedStyle(document.documentElement).getPropertyValue('--ease-in-out-sine').trim() || 'ease-in-out';

	function run(el: HTMLElement | undefined, frames: Keyframe[], ms: number): Promise<void> {
		if (!el || typeof el.animate !== 'function') return Promise.resolve();
		const a = el.animate(frames, { duration: ms, easing: easing(), fill: 'forwards' });
		return a.finished.then(
			() => undefined,
			() => undefined
		);
	}

	const OUT_L = 'translate3d(-105%, 0, 0) scale(1.08)';
	const OUT_R = 'translate3d(105%, 0, 0) scale(1.08)';
	const IN = 'translate3d(0, 0, 0) scale(1)';

	$effect(() => {
		stage.wipe = {
			async cover(kind: TransitionKind, ms: number) {
				if (kind === 'instant') return;
				active = true;
				if (kind === 'fade') return run(veil, [{ opacity: 0 }, { opacity: 1 }], ms);
				await Promise.all([
					run(left, [{ transform: OUT_L }, { transform: IN }], ms),
					run(right, [{ transform: OUT_R }, { transform: IN }], ms),
					run(veil, [{ opacity: 0 }, { opacity: 0.85, offset: 0.7 }, { opacity: 1 }], ms)
				]);
			},
			async reveal(kind: TransitionKind, ms: number) {
				if (kind === 'instant') return;
				if (kind === 'fade') await run(veil, [{ opacity: 1 }, { opacity: 0 }], ms);
				else
					await Promise.all([
						// the banks keep drifting apart the way they came (not back), the veil lifts first
						run(left, [{ transform: IN }, { transform: OUT_L }], ms),
						run(right, [{ transform: IN }, { transform: OUT_R }], ms),
						run(veil, [{ opacity: 1 }, { opacity: 0, offset: 0.45 }, { opacity: 0 }], ms)
					]);
				for (const el of [left, right, veil]) for (const a of el?.getAnimations() ?? []) a.cancel();
				active = false;
			}
		};
		return () => {
			stage.wipe = null;
		};
	});
</script>

<div class="wipe" class:is-active={active} aria-hidden="true" data-phase={stage.phase}>
	<div bind:this={veil} class="veil"></div>
	<div bind:this={left} class="bank is-left"><span class="rim"></span><span class="body"></span></div>
	<div bind:this={right} class="bank is-right"><span class="rim"></span><span class="body"></span></div>
</div>

<style>
	.wipe {
		position: absolute;
		inset: 0;
		z-index: 40;
		overflow: hidden;
		pointer-events: none;
		visibility: hidden;
	}
	.wipe.is-active {
		visibility: visible;
		pointer-events: auto; /* nothing underneath is clickable mid-transition */
	}
	.veil {
		position: absolute;
		inset: 0;
		opacity: 0;
		background: radial-gradient(ellipse at 50% 50%, var(--color-ink-850), var(--color-ink-950) 70%);
	}
	/* A bank of ink cloud: a body with a scalloped (cloud) leading edge and a paler mist rim just ahead of it. */
	.bank {
		position: absolute;
		top: -10%;
		bottom: -10%;
		width: 64%;
		will-change: transform;
	}
	.bank.is-left {
		left: 0;
		transform: translate3d(-105%, 0, 0);
	}
	.bank.is-right {
		right: 0;
		transform: translate3d(105%, 0, 0);
	}
	.rim,
	.body {
		position: absolute;
		inset: 0;
		--bump: 96px;
		mask:
			linear-gradient(var(--color-ink-950), var(--color-ink-950)) left / calc(100% - var(--bump) / 2) 100% no-repeat,
			radial-gradient(circle at 0 50%, var(--color-ink-950) 69%, transparent 71%) right / var(--bump) var(--bump) repeat-y;
	}
	/* mirrored bank: `scale` composes with the animated `transform`, so WAAPI can't undo the flip */
	.is-right .rim,
	.is-right .body {
		scale: -1 1;
	}
	.rim {
		transform: translateX(14px);
		--bump: 120px;
		background: linear-gradient(90deg, transparent, color-mix(in oklab, var(--color-mist-300) 22%, transparent));
	}
	.body {
		background:
			radial-gradient(ellipse 60% 30% at 70% 30%, color-mix(in oklab, var(--color-ink-700) 50%, transparent), transparent 70%),
			radial-gradient(ellipse 50% 25% at 40% 75%, color-mix(in oklab, var(--color-ink-700) 40%, transparent), transparent 70%),
			linear-gradient(90deg, var(--color-ink-950), var(--color-ink-900));
	}
</style>
