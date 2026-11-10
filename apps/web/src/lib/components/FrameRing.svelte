<!--
	Portrait frame fallback (cosmetics.yaml avatar_frame:*, until raster `cosmetics/<slug>` art ships): a layered SVG
	ring per style — base band, inner/outer rims and the style's ornaments (bamboo nodes, carved jade beads, bronze
	thunder meander + rivets, cloud scrolls, flame tongues, gold ruyi heads, a star orbit) — richer with the rarity.
	A thin light glint travels round the ring at effects full only. Colours are tokens; looks only, no gameplay effect.
-->
<script lang="ts">
	import type { FrameStyle } from '$lib/game/cosmetics.svelte';

	let { style, class: cls = '' }: { style: FrameStyle; class?: string } = $props();
	const uid = $props.id();
	// ornament angles (deg)
	const ring = (n: number, off = 0) => Array.from({ length: n }, (_, i) => off + (360 / n) * i);
	const P: Record<FrameStyle, { band: string; rim: string; hi: string; w: number }> = {
		bamboo: { band: 'var(--color-jade-500)', rim: 'var(--color-jade-700)', hi: 'var(--color-jade-300)', w: 6 },
		jade: { band: 'var(--color-jade-300)', rim: 'var(--color-jade-700)', hi: 'var(--color-mist-100)', w: 6.5 },
		bronze: { band: 'var(--color-bronze-500)', rim: 'var(--color-gold-700)', hi: 'var(--color-gold-300)', w: 7 },
		cloud: { band: 'var(--color-silver-300)', rim: 'var(--color-mist-500)', hi: 'var(--color-mist-100)', w: 6 },
		flame: { band: 'var(--color-cinnabar-500)', rim: 'var(--color-gold-700)', hi: 'var(--color-gold-300)', w: 6.5 },
		gold: { band: 'var(--color-gold-500)', rim: 'var(--color-gold-700)', hi: 'var(--color-gold-300)', w: 7 },
		celestial: { band: 'var(--color-spirit-violet)', rim: 'var(--color-ink-700)', hi: 'var(--color-spirit-cyan)', w: 6.5 }
	};
	const p = $derived(P[style]);
	// ring radius: the portrait disc is r=50 of the 100-box frame drawn at inset -8% (so r≈43 in this box)
	const R = 46;
</script>

