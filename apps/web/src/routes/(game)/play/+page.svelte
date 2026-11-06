<!-- Thế Giới: the world map is the home screen; places open the other screens. -->
<script lang="ts">
	import Icon from '$lib/components/Icon.svelte';
	import WorldMap from '$lib/components/WorldMap.svelte';
	import { useIdleClock } from '$lib/game/idle.svelte';
	import { game } from '$lib/game/store.svelte';

	const ch = $derived(game.character);
	const clock = useIdleClock();
</script>

<svelte:head><title>Thế Giới · Tiên Đạo</title></svelte:head>

<div class="relative h-full overflow-y-auto sm:overflow-hidden">
	<WorldMap />

	{#if ch}
		<aside class="status panel mt-3 flex items-center gap-3 p-3 sm:absolute sm:bottom-4 sm:left-4 sm:mt-0 sm:w-80">
			<div class="min-w-0 flex-1">
				<p class="eyebrow">{clock.isFull ? 'Linh khí đã đầy' : 'Đang bế quan'}</p>
				<p class="mt-0.5 truncate font-display text-lg font-bold text-mist-100">{game.catalogue?.stageLabel(ch.stageId)}</p>
				<p class="text-xs tabular-nums" class:text-jade-300={!clock.isFull} class:text-gold-300={clock.isFull}>
					Đã tích lũy {clock.elapsedLabel} / {Math.round(clock.capSeconds / 3600)} canh giờ
				</p>
			</div>
			<button class="btn btn-jade h-11 px-3" disabled={game.busy} onclick={game.settle}>
				<Icon name="lotus" size={18} /> Thu hoạch
			</button>
		</aside>
	{/if}
</div>

<style>
	.status {
		backdrop-filter: blur(4px);
	}
</style>
