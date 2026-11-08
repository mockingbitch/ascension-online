<!--
	Thế Giới: the world map is a SCENE (SceneLayer + WorldView draw the map, plaques and ambient life from the world
	def). This page only holds UI: the slot the map fills, the phone place list and the cultivation status card.
-->
<script lang="ts">
	import HelpButton from '$lib/components/HelpButton.svelte';
	import MapPlaque from '$lib/components/MapPlaque.svelte';
	import SceneSlot from '$lib/engine/scene/SceneSlot.svelte';
	import { useIdleClock } from '$lib/game/idle.svelte';
	import { mapLocations } from '$lib/game/nav';
	import { usePrimaryAction } from '$lib/game/primary.svelte';
	import { game } from '$lib/game/store.svelte';

	const ch = $derived(game.character);
	const clock = useIdleClock();
	const ratio = $derived(clock.capSeconds ? clock.elapsedSeconds / clock.capSeconds : 0);
	usePrimaryAction(() => ({
		id: 'settle',
		label: 'Thu nhận',
		icon: 'lotus',
		hint: clock.isFull ? 'Đã đầy' : clock.elapsedLabel,
		ready: !!ch && clock.shown > ch.cultivationMilli,
		disabled: game.busy,
		busy: game.busy,
		progress: ratio,
		tone: 'jade',
		run: game.settle
	}));
</script>

<svelte:head><title>Thế Giới · Tiên Đạo</title></svelte:head>

<!-- phones: map banner → place grid (rows share the height left, no dead space) → status card; one scroller -->
<div class="world-page relative flex h-full flex-col overflow-y-auto overscroll-contain sm:block sm:overflow-hidden">
	<SceneSlot class="slot" passthrough />

	<!-- phones: the map is a banner, places are a list (thumb-friendly) -->
	<ul class="places ui mt-2 grid grid-cols-2 gap-2 sm:hidden">
		{#each mapLocations as l (l.screen)}
			<li><MapPlaque screen={l.screen} title={l.title} subtitle={l.subtitle} compact /></li>
		{/each}
	</ul>

	{#if ch}
		<aside data-scene-avoid class="ui status panel mt-2 flex shrink-0 items-center gap-3 p-3 sm:absolute sm:top-3 sm:left-3 sm:mt-0 sm:w-80">
			<div class="min-w-0 flex-1">
				<p class="eyebrow">{clock.isFull ? 'Linh khí đã đầy' : 'Đang bế quan'}</p>
				<p class="mt-0.5 truncate font-display text-lg font-bold text-mist-100">{game.catalogue?.stageLabel(ch.stageId)}</p>
				<p class="text-xs tabular-nums" class:text-jade-300={!clock.isFull} class:text-gold-300={clock.isFull}>
					<span class="sr-only">Đã tích lũy</span> {clock.elapsedLabel} / {Math.round(clock.capSeconds / 3600)} canh giờ
				</p>
			</div>
			<HelpButton topic="world" />
		</aside>
	{/if}
</div>

<style>
	/* phones: a 16:9 banner; wider: the whole stage (the map is contain-fitted inside by the renderer) */
	.world-page :global(.slot) {
		flex: none;
		width: 100%;
		aspect-ratio: 16 / 9;
	}
	@media (min-width: 640px) {
		.world-page :global(.slot) {
			position: absolute;
			inset: 0;
			aspect-ratio: auto;
		}
	}
	.status {
		backdrop-filter: blur(4px);
	}
	/* landscape phones: a one-line card (the map needs the height for its plaques) */
	@media (min-width: 640px) and (max-height: 560px) {
		.status {
			padding: 0.25rem 0.25rem 0.25rem 0.75rem;
			width: auto;
			max-width: 18rem;
		}
		.status :global(.eyebrow) {
			display: none;
		}
	}
	.places {
		flex: 1 0 auto;
		grid-auto-rows: minmax(var(--tap), 1fr);
	}
	.places li > :global(.plaque) {
		height: 100%;
	}
</style>
