<!--
	Original vector art: a robed cultivator seated in lotus pose, cupping a qi pearl. Ink silhouette with
	gold rim light, tinted by the realm aura. One asset, two crops: `crop="full"` (scene) / `crop="bust"` (portrait).
	Placeholder-grade until painted art exists — replace with static/art/cultivator@{1,2}x.webp (transparent, 800×840).
-->
<script lang="ts">
	import type { Aura } from '$lib/game/types';

	interface Props {
		aura: Aura;
		crop?: 'full' | 'bust';
		class?: string;
	}
	let { aura, crop = 'full', class: cls = '' }: Props = $props();
	const uid = $props.id();
	const viewBox = $derived(crop === 'bust' ? '130 52 140 140' : '40 40 320 300');
</script>

<svg {viewBox} class={cls} style:--aura="var(--color-aura-{aura})" aria-hidden="true">
	<defs>
		<linearGradient id="{uid}-robe" x1="0" y1="0" x2="0" y2="1">
			<stop offset="0" stop-color="var(--color-ink-600)" />
			<stop offset="0.6" stop-color="var(--color-ink-800)" />
			<stop offset="1" stop-color="var(--color-ink-900)" />
		</linearGradient>
		<linearGradient id="{uid}-sleeve" x1="0" y1="0" x2="0" y2="1">
			<stop offset="0" stop-color="var(--color-ink-700)" />
			<stop offset="1" stop-color="var(--color-ink-850)" />
		</linearGradient>
		<radialGradient id="{uid}-pearl" cx="0.4" cy="0.35" r="0.65">
			<stop offset="0" stop-color="var(--color-mist-100)" />
			<stop offset="0.45" stop-color="var(--aura)" />
			<stop offset="1" stop-color="var(--aura)" stop-opacity="0" />
		</radialGradient>
		<linearGradient id="{uid}-ribbon" x1="0" y1="0" x2="1" y2="0">
			<stop offset="0" stop-color="var(--aura)" stop-opacity="0" />
			<stop offset="0.3" stop-color="var(--aura)" stop-opacity="0.55" />
			<stop offset="0.7" stop-color="var(--color-gold-300)" stop-opacity="0.45" />
			<stop offset="1" stop-color="var(--color-gold-300)" stop-opacity="0" />
		</linearGradient>
	</defs>

	<!-- floating silk ribbon behind the shoulders -->
	{#if crop === 'full'}
		<g class="ribbon">
			<path
				d="M44 268 C70 250 92 214 118 196 C150 172 170 150 200 150 C232 150 252 172 284 196 C310 214 330 250 356 262"
				fill="none"
				stroke="url(#{uid}-ribbon)"
				stroke-width="4"
				stroke-linecap="round"
			/>
		</g>
	{/if}

	<!-- long hair behind -->
	<path d="M182 98 C170 128 168 168 174 204 L226 204 C232 168 230 128 218 98 Z" fill="var(--color-ink-950)" />

	<!-- robe body + crossed legs -->
	<path
		d="M200 136 C178 136 160 142 150 156 C138 174 136 208 142 238 C124 250 92 262 74 282 C66 292 72 304 94 308
		   C140 316 260 316 306 308 C328 304 334 292 326 282 C308 262 276 250 258 238 C264 208 262 174 250 156 C240 142 222 136 200 136 Z"
		fill="url(#{uid}-robe)"
		stroke="var(--color-gold-300)"
		stroke-opacity="0.28"
		stroke-width="1.2"
	/>
	<!-- knee folds -->
	<path d="M98 292 Q150 280 190 298 M302 292 Q250 280 210 298" fill="none" stroke="var(--color-ink-950)" stroke-opacity="0.55" stroke-width="2" />
	<!-- sash -->
	<path d="M150 222 Q200 234 250 222" fill="none" stroke="var(--color-gold-700)" stroke-width="3.5" />
	<path d="M196 230 L188 262 M204 230 L212 260" stroke="var(--color-gold-700)" stroke-width="2" stroke-linecap="round" />

	<!-- wide sleeves, hands meeting before the dantian -->
	<path
		d="M154 156 C132 188 124 226 138 252 C148 268 172 270 188 258 L198 250 C182 236 170 210 168 182 Z"
		fill="url(#{uid}-sleeve)"
		stroke="var(--color-gold-300)"
		stroke-opacity="0.22"
	/>
	<path
		d="M246 156 C268 188 276 226 262 252 C252 268 228 270 212 258 L202 250 C218 236 230 210 232 182 Z"
		fill="url(#{uid}-sleeve)"
		stroke="var(--color-gold-300)"
		stroke-opacity="0.22"
	/>
	<path d="M186 252 Q200 262 214 252 Q206 246 200 246 Q194 246 186 252Z" fill="var(--color-mist-300)" opacity="0.8" />

	<!-- qi pearl -->
	<circle class="pearl" cx="200" cy="236" r="18" fill="url(#{uid}-pearl)" />

	<!-- neck, collar -->
	<path d="M192 122 L208 122 L210 140 L190 140 Z" fill="var(--color-mist-500)" opacity="0.55" />
	<path d="M184 140 L200 174 L216 140" fill="none" stroke="var(--color-gold-500)" stroke-width="2.2" stroke-linejoin="round" />
	<path d="M189 140 L200 164 L211 140" fill="none" stroke="var(--color-mist-100)" stroke-opacity="0.55" stroke-width="1.4" />

	<!-- head, topknot, hairpin -->
	<ellipse cx="200" cy="104" rx="19" ry="23" fill="var(--color-ink-950)" stroke="var(--color-gold-300)" stroke-opacity="0.25" />
	<path d="M183 100 Q200 112 217 100" fill="none" stroke="var(--color-mist-500)" stroke-opacity="0.35" stroke-width="1.2" />
	<ellipse cx="200" cy="77" rx="10" ry="8" fill="var(--color-ink-950)" />
	<rect x="191" y="80" width="18" height="4.5" rx="1" fill="var(--color-gold-500)" />
	<path d="M176 75 L226 69" stroke="var(--color-gold-300)" stroke-width="2" stroke-linecap="round" />
	<circle cx="226" cy="69" r="2.6" fill="var(--aura)" />
</svg>

<style>
	.ribbon {
		transform-origin: 200px 160px;
		animation: sway 7s ease-in-out infinite alternate;
	}
	@keyframes sway {
		from {
			transform: rotate(-1.6deg) translateY(-2px);
		}
		to {
			transform: rotate(1.6deg) translateY(3px);
		}
	}
	.pearl {
		transform-origin: 200px 236px;
		animation: pulse 3.6s ease-in-out infinite;
	}
	@keyframes pulse {
		0%,
		100% {
			transform: scale(0.92);
			opacity: 0.85;
		}
		50% {
			transform: scale(1.08);
			opacity: 1;
		}
	}
</style>
