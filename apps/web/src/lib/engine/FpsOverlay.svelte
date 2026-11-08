<!-- Dev/QA overlay (?fps=1): frames per second of the shared rAF loop, refreshed twice a second without re-rendering.
     A manual popover (top layer) so it stays above windows; never takes the pointer. -->
<script lang="ts">
	import { gameLoop } from './loop';

	let el = $state<HTMLSpanElement>();
	$effect(() => {
		try {
			el?.showPopover?.();
		} catch {
			/* popover unsupported: fixed + --z-boot */
		}
	});

	$effect(() => {
		const loop = gameLoop();
		let acc = 0;
		return loop.subscribe((dt) => {
			acc += dt;
			if (acc < 500 || !el) return;
			acc = 0;
			el.textContent = `${loop.fps} fps`;
		});
	});
</script>

<span bind:this={el} class="fps tabular-nums" popover="manual" aria-hidden="true">— fps</span>

<style>
	.fps {
		position: fixed;
		left: 6px;
		bottom: 6px;
		inset: auto auto 6px 6px;
		margin: 0;
		border: 0;
		pointer-events: none;
		z-index: var(--z-boot);
		padding: 2px 6px;
		font-size: 0.7rem;
		font-weight: 600;
		color: var(--color-jade-300);
		background: color-mix(in oklab, var(--color-ink-950) 80%, transparent);
		border: 1px solid color-mix(in oklab, var(--color-jade-500) 40%, transparent);
		pointer-events: none;
	}
</style>
