<!--
	Vân Khê (content/story/characters/van-khe.md), original SVG fallback of the art slots characters/guide/<expr>:
	an adult woman, half body, low bun with a peach-wood pin, smoke-violet crossed collar under an ink cloak, brush and
	bamboo notebook at the hip. Modest, non-sexualised. A face (eyes, brows, mouth) unlike the players' silhouettes, so
	she reads as a person, not a player figure. Six expressions; idle motion (breathing, head tilt, blink) only at
	effects full (lite: breathing), none at off / reduced motion — gated by :root[data-fx].
-->
<script lang="ts">
	import type { GuideExpr } from '$lib/game/types';

	let { expr = 'neutral', class: cls = '' }: { expr?: GuideExpr; class?: string } = $props();
	const uid = $props.id();
</script>

<svg viewBox="0 0 300 400" class="guide {cls}" aria-hidden="true" data-expr={expr}>
	<defs>
		<linearGradient id="{uid}-robe" x1="0" y1="0" x2="0" y2="1">
			<stop offset="0" stop-color="color-mix(in oklab, var(--color-spirit-violet) 62%, var(--color-ink-800))" />
			<stop offset="1" stop-color="color-mix(in oklab, var(--color-spirit-violet) 30%, var(--color-ink-950))" />
		</linearGradient>
		<linearGradient id="{uid}-cloak" x1="0" y1="0" x2="0" y2="1">
			<stop offset="0" stop-color="var(--color-ink-700)" />
			<stop offset="1" stop-color="var(--color-ink-950)" />
		</linearGradient>
		<radialGradient id="{uid}-skin" cx="0.45" cy="0.4" r="0.7">
			<stop offset="0" stop-color="color-mix(in oklab, var(--color-mist-100) 92%, var(--color-gold-300))" />
			<stop offset="1" stop-color="color-mix(in oklab, var(--color-mist-300) 85%, var(--color-gold-500))" />
		</radialGradient>
	</defs>

	<g class="body">
		<!-- ink cloak behind the shoulders -->
		<path d="M48 400 C52 300 74 240 112 214 L188 214 C226 240 248 300 252 400 Z" fill="url(#{uid}-cloak)" />
		<!-- smoke-violet robe, crossed collar (closed, high) -->
		<path d="M78 400 C84 316 104 262 130 236 L170 236 C196 262 216 316 222 400 Z" fill="url(#{uid}-robe)" stroke="var(--color-gold-300)" stroke-opacity="0.2" />
		<path d="M128 234 L150 290 L172 234" fill="none" stroke="var(--color-mist-100)" stroke-opacity="0.75" stroke-width="5" stroke-linejoin="round" />
		<path d="M136 236 L150 270 L164 236" fill="color-mix(in oklab, var(--color-spirit-violet) 40%, var(--color-ink-900))" />
		<!-- sash, brush and bamboo notebook at the hip -->
		<path d="M92 330 Q150 344 208 330" fill="none" stroke="var(--color-gold-700)" stroke-width="7" />
		<rect x="186" y="324" width="26" height="36" rx="2" fill="color-mix(in oklab, var(--color-aura-earth) 70%, var(--color-ink-900))" stroke="var(--color-gold-700)" />
		<path d="M190 332 H208 M190 340 H208 M190 348 H208" stroke="var(--color-ink-900)" stroke-width="1.2" />
		<path d="M104 318 L92 372" stroke="color-mix(in oklab, var(--color-aura-earth) 80%, var(--color-ink-700))" stroke-width="4" stroke-linecap="round" />
		<path d="M92 372 L88 384" stroke="var(--color-ink-950)" stroke-width="5" stroke-linecap="round" />

		<!-- arms: pointing (point), hand on the chest (proud), else folded sleeves -->
		{#if expr === 'point'}
			<path d="M206 256 C236 252 256 230 266 206" fill="none" stroke="url(#{uid}-robe)" stroke-width="22" stroke-linecap="round" />
			<circle cx="268" cy="200" r="8" fill="url(#{uid}-skin)" />
			<path d="M272 194 L284 180" stroke="url(#{uid}-skin)" stroke-width="5" stroke-linecap="round" />
			<path d="M96 262 C88 300 98 320 128 322" fill="none" stroke="url(#{uid}-robe)" stroke-width="22" stroke-linecap="round" />
		{:else if expr === 'proud'}
			<path d="M96 262 C88 300 104 300 140 280" fill="none" stroke="url(#{uid}-robe)" stroke-width="22" stroke-linecap="round" />
			<circle cx="146" cy="276" r="9" fill="url(#{uid}-skin)" />
			<path d="M204 262 C214 300 202 320 172 322" fill="none" stroke="url(#{uid}-robe)" stroke-width="22" stroke-linecap="round" />
		{:else}
			<path d="M96 262 C88 300 104 318 140 318 M204 262 C212 300 196 318 160 318" fill="none" stroke="url(#{uid}-robe)" stroke-width="22" stroke-linecap="round" />
			<ellipse cx="150" cy="318" rx="14" ry="8" fill="url(#{uid}-skin)" />
		{/if}
	</g>

	<g class="head">
		<!-- low bun at the nape, peach-wood pin, a few early silver strands -->
		<path d="M112 128 C108 86 130 62 150 62 C170 62 192 86 188 128 C188 160 176 186 150 190 C124 186 112 160 112 128 Z" fill="var(--color-ink-950)" />
		<ellipse cx="150" cy="186" rx="26" ry="15" fill="var(--color-ink-950)" />
		<path d="M118 176 L186 196" stroke="color-mix(in oklab, var(--color-rarity-tien) 55%, var(--color-aura-earth))" stroke-width="3.5" stroke-linecap="round" />
		<circle cx="186" cy="196" r="3.5" fill="var(--color-rarity-tien)" />
		<path d="M170 76 C178 90 182 104 182 118" fill="none" stroke="var(--color-mist-300)" stroke-opacity="0.5" stroke-width="1.2" />
		<!-- neck + face -->
		<path d="M140 196 L160 196 L162 230 L138 230 Z" fill="url(#{uid}-skin)" />
		<ellipse cx="150" cy="140" rx="31" ry="38" fill="url(#{uid}-skin)" />
		<!-- side-swept fringe -->
		<path d="M118 132 C120 98 138 84 158 86 C176 88 186 104 184 128 C172 112 156 104 140 106 C130 110 122 120 118 132 Z" fill="var(--color-ink-950)" />

		<!-- brows -->
		{#if expr === 'worried'}
			<path d="M128 122 Q136 118 144 124 M156 124 Q164 118 172 122" fill="none" stroke="var(--color-ink-900)" stroke-width="2.2" stroke-linecap="round" />
		{:else if expr === 'surprised'}
			<path d="M128 118 Q136 112 144 117 M156 117 Q164 112 172 118" fill="none" stroke="var(--color-ink-900)" stroke-width="2.2" stroke-linecap="round" />
		{:else}
			<path d="M128 124 Q136 120 144 123 M156 123 Q164 120 172 124" fill="none" stroke="var(--color-ink-900)" stroke-width="2.2" stroke-linecap="round" />
		{/if}

		<!-- eyes -->
		<g class="eyes">
			{#if expr === 'smile' || expr === 'proud'}
				<path d="M128 140 Q136 133 144 140 M156 140 Q164 133 172 140" fill="none" stroke="var(--color-ink-950)" stroke-width="2.6" stroke-linecap="round" />
			{:else if expr === 'surprised'}
				<circle cx="136" cy="139" r="4.6" fill="var(--color-ink-950)" />
				<circle cx="164" cy="139" r="4.6" fill="var(--color-ink-950)" />
			{:else}
				<ellipse class="eye" cx="136" cy="140" rx="5" ry="3.4" fill="var(--color-ink-950)" />
				<ellipse class="eye" cx="164" cy="140" rx="5" ry="3.4" fill="var(--color-ink-950)" />
			{/if}
		</g>

		<!-- mouth: a one-sided smile is her habit -->
		{#if expr === 'smile'}
			<path d="M138 160 Q150 170 164 158" fill="none" stroke="color-mix(in oklab, var(--color-cinnabar-300) 70%, var(--color-ink-800))" stroke-width="2.4" stroke-linecap="round" />
		{:else if expr === 'proud'}
			<path d="M140 160 Q152 166 162 157" fill="none" stroke="color-mix(in oklab, var(--color-cinnabar-300) 70%, var(--color-ink-800))" stroke-width="2.4" stroke-linecap="round" />
		{:else if expr === 'worried'}
			<path d="M140 163 Q150 158 160 163" fill="none" stroke="color-mix(in oklab, var(--color-cinnabar-300) 60%, var(--color-ink-800))" stroke-width="2.2" stroke-linecap="round" />
		{:else if expr === 'surprised'}
			<ellipse cx="150" cy="162" rx="4.5" ry="5.5" fill="color-mix(in oklab, var(--color-cinnabar-300) 55%, var(--color-ink-800))" />
		{:else}
			<path d="M142 160 Q151 164 160 158" fill="none" stroke="color-mix(in oklab, var(--color-cinnabar-300) 65%, var(--color-ink-800))" stroke-width="2.2" stroke-linecap="round" />
		{/if}
	</g>
</svg>

<style>
	.guide {
		overflow: visible;
	}
	.body,
	.head {
		transform-box: fill-box;
	}
	:global(:root[data-fx='full']) .body,
	:global(:root[data-fx='lite']) .body {
		transform-origin: 50% 100%;
		animation: breathe 4.2s var(--ease-in-out-sine) infinite;
	}
	:global(:root[data-fx='full']) .head {
		transform-origin: 50% 90%;
		animation: tilt 9s var(--ease-in-out-sine) infinite;
	}
	:global(:root[data-fx='full']) .eye {
		transform-box: fill-box;
		transform-origin: 50% 50%;
		animation: blink 5.5s steps(1, end) infinite;
	}
	@keyframes breathe {
		0%,
		100% {
			transform: scale(1);
		}
		50% {
			transform: scale(1.012, 1.018);
		}
	}
	@keyframes tilt {
		0%,
		60%,
		100% {
			transform: rotate(0deg);
		}
		70%,
		85% {
			transform: rotate(-3deg);
		}
	}
	@keyframes blink {
		0%,
		96% {
			transform: scaleY(1);
		}
		97%,
		99% {
			transform: scaleY(0.1);
		}
	}
</style>