<svg class="frame-ring {cls}" data-frame-style={style} viewBox="0 0 100 100" aria-hidden="true" style:--band={p.band} style:--rim={p.rim} style:--hi={p.hi}>
	<defs>
		<linearGradient id="{uid}-g" x1="0" y1="0" x2="1" y2="1">
			<stop offset="0" stop-color={p.hi} />
			<stop offset="0.45" stop-color={p.band} />
			<stop offset="1" stop-color={p.rim} />
		</linearGradient>
	</defs>
	<!-- base band + rims -->
	<circle cx="50" cy="50" r={R} fill="none" stroke="url(#{uid}-g)" stroke-width={p.w} />
	<circle cx="50" cy="50" r={R - p.w / 2} fill="none" stroke={p.rim} stroke-width="0.8" />
	<circle cx="50" cy="50" r={R + p.w / 2} fill="none" stroke={p.rim} stroke-width="0.8" />

	{#if style === 'bamboo'}
		{#each ring(12) as a (a)}<rect x="48.6" y={50 - R - p.w / 2} width="2.8" height={p.w} rx="0.8" fill={p.rim} transform="rotate({a} 50 50)" />{/each}
		{#each ring(3, 20) as a (a)}<path d="M50 {50 - R - 3} q5 -6 11 -5 q-5 3 -11 5 Z" fill={p.hi} transform="rotate({a} 50 50)" />{/each}
	{:else if style === 'jade'}
		<circle cx="50" cy="50" r={R} fill="none" stroke={p.hi} stroke-width="0.6" stroke-dasharray="1.2 3.2" opacity="0.8" />
		{#each ring(4, 45) as a (a)}<circle cx="50" cy={50 - R} r="3.4" fill={p.band} stroke={p.hi} stroke-width="0.8" transform="rotate({a} 50 50)" />{/each}
	{:else if style === 'bronze'}
		{#each ring(16) as a (a)}<path d="M48 {50 - R + 1.6} h2.6 v-2.4 h-1.4 v1.2" fill="none" stroke={p.hi} stroke-width="0.7" transform="rotate({a} 50 50)" />{/each}
		{#each ring(8, 11.25) as a (a)}<circle cx="50" cy={50 - R} r="1.3" fill={p.hi} transform="rotate({a} 50 50)" />{/each}
	{:else if style === 'cloud'}
		{#each ring(6, 30) as a (a)}<path d="M44 {50 - R + 1} q0 -6 6 -6 q5 0 5 4 q0 3 -3 3 q-2.4 0 -2.4 -2" fill="none" stroke={p.hi} stroke-width="1.6" stroke-linecap="round" transform="rotate({a} 50 50)" />{/each}
	{:else if style === 'flame'}
		{#each ring(10) as a, i (a)}<path d="M47.4 {50 - R - 2} q2.6 -{i % 2 ? 5 : 8} 0.4 -{i % 2 ? 8 : 12} q4.6 {i % 2 ? 4 : 6} 4.8 {i % 2 ? 8 : 12} Z" fill={i % 2 ? p.hi : p.band} transform="rotate({a} 50 50)" />{/each}
	{:else if style === 'gold'}
		<circle cx="50" cy="50" r={R - p.w / 2 - 2} fill="none" stroke={p.band} stroke-width="1" />
		{#each ring(4) as a (a)}<path d="M50 {50 - R - 6} c3 0 4.6 3 2.4 5 l-2.4 3 l-2.4 -3 c-2.2 -2 -0.6 -5 2.4 -5 Z" fill={p.hi} stroke={p.rim} stroke-width="0.6" transform="rotate({a} 50 50)" />{/each}
		{#each ring(8, 22.5) as a (a)}<circle cx="50" cy={50 - R} r="1.1" fill={p.hi} transform="rotate({a} 50 50)" />{/each}
	{:else if style === 'celestial'}
		<circle cx="50" cy="50" r={R + 1} fill="none" stroke={p.hi} stroke-width="0.5" opacity="0.7" />
		<g class="orbit">
			{#each ring(14) as a, i (a)}<circle cx="50" cy={50 - R - 2.5} r={i % 4 === 0 ? 1.6 : 0.8} fill={i % 4 === 0 ? 'var(--color-gold-300)' : p.hi} transform="rotate({a} 50 50)" />{/each}
		</g>
		{#each ring(4, 45) as a (a)}<path d="M50 {50 - R - 4.5} l1.2 3.3 l3.3 1.2 l-3.3 1.2 l-1.2 3.3 l-1.2 -3.3 l-3.3 -1.2 l3.3 -1.2 Z" fill="var(--color-gold-300)" transform="rotate({a} 50 50)" />{/each}
	{/if}
	<!-- travelling glint (full only) -->
	<circle class="glint" cx="50" cy="50" r={R} fill="none" stroke="var(--color-mist-100)" stroke-width={p.w * 0.45} stroke-linecap="round" pathLength="100" stroke-dasharray="5 95" />
</svg>

<style>
	.frame-ring {
		position: absolute;
		inset: -8%;
		width: 116%;
		height: 116%;
		max-width: none;
		overflow: visible;
		pointer-events: none;
		filter: drop-shadow(0 1px 2px var(--color-ink-950));
	}
	.glint {
		opacity: 0;
		transform-origin: 50px 50px;
	}
	.orbit {
		transform-origin: 50px 50px;
	}
	:global(:root[data-fx='full']) .glint {
		opacity: 0.55;
		mix-blend-mode: screen;
		animation: glint 5.5s linear infinite;
	}
	:global(:root[data-fx='full']) .orbit {
		animation: glint 60s linear infinite;
	}
	@keyframes glint {
		to {
			transform: rotate(360deg);
		}
	}
</style>
