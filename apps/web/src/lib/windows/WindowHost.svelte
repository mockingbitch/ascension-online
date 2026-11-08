<!--
	The window stack over the scene (UiLayer): one GameWindow per entry of page.state.wins (bottom → top, ≤ 2). The top
	one is the native modal on top; Esc / Back / swipe-down close it through history.
-->
<script lang="ts">
	import { page } from '$app/state';
	import WindowFrame from './WindowFrame.svelte';

	const wins = $derived(page.state.wins ?? []);
</script>

{#each wins as w, i (i + ':' + w.kind)}
	<WindowFrame win={w} top={i === wins.length - 1} />
{/each}
