<!--
	Placeholder a scene page puts where its scene should show (spec §3.1). The SceneLayer (behind the UI) sizes the
	scene box to this element, so pages lay out their UI panels and the scene fills the space left. Children render
	inside (scene-relative UI such as subtitles or "+N tu vi" floats). Transparent to the pointer: the scene's own
	hotspots under it stay clickable.
-->
<script lang="ts">
	import type { Snippet } from 'svelte';
	import { sceneStage } from './stage.svelte';

	let { class: cls = '', children }: { class?: string; children?: Snippet } = $props();
	let el = $state<HTMLDivElement>();

	$effect(() => {
		const me = el;
		if (!me) return;
		sceneStage.slot = me;
		return () => {
			if (sceneStage.slot === me) sceneStage.slot = null;
		};
	});
</script>

<div bind:this={el} class={['scene-slot', cls]} data-scene-slot>{@render children?.()}</div>

<style>
	.scene-slot {
		position: relative;
		pointer-events: none;
	}
</style>
