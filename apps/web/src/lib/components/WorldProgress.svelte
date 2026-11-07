<!-- Compact world-progress indicator for the game HUD: server day + progress toward the next story milestone
     (GET /world, refreshed on world.* WS events). Links to Thiên Đạo Sử. Hidden before launch and on narrow screens. -->
<script lang="ts">
	import { formatBp } from '$lib/game/format';
	import { screens } from '$lib/game/nav';
	import { loadWorld, world } from '$lib/game/world.svelte';
	import Icon from './Icon.svelte';

	const REFRESH_MS = 10 * 60_000; // progress only moves when a game day closes; WS events force a refresh
	$effect(() => {
		void loadWorld();
		const t = setInterval(() => void loadWorld(), REFRESH_MS);
		return () => clearInterval(t);
	});
	const v = $derived(world.view);
	const next = $derived(v?.nextMilestone);
	const label = $derived(
		v ? `Thiên Đạo · ngày ${v.serverDay}${v.era ? ` · ${v.era.name}` : ''}${next ? ` · tới ${next.name}: ${formatBp(next.progressBp)}` : ''}` : ''
	);
</script>

{#if v?.started}
	<a href={screens.timeline.href} class="world hidden lg:inline-flex" title={label} aria-label={label}>
		<Icon name="scroll" size={15} class="shrink-0 text-gold-500" />
		<span class="text-xs text-mist-300 tabular-nums">Ngày <strong class="text-mist-100">{v.serverDay}</strong></span>
		{#if next}
			<span class="track" aria-hidden="true"><span class="fill" style:width="{next.progressBp / 100}%"></span></span>
		{/if}
	</a>
{/if}

<style>
	.world {
		height: 2.25rem;
		align-items: center;
		gap: 0.4rem;
		padding: 0 0.6rem;
		border: 1px solid color-mix(in oklab, var(--color-gold-500) 22%, transparent);
		background: color-mix(in oklab, var(--color-ink-950) 55%, transparent);
		border-radius: 2px;
	}
	.world:hover {
		border-color: color-mix(in oklab, var(--color-gold-500) 50%, transparent);
	}
	.track {
		width: 3.5rem;
		height: 5px;
		overflow: hidden;
		border-radius: 9999px;
		background: var(--color-ink-950);
		box-shadow: inset 0 0 0 1px color-mix(in oklab, var(--color-gold-500) 25%, transparent);
	}
	.fill {
		display: block;
		height: 100%;
		background: linear-gradient(90deg, var(--color-gold-700), var(--color-gold-300));
		transition: width 600ms var(--ease-qi);
	}
</style>
