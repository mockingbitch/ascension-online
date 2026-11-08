<!-- Nhiệm vụ ngày (window): the daily board from the store; claiming goes through the store (server decides). -->
<script lang="ts">
	import DailyTasks from '$lib/components/DailyTasks.svelte';
	import { game } from '$lib/game/store.svelte';
</script>

{#if game.catalogue}
	<DailyTasks board={game.board} catalogue={game.catalogue} busy={game.busy} onclaim={game.claimTask} onincense={game.offerIncense} />
{/if}
