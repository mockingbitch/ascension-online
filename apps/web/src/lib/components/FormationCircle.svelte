<!--
	Original spirit-gathering formation: rune ring, eight broken/solid bars, a neutral eight-point star (two squares). Colour via `color`.
	`tier` (1–6, realms.yaml visual.fx_tier) adds layers cumulatively, so a higher realm's formation is always richer:
	2 outer dashed ring + nodes · 3 inner counter ring · 4 lightning glyphs · 5 cloud scrolls · 6 star ring + inner star.
	Generic motifs only (no worship iconography).
-->
<script lang="ts">
	interface Props {
		color?: string; // CSS colour expression, e.g. 'var(--color-aura-jade)'
		tier?: number;
		class?: string;
	}
	let { color = 'var(--color-gold-500)', tier = 1, class: cls = '' }: Props = $props();

	const ticks = Array.from({ length: 48 }, (_, i) => i * 7.5);
	const bars = Array.from({ length: 8 }, (_, i) => i * 45);
	const inner = Array.from({ length: 24 }, (_, i) => i * 15);
	const quarter = [0, 90, 180, 270];
	const stars = Array.from({ length: 12 }, (_, i) => i * 30);
</script>

<svg viewBox="0 0 200 200" class={cls} style:color aria-hidden="true" data-formation-tier={tier}>
	<g fill="none" stroke="currentColor">
		<circle cx="100" cy="100" r="97" stroke-width="1.2" opacity="0.9" />
		<circle cx="100" cy="100" r="91" stroke-width="0.5" opacity="0.6" />
		{#each ticks as a (a)}
			<line x1="100" y1="4" x2="100" y2={a % 45 === 0 ? 14 : 9} stroke-width={a % 45 === 0 ? 1.4 : 0.6} transform="rotate({a} 100 100)" />
		{/each}
		<!-- eight broken/solid bars, original arrangement -->
		{#each bars as a, i (a)}
			<g transform="rotate({a} 100 100)" stroke-width="2.2" stroke-linecap="round" opacity="0.85">
				<line x1="88" y1="22" x2={i % 2 ? 97 : 112} y2="22" />
				{#if i % 2}<line x1="103" y1="22" x2="112" y2="22" />{/if}
				<line x1="88" y1="28" x2={i % 3 ? 112 : 97} y2="28" />
				{#if !(i % 3)}<line x1="103" y1="28" x2="112" y2="28" />{/if}
			</g>
		{/each}
		<circle cx="100" cy="100" r="66" stroke-width="0.8" opacity="0.7" stroke-dasharray="2 4" />
		<!-- neutral eight-point star (two squares) — no religious symbol -->
		<rect x="62" y="62" width="76" height="76" stroke-width="0.9" opacity="0.75" />
		<rect x="62" y="62" width="76" height="76" stroke-width="0.9" opacity="0.75" transform="rotate(45 100 100)" />
		<circle cx="100" cy="100" r="34" stroke-width="1.2" />
		<circle cx="100" cy="100" r="6" fill="currentColor" opacity="0.6" />

		{#if tier >= 2}
			<g class="t2" opacity="0.8">
				<circle cx="100" cy="100" r="99.5" stroke-width="0.6" stroke-dasharray="1 3" />
				{#each bars as a (a)}<circle cx="100" cy="0.8" r="1.8" fill="currentColor" transform="rotate({a + 22.5} 100 100)" />{/each}
			</g>
		{/if}
		{#if tier >= 3}
			<g class="t3" opacity="0.75">
				<circle cx="100" cy="100" r="48" stroke-width="0.9" />
				{#each inner as a (a)}<line x1="100" y1="52" x2="100" y2={a % 45 === 0 ? 60 : 56} stroke-width="0.7" transform="rotate({a + 7.5} 100 100)" />{/each}
			</g>
		{/if}
		{#if tier >= 4}
			<g class="t4" stroke-width="1.3" stroke-linejoin="bevel" opacity="0.9">
				{#each quarter as a (a)}<path d="M100 36 L94 46 L101 47 L95 58" transform="rotate({a + 45} 100 100)" />{/each}
			</g>
		{/if}
		{#if tier >= 5}
			<g class="t5" stroke-width="0.9" opacity="0.8">
				{#each bars as a (a)}<path d="M92 80 q4 -6 8 0 q4 6 8 0" transform="rotate({a + 22.5} 100 100) translate(0 -2)" />{/each}
			</g>
		{/if}
		{#if tier >= 6}
			<g class="t6" opacity="0.95">
				{#each stars as a, i (a)}<circle cx="100" cy="20" r={i % 3 ? 1.1 : 2} fill="currentColor" transform="rotate({a + 15} 100 100)" />{/each}
				<rect x="84" y="84" width="32" height="32" stroke-width="0.8" />
				<rect x="84" y="84" width="32" height="32" stroke-width="0.8" transform="rotate(45 100 100)" />
			</g>
		{/if}
	</g>
</svg>
