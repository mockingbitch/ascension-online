<!-- Rank numeral in a hexagonal jade seal; 1–3 get gold / silver / bronze rims. -->
<script lang="ts">
	interface Props {
		rank: number;
		size?: number;
	}
	let { rank, size = 40 }: Props = $props();
	const tone = $derived(
		rank === 1 ? 'var(--color-gold-300)' : rank === 2 ? 'var(--color-silver-300)' : rank === 3 ? 'var(--color-bronze-500)' : 'var(--color-mist-500)'
	);
</script>

<span class="relative inline-grid shrink-0 place-items-center" style:width="{size}px" style:height="{size}px" style:color={tone}>
	<svg viewBox="0 0 40 40" class="absolute inset-0 size-full" aria-hidden="true">
		<path d="M20 2 36 11v18L20 38 4 29V11Z" fill="var(--color-ink-900)" stroke="currentColor" stroke-width={rank <= 3 ? 1.6 : 1} />
		{#if rank <= 3}<path d="M20 6 32.5 13v14L20 34 7.5 27V13Z" fill="none" stroke="currentColor" stroke-width="0.6" opacity="0.6" />{/if}
	</svg>
	<span class="relative leading-none font-semibold tabular-nums" style:font-size="{size * (rank > 99 ? 0.28 : 0.36)}px">{rank}</span>
</span>
