<!--
	Vân Khê over the game (/play): the active tutorial step in the visual-novel overlay (GuideOverlay — top layer, big,
	backdrop with a spotlight hole). Loaded lazily by TutorialHost when a step starts.
-->
<script lang="ts">
	import { tutorial } from '$lib/game/tutorial.svelte';
	import GuideOverlay from './GuideOverlay.svelte';

	const a = $derived(tutorial.active);
</script>

{#if a}
	<GuideOverlay
		name={tutorial.script?.name ?? 'Vân Khê'}
		step={a.step}
		i={a.i}
		busy={tutorial.busy}
		onnext={() => tutorial.next()}
		onprev={() => tutorial.prev()}
		onskip={() => tutorial.skip()}
		onmute={a.summoned ? undefined : () => void tutorial.muteAll()}
	/>
{/if}
