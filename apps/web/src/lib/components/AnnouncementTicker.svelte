<script lang="ts">
	import { feed, startRealtime } from '$lib/api/realtime.svelte';
	import Icon from './Icon.svelte';

	$effect(() => startRealtime());
	const latest = $derived(feed.items[0]);
</script>

{#if latest}
	<div class="ticker-strip flex items-center gap-2 px-3 py-1.5 text-sm text-gold-300" aria-live="polite">
		<Icon name="bolt" size={16} class="text-gold-500" />
		{#key latest.id}
			<p class="ticker min-w-0 truncate">{latest.data.text}</p>
		{/key}
	</div>
{/if}

<style>
	.ticker-strip {
		background: linear-gradient(90deg, transparent, color-mix(in oklab, var(--color-gold-500) 10%, transparent) 15%, color-mix(in oklab, var(--color-gold-500) 10%, transparent) 85%, transparent);
	}
	.ticker {
		animation: ticker-in 600ms var(--ease-qi);
	}
	@keyframes ticker-in {
		from {
			opacity: 0;
			transform: translateY(-4px);
		}
		to {
			opacity: 1;
			transform: none;
		}
	}
</style>
