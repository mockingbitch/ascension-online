<!-- Original spirit-gathering formation: rune ring, eight trigram-like bars, inner star. Colour via `color`. -->
<script lang="ts">
	interface Props {
		color?: string; // CSS colour expression, e.g. 'var(--color-aura-jade)'
		class?: string;
	}
	let { color = 'var(--color-gold-500)', class: cls = '' }: Props = $props();

	const ticks = Array.from({ length: 48 }, (_, i) => i * 7.5);
	const bars = Array.from({ length: 8 }, (_, i) => i * 45);
</script>

<svg viewBox="0 0 200 200" class={cls} style:color aria-hidden="true">
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
		<path d="M100 44 L148 128 L52 128 Z M100 156 L52 72 L148 72 Z" stroke-width="0.9" opacity="0.75" />
		<circle cx="100" cy="100" r="34" stroke-width="1.2" />
		<circle cx="100" cy="100" r="6" fill="currentColor" opacity="0.6" />
	</g>
</svg>
