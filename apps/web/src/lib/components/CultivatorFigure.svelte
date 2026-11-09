<!--
	Original vector art: a robed cultivator, tinted by the realm aura. One asset, two crops: `crop="full"` (scene) /
	`crop="bust"` (portrait). Parameterised by the player's look (decisions 2026-11-09, GĐ-A): silhouette (nam / nữ,
	tiên / Vu), the body's hairstyle (+ beard), the hair accessory, the robe colour (token --color-robe-<id>) and the
	pose. It is the fallback of every per-body raster slot (characters/<body_id>/…), so the game is complete before any
	painted art exists. Without `figure` it draws the original default (male, topknot, ink robe, lotus pose).
	`still` = no ambient motion (many small figures, e.g. hall seats: ambient loops only on 1–3 focal elements).
-->
<script lang="ts">
	import type { Figure } from '$lib/game/looks';
	import type { Aura } from '$lib/game/types';

	interface Props {
		aura: Aura;
		crop?: 'full' | 'bust';
		figure?: Figure;
		still?: boolean;
		class?: string;
	}
	let { aura, crop = 'full', figure, still = false, class: cls = '' }: Props = $props();
	const uid = $props.id();
	const viewBox = $derived(crop === 'bust' ? '130 52 140 140' : '40 40 320 300');

	const f = $derived<Figure>(figure ?? { sex: 'nam', path: 'tien', hair: 'bui_cao', beard: false, acc: 'tram_ngoc', robe: '', pose: 'kiet_gia' });
	const nu = $derived(f.sex === 'nu');
	const vu = $derived(f.path === 'vu');
	// robe colour: the palette token, or the original ink robe
	const robe = $derived(f.robe ? `var(--color-robe-${f.robe})` : 'var(--color-ink-600)');
	// lower body: cross-legged (lotus / half lotus / sword), seated on a rock, or a standing horse stance (Vu)
	const stance = $derived(f.pose === 'trung_binh_tan');
	const rock = $derived(f.pose === 'toa_thach');
	const handsOnKnees = $derived(f.pose === 'ban_gia' || f.pose === 'toa_thach');
	const sword = $derived(f.pose === 'kiem_ngang');
	const pearl = $derived(f.pose === 'kiet_gia' || f.pose === 'kiem_ngang');

	// Head geometry per silhouette: the head is a little smaller for nữ, the jaw a little wider for Vu.
	const head = $derived(nu ? { rx: 17, ry: 21 } : vu ? { rx: 20, ry: 23 } : { rx: 19, ry: 23 });
	// Where the hair knot / accessory sits for each hairstyle.
	const knots: Record<string, { x: number; y: number }> = {
		bui_cao: { x: 200, y: 77 },
		bui_nua: { x: 200, y: 82 },
		duoi_ngua: { x: 200, y: 79 },
		van_cao: { x: 200, y: 66 },
		tet_lech: { x: 192, y: 84 },
		xoa_buoc_thap: { x: 214, y: 88 },
		bim_nhieu: { x: 200, y: 82 },
		chom: { x: 200, y: 78 },
		tet_duoi: { x: 200, y: 82 },
		ngan_khan: { x: 214, y: 92 }
	};
	const knot = $derived(knots[f.hair] ?? { x: 200, y: 77 });
	// Long hair behind the shoulders: how far down it falls.
	const behind = $derived(
		({ bui_cao: 204, bui_nua: 196, duoi_ngua: 0, van_cao: 150, tet_lech: 150, xoa_buoc_thap: 226, bim_nhieu: 0, chom: 0, tet_duoi: 0, ngan_khan: 0 } as Record<string, number>)[f.hair] ?? 204
	);
	// Shoulder width: Vu broad, nữ narrower.
	const sh = $derived(vu ? 1.12 : nu ? 0.9 : 1);
</script>

