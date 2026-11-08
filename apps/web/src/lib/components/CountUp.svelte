<!--
	A number that counts to the SERVER value when it changes (spec §4.4): display only, it always ends exactly on
	`value`. While `hold` is set (orbs / an item still flying to it) it keeps showing the old value. Increases flash
	jade. Effects off (or reduced motion): jumps to the value immediately, no flash. Runs on the shared loop only while
	counting.
-->
<script lang="ts">
	import { untrack } from 'svelte';
	import { gameLoop } from '$lib/engine/loop';
	import { countUpAt, countUpMs } from '$lib/engine/fx/presets';
	import { settings } from '$lib/engine/settings.svelte';

	interface Props {
		value: number;
		format?: (n: number) => string;
		hold?: boolean;
		class?: string;
	}
	let { value, format = (n) => n.toLocaleString('vi-VN'), hold = false, class: cls = '' }: Props = $props();

	// svelte-ignore state_referenced_locally
	let shown = $state(value);
	let flash = $state(0);

	$effect(() => {
		const to = value;
		if (hold) return;
		const from = untrack(() => shown);
		const ms = countUpMs(to - from, settings.fx);
		if (ms === 0) {
			shown = to;
			return;
		}
		if (to > from) untrack(() => flash++);
		let start: number | null = null;
		const off = gameLoop().subscribe((_, now) => {
			start ??= now;
			shown = countUpAt(from, to, now - start, ms);
			if (now - start >= ms) off();
		});
		return off;
	});
</script>

<span class={['count-up tabular-nums', cls]} data-count-up
	>{format(shown)}{#key flash}{#if flash > 0}<span class="glow" aria-hidden="true">{format(shown)}</span>{/if}{/key}</span
>

<style>
	/* jade flash on increase: a jade copy on top fading out (opacity only) */
	.count-up {
		position: relative;
	}
	.glow {
		position: absolute;
		inset: 0;
		color: var(--color-jade-300);
		text-shadow: 0 0 10px color-mix(in oklab, var(--color-jade-300) 70%, transparent);
		opacity: 0;
		animation: jade-up calc(var(--dur-slow) * 2) var(--ease-qi) 1;
		pointer-events: none;
	}
	:global(:root[data-fx='off']) .glow {
		display: none;
	}
	@keyframes jade-up {
		0%,
		50% {
			opacity: 1;
		}
		100% {
			opacity: 0;
		}
	}
</style>