<svg {viewBox} class={cls} class:still style:--aura="var(--color-aura-{aura})" style:--robe={robe} aria-hidden="true" data-figure={f.sex + '-' + f.path}>
	<defs>
		<linearGradient id="{uid}-robe" x1="0" y1="0" x2="0" y2="1">
			<stop offset="0" stop-color="var(--robe)" />
			<stop offset="0.6" stop-color="color-mix(in oklab, var(--robe) 55%, var(--color-ink-850))" />
			<stop offset="1" stop-color="color-mix(in oklab, var(--robe) 30%, var(--color-ink-950))" />
		</linearGradient>
		<linearGradient id="{uid}-sleeve" x1="0" y1="0" x2="0" y2="1">
			<stop offset="0" stop-color="color-mix(in oklab, var(--robe) 80%, var(--color-ink-700))" />
			<stop offset="1" stop-color="color-mix(in oklab, var(--robe) 35%, var(--color-ink-850))" />
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

	<!-- floating silk ribbon behind the shoulders (tiên only; Vu carry no qi ribbon) -->
	{#if crop === 'full' && !vu}
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

	{#if rock}
		<!-- a flat boulder under the seated Vu -->
		<path d="M70 300 C80 270 130 262 200 264 C270 262 320 270 330 300 C334 316 300 326 200 326 C100 326 66 316 70 300 Z" fill="var(--color-ink-700)" stroke="var(--color-gold-300)" stroke-opacity="0.15" />
	{/if}

	<!-- long hair behind -->
	{#if behind}
		<path
			d="M{200 - (nu ? 20 : 18)} 98 C{170 - (nu ? 6 : 0)} 128 {168 - (nu ? 8 : 0)} {behind - 36} {174 - (nu ? 6 : 0)} {behind} L{226 + (nu ? 6 : 0)} {behind} C{232 + (nu ? 8 : 0)} {behind - 36} {230 + (nu ? 6 : 0)} 128 {200 + (nu ? 20 : 18)} 98 Z"
			fill="var(--color-ink-950)"
		/>
	{/if}
	{#if f.hair === 'duoi_ngua'}
		<!-- high ponytail swinging behind -->
		<path d="M204 78 C232 80 244 110 238 150 C236 166 228 178 222 186 C226 160 226 128 210 96 Z" fill="var(--color-ink-950)" />
	{:else if f.hair === 'tet_duoi'}
		<!-- one long braid down the back -->
		<path d="M196 116 L204 116 L206 236 C206 242 194 242 194 236 Z" fill="var(--color-ink-950)" />
	{/if}

	<g transform="translate(200 0) scale({sh} 1) translate(-200 0)">
		{#if stance}
			<!-- horse stance: knees bent outwards, feet planted -->
			<path
				d="M200 136 C178 136 160 142 150 156 C138 174 136 208 140 236 L104 276 L96 318 L132 318 L136 286 L176 252 L224 252 L264 286 L268 318 L304 318 L296 276 L260 236 C264 208 262 174 250 156 C240 142 222 136 200 136 Z"
				fill="url(#{uid}-robe)"
				stroke="var(--color-gold-300)"
				stroke-opacity="0.28"
				stroke-width="1.2"
			/>
		{:else}
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
		{/if}
		<!-- sash -->
		<path d="M150 222 Q200 234 250 222" fill="none" stroke={vu ? 'var(--color-bronze-500)' : 'var(--color-gold-700)'} stroke-width="3.5" />
		<path d="M196 230 L188 262 M204 230 L212 260" stroke="var(--color-gold-700)" stroke-width="2" stroke-linecap="round" />

		{#if vu}
			<!-- leather shoulder guards -->
			<path d="M150 150 C138 148 128 158 128 172 C140 168 152 166 160 170 Z M250 150 C262 148 272 158 272 172 C260 168 248 166 240 170 Z" fill="color-mix(in oklab, var(--color-bronze-500) 55%, var(--color-ink-900))" stroke="var(--color-gold-700)" stroke-opacity="0.5" />
		{/if}

		<!-- sleeves / arms -->
		{#if handsOnKnees || stance}
			<path
				d={stance
					? 'M154 156 C140 182 134 210 140 232 L156 234 C160 214 164 194 168 178 Z M246 156 C260 182 266 210 260 232 L244 234 C240 214 236 194 232 178 Z'
					: 'M154 156 C132 186 112 230 104 268 C112 278 130 280 138 270 C146 240 156 212 168 182 Z M246 156 C268 186 288 230 296 268 C288 278 270 280 262 270 C254 240 244 212 232 182 Z'}
				fill={vu && f.sex === 'nam' ? 'color-mix(in oklab, var(--color-mist-500) 55%, var(--color-ink-800))' : `url(#${uid}-sleeve)`}
				stroke="var(--color-gold-300)"
				stroke-opacity="0.22"
			/>
			{#if vu}
				<!-- bronze arm rings -->
				<path d={stance ? 'M140 214 L160 216 M260 214 L240 216' : 'M126 236 L146 244 M274 236 L254 244'} stroke="var(--color-bronze-500)" stroke-width="4" stroke-linecap="round" />
			{/if}
			<!-- fists / hands -->
			<circle cx={stance ? 148 : 120} cy={stance ? 238 : 272} r="7" fill="var(--color-mist-300)" opacity="0.8" />
			<circle cx={stance ? 252 : 280} cy={stance ? 238 : 272} r="7" fill="var(--color-mist-300)" opacity="0.8" />
		{:else}
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
		{/if}
	</g>

	{#if sword}
		<!-- sword laid across the lap (sheathed) -->
		<g>
			<rect x="96" y="268" width="196" height="7" rx="3" fill="color-mix(in oklab, var(--color-ink-950) 70%, var(--color-gold-700))" stroke="var(--color-gold-700)" stroke-width="1" />
			<rect x="292" y="264" width="6" height="15" rx="1" fill="var(--color-gold-500)" />
			<rect x="298" y="268" width="18" height="7" rx="2" fill="var(--color-ink-950)" />
		</g>
	{/if}

	<!-- qi pearl -->
	{#if pearl}
		<circle class="pearl" cx="200" cy="236" r="18" fill="url(#{uid}-pearl)" />
	{/if}

	<!-- neck, collar -->
	<path d="M192 122 L208 122 L210 140 L190 140 Z" fill="var(--color-mist-500)" opacity="0.55" />
	{#if vu}
		<path d="M182 140 L200 164 L218 140" fill="none" stroke="var(--color-bronze-500)" stroke-width="2.4" stroke-linejoin="round" />
		{#if f.acc !== 'khong'}<path d="M186 146 Q200 158 214 146" fill="none" stroke="var(--color-mist-300)" stroke-opacity="0.6" stroke-width="1.4" stroke-dasharray="2 3" />{/if}
	{:else}
		<path d="M184 140 L200 174 L216 140" fill="none" stroke="var(--color-gold-500)" stroke-width="2.2" stroke-linejoin="round" />
		<path d="M189 140 L200 164 L211 140" fill="none" stroke="var(--color-mist-100)" stroke-opacity="0.55" stroke-width="1.4" />
	{/if}

	<!-- head -->
	<ellipse cx="200" cy="104" rx={head.rx} ry={head.ry} fill="var(--color-ink-950)" stroke="var(--color-gold-300)" stroke-opacity="0.25" />
	<path d="M{200 - head.rx + 2} 100 Q200 112 {200 + head.rx - 2} 100" fill="none" stroke="var(--color-mist-500)" stroke-opacity="0.35" stroke-width="1.2" />
	{#if f.beard}
		<path d="M190 120 Q200 136 210 120 Q200 126 190 120 Z" fill="var(--color-ink-950)" stroke="var(--color-mist-500)" stroke-opacity="0.3" />
	{/if}

	<!-- hairstyle on top of the head -->
	{#if f.hair === 'bui_cao'}
		<ellipse cx="200" cy="77" rx="10" ry="8" fill="var(--color-ink-950)" />
	{:else if f.hair === 'bui_nua'}
		<ellipse cx="200" cy="82" rx="9" ry="6" fill="var(--color-ink-950)" />
	{:else if f.hair === 'duoi_ngua'}
		<ellipse cx="200" cy="79" rx="9" ry="7" fill="var(--color-ink-950)" />
	{:else if f.hair === 'van_cao'}
		<ellipse cx="200" cy="80" rx="15" ry="8" fill="var(--color-ink-950)" />
		<ellipse cx="200" cy="66" rx="11" ry="9" fill="var(--color-ink-950)" />
	{:else if f.hair === 'tet_lech'}
		<ellipse cx="192" cy="84" rx="12" ry="6" fill="var(--color-ink-950)" />
		{#each [0, 1, 2, 3, 4] as i (i)}
			<ellipse cx={180 - i * 2.5} cy={124 + i * 14} rx="6" ry="8" fill="var(--color-ink-950)" stroke="var(--color-mist-500)" stroke-opacity="0.25" />
		{/each}
	{:else if f.hair === 'xoa_buoc_thap'}
		<path d="M182 92 Q200 80 218 92 L218 100 Q200 90 182 100 Z" fill="var(--color-ink-950)" />
	{:else if f.hair === 'bim_nhieu'}
		<ellipse cx="200" cy="82" rx="12" ry="6" fill="var(--color-ink-950)" />
		{#each [-1, 1] as side (side)}
			{#each [0, 1] as k (k)}
				<path d="M{200 + side * (15 + k * 5)} 108 L{200 + side * (22 + k * 7)} 170" stroke="var(--color-ink-950)" stroke-width="5" stroke-linecap="round" />
			{/each}
		{/each}
	{:else if f.hair === 'chom'}
		<ellipse cx="200" cy="80" rx="6" ry="6" fill="var(--color-ink-950)" />
		<path d="M184 96 L186 112 M216 96 L214 112" stroke="var(--color-mist-500)" stroke-opacity="0.4" stroke-width="1" />
	{:else if f.hair === 'tet_duoi'}
		<ellipse cx="200" cy="83" rx="10" ry="5" fill="var(--color-ink-950)" />
	{:else if f.hair === 'ngan_khan'}
		<path d="M182 96 Q200 80 218 96" fill="none" stroke="var(--robe)" stroke-width="6" stroke-linecap="round" />
		<path d="M216 94 L228 100 L222 104 Z" fill="var(--robe)" />
	{/if}

	<!-- hair accessory -->
	{#if f.acc === 'tram_ngoc'}
		<path d="M{knot.x - 24} {knot.y - 2} L{knot.x + 26} {knot.y - 8}" stroke="var(--color-gold-300)" stroke-width="2" stroke-linecap="round" />
		<circle cx={knot.x + 26} cy={knot.y - 8} r="2.8" fill="var(--color-jade-300)" />
	{:else if f.acc === 'kim_quan'}
		<rect x={knot.x - 10} y={knot.y + 2} width="20" height="5" rx="1" fill="var(--color-gold-500)" />
		<path d="M{knot.x - 7} {knot.y + 2} L{knot.x} {knot.y - 9} L{knot.x + 7} {knot.y + 2} Z" fill="var(--color-gold-500)" />
	{:else if f.acc === 'day_lua'}
		<path d="M{knot.x + 6} {knot.y + 2} C{knot.x + 24} {knot.y + 8} {knot.x + 30} {knot.y + 30} {knot.x + 22} {knot.y + 52}" fill="none" stroke="var(--aura)" stroke-opacity="0.75" stroke-width="2.5" stroke-linecap="round" />
		<path d="M{knot.x + 6} {knot.y + 4} C{knot.x + 18} {knot.y + 14} {knot.x + 16} {knot.y + 36} {knot.x + 8} {knot.y + 50}" fill="none" stroke="var(--color-gold-300)" stroke-opacity="0.55" stroke-width="2" stroke-linecap="round" />
	{:else if f.acc === 'hoa_mai'}
		{#each [0, 72, 144, 216, 288] as a (a)}
			<circle cx={knot.x + 12 + 3.2 * Math.cos((a * Math.PI) / 180)} cy={knot.y + 2 + 3.2 * Math.sin((a * Math.PI) / 180)} r="2.6" fill="var(--color-rarity-tien)" />
		{/each}
		<circle cx={knot.x + 12} cy={knot.y + 2} r="1.4" fill="var(--color-gold-300)" />
	{:else if f.acc === 'tram_go'}
		<path d="M{knot.x - 20} {knot.y + 4} L{knot.x + 22} {knot.y - 4}" stroke="var(--color-bronze-500)" stroke-width="2.4" stroke-linecap="round" />
	{:else if f.acc === 'vong_dong'}
		{#each [-1, 1] as side (side)}
			<circle cx={200 + side * 18} cy="128" r="3.4" fill="none" stroke="var(--color-bronze-500)" stroke-width="2" />
		{/each}
		<circle cx={knot.x} cy={knot.y + 4} r="4" fill="none" stroke="var(--color-bronze-500)" stroke-width="2" />
	{:else if f.acc === 'long_vu'}
		<path d="M{knot.x + 6} {knot.y + 4} C{knot.x + 18} {knot.y - 8} {knot.x + 22} {knot.y - 22} {knot.x + 18} {knot.y - 34} C{knot.x + 12} {knot.y - 20} {knot.x + 6} {knot.y - 8} {knot.x + 6} {knot.y + 4} Z" fill="var(--color-gold-300)" opacity="0.85" />
	{:else if f.acc === 'day_da'}
		<path d="M181 98 Q200 92 219 98" fill="none" stroke="var(--color-bronze-500)" stroke-width="2.6" />
		<path d="M219 98 L226 112" stroke="var(--color-bronze-500)" stroke-width="2" stroke-linecap="round" />
	{/if}
	{#if !figure}
		<!-- the original default: gold band + aura bead -->
		<rect x="191" y="80" width="18" height="4.5" rx="1" fill="var(--color-gold-500)" />
	{/if}
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
	.still .ribbon,
	.still .pearl {
		animation: none;
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
