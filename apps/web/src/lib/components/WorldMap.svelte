<!--
	Thiên Nguyên Giới world map: one layered SVG (sky, moon, far peaks, drifting cloud sea, floating palace with light
	rays, cave peak, sect hall, peach pavilion, moon-gate portal, battlefield) + clickable location plaques.
	Only transform/opacity animate (clouds, rays, motes). Locations come from lib/game/nav (status-aware).
-->
<script lang="ts">
	import { goto } from '$app/navigation';
	import { art } from '$lib/game/art';
	import { mapLocations, screens } from '$lib/game/nav';
	import { toast } from '$lib/game/toasts.svelte';
	import Icon from './Icon.svelte';

	/** cover: decorative full-bleed background (login) — art fills the box, no plaques. */
	let { cover = false }: { cover?: boolean } = $props();

	const motes = Array.from({ length: 22 }, (_, i) => ({
		x: (i * 73) % 1600,
		y: 180 + ((i * 131) % 520),
		r: 1.2 + (i % 3) * 0.8,
		d: (i * 0.53) % 7
	}));
	const stars = Array.from({ length: 60 }, (_, i) => ({ x: (i * 197) % 1600, y: (i * 89) % 330, r: 0.6 + (i % 4) * 0.35, o: 0.25 + (i % 5) * 0.12 }));

	// Raster art (docs/design/art-prompts.md §1–2): 4 parallax layers > one painted map > the SVG below.
	const layers = (['sky', 'far', 'mid', 'ground'] as const).map((l, depth) => ({ depth, img: art(`world/${l}`) })).filter((l) => l.img);
	const layered = layers.length > 0 && !!art('world/sky');
	const painted = art('world/map');
	let px = $state(0);
	let py = $state(0);
	function parallax(e: PointerEvent) {
		const r = (e.currentTarget as HTMLElement).getBoundingClientRect();
		px = ((e.clientX - r.left) / r.width - 0.5) * 2;
		py = ((e.clientY - r.top) / r.height - 0.5) * 2;
	}

	function enter(id: keyof typeof screens) {
		const s = screens[id];
		if (s.status === 'soon') toast('warn', `${s.label}: cấm chế chưa giải — sẽ khai mở theo Thiên Đạo Sử.`);
		else void goto(s.href);
	}
</script>

<div class="fit-box" class:is-cover={cover}>
<div class="map relative overflow-hidden">
	{#if layered && !cover}
		<div class="absolute inset-0" onpointermove={parallax} onpointerleave={() => ((px = 0), (py = 0))} role="presentation">
			{#each layers as l (l.depth)}
				<img src={l.img!.src} srcset={l.img!.srcset} alt="" class="layer" style:--d={l.depth} style:--px={px} style:--py={py} draggable="false" />
			{/each}
			{#each mapLocations as l (l.screen)}
				{@const a = art(`landmarks/${l.landmark}`)}
				{#if a}
					<img src={a.src} srcset={a.srcset} alt="" class="landmark" class:is-soon={screens[l.screen].status === 'soon'} style:left="{l.x}%" style:top="{l.y - 9}%" draggable="false" />
				{/if}
			{/each}
		</div>
	{:else if painted || (layered && cover)}
		{@const img = painted ?? art('world/sky')!}
		<img src={img.src} srcset={img.srcset} alt="" class="absolute inset-0 size-full object-cover" draggable="false" />
	{:else}
	<svg viewBox="0 0 1600 900" class="block h-full w-full" preserveAspectRatio={cover ? 'xMidYMid slice' : 'xMidYMid meet'} aria-hidden="true">
		<defs>
			<linearGradient id="wm-sky" x1="0" y1="0" x2="0" y2="1">
				<stop offset="0" stop-color="var(--color-ink-950)" />
				<stop offset="0.5" stop-color="var(--color-ink-850)" />
				<stop offset="1" stop-color="var(--color-ink-700)" />
			</linearGradient>
			<radialGradient id="wm-glow" cx="0.5" cy="0.5" r="0.5">
				<stop offset="0" stop-color="var(--color-gold-300)" stop-opacity="0.55" />
				<stop offset="0.35" stop-color="var(--color-gold-500)" stop-opacity="0.18" />
				<stop offset="1" stop-color="var(--color-gold-500)" stop-opacity="0" />
			</radialGradient>
			<radialGradient id="wm-portal" cx="0.5" cy="0.5" r="0.5">
				<stop offset="0" stop-color="var(--color-spirit-violet)" stop-opacity="0.9" />
				<stop offset="0.6" stop-color="var(--color-spirit-violet)" stop-opacity="0.25" />
				<stop offset="1" stop-color="var(--color-spirit-violet)" stop-opacity="0" />
			</radialGradient>
			<radialGradient id="wm-war" cx="0.5" cy="0.5" r="0.5">
				<stop offset="0" stop-color="var(--color-cinnabar-500)" stop-opacity="0.55" />
				<stop offset="1" stop-color="var(--color-cinnabar-500)" stop-opacity="0" />
			</radialGradient>
			<radialGradient id="wm-cave" cx="0.5" cy="0.5" r="0.5">
				<stop offset="0" stop-color="var(--color-jade-300)" stop-opacity="0.9" />
				<stop offset="1" stop-color="var(--color-jade-500)" stop-opacity="0" />
			</radialGradient>
			<linearGradient id="wm-far" x1="0" y1="0" x2="0" y2="1">
				<stop offset="0" stop-color="var(--color-ink-600)" stop-opacity="0.7" />
				<stop offset="1" stop-color="var(--color-ink-800)" stop-opacity="0.2" />
			</linearGradient>
			<linearGradient id="wm-rock" x1="0" y1="0" x2="0" y2="1">
				<stop offset="0" stop-color="var(--color-ink-600)" />
				<stop offset="1" stop-color="var(--color-ink-900)" />
			</linearGradient>
			<linearGradient id="wm-cloud" x1="0" y1="0" x2="0" y2="1">
				<stop offset="0" stop-color="var(--color-mist-100)" stop-opacity="0.22" />
				<stop offset="1" stop-color="var(--color-mist-100)" stop-opacity="0.02" />
			</linearGradient>
			<linearGradient id="wm-ray" x1="0" y1="0" x2="0" y2="1">
				<stop offset="0" stop-color="var(--color-gold-300)" stop-opacity="0.35" />
				<stop offset="1" stop-color="var(--color-gold-300)" stop-opacity="0" />
			</linearGradient>
		</defs>

		<!-- sky, stars, moon -->
		<rect width="1600" height="900" fill="url(#wm-sky)" />
		{#each stars as s, i (i)}<circle cx={s.x} cy={s.y} r={s.r} fill="var(--color-mist-100)" opacity={s.o} />{/each}
		<circle cx="1330" cy="120" r="150" fill="url(#wm-glow)" opacity="0.5" />
		<circle cx="1330" cy="120" r="34" fill="var(--color-gold-300)" opacity="0.55" />

		<!-- far peaks -->
		<path d="M0 520 120 380 230 470 380 300 520 450 640 360 760 470 900 330 1040 460 1180 340 1320 470 1460 360 1600 450V900H0Z" fill="url(#wm-far)" />

		<!-- light rays from the palace -->
		<g class="rays" style="transform-origin: 800px 190px">
			{#each [-38, -22, -8, 8, 22, 38] as a (a)}
				<path d="M800 190 L770 900 L830 900Z" fill="url(#wm-ray)" transform="rotate({a} 800 190)" />
			{/each}
		</g>

		<!-- floating palace (Tử Tiêu Cung) on a cloud island -->
		<g>
			<circle cx="800" cy="180" r="230" fill="url(#wm-glow)" />
			<path d="M640 300c40-30 280-30 320 0-20 30-90 60-160 70-70-10-140-40-160-70Z" fill="url(#wm-rock)" />
			<ellipse cx="800" cy="300" rx="200" ry="26" fill="var(--color-mist-100)" opacity="0.16" />
			<!-- three-tier hall -->
			<g fill="var(--color-ink-800)" stroke="var(--color-gold-500)" stroke-width="2">
				<rect x="730" y="250" width="140" height="44" />
				<path d="M690 252h220l-30-22H720Z" />
				<rect x="752" y="196" width="96" height="34" />
				<path d="M718 198h164l-26-20H744Z" />
				<rect x="776" y="150" width="48" height="28" />
				<path d="M752 152h96l-22-18h-52Z" />
				<path d="M800 104v30" />
			</g>
			<circle cx="800" cy="100" r="7" fill="var(--color-gold-300)" />
			<g fill="var(--color-gold-300)" opacity="0.85">
				<rect x="790" y="262" width="20" height="32" />
				<rect x="760" y="208" width="10" height="16" /><rect x="830" y="208" width="10" height="16" />
			</g>
		</g>

		<!-- left: cave peak (Động Phủ) -->
		<path d="M140 900 250 560 330 600 420 470 560 900Z" fill="url(#wm-rock)" />
		<path d="M420 470 440 520 400 560" fill="none" stroke="var(--color-mist-100)" stroke-opacity="0.15" stroke-width="3" />
		<circle cx="352" cy="560" r="70" fill="url(#wm-cave)" opacity="0.55" class="pulse" />
		<path d="M326 600v-34a26 26 0 0 1 52 0v34Z" fill="var(--color-ink-950)" stroke="var(--color-jade-300)" stroke-width="2" />

		<!-- right: sect hall on a cliff (Thế Lực) -->
		<path d="M1040 900 1120 520 1230 470 1300 540 1380 900Z" fill="url(#wm-rock)" />
		<g fill="var(--color-ink-800)" stroke="var(--color-gold-500)" stroke-width="2">
			<rect x="1180" y="430" width="90" height="40" />
			<path d="M1158 432h134l-22-18h-90Z" />
			<rect x="1198" y="388" width="54" height="26" />
			<path d="M1182 390h86l-18-15h-50Z" />
		</g>
		<path d="M1290 420v-70" stroke="var(--color-gold-500)" stroke-width="2" />
		<path d="M1290 350h30l-8 10 8 10h-30" fill="var(--color-cinnabar-500)" />

		<!-- bottom-left-centre: peach pavilion (Đào Hoa Đình) -->
		<g>
			<path d="M520 900 560 760 700 720 760 790 800 900Z" fill="url(#wm-rock)" opacity="0.9" />
			{#each [[580, 690], [610, 660], [650, 670], [690, 690], [630, 700], [560, 715], [720, 715]] as [x, y], i (i)}
				<circle cx={x} cy={y} r={20 + (i % 3) * 6} fill="var(--color-cinnabar-300)" opacity="0.32" />
			{/each}
			<g fill="var(--color-ink-800)" stroke="var(--color-gold-500)" stroke-width="2">
				<path d="M600 742h70l-14-16h-42Z" />
				<path d="M610 742v26M660 742v26" />
			</g>
		</g>

		<!-- bottom-centre-right: battlefield haze (Chiến Trường) -->
		<ellipse cx="990" cy="760" rx="170" ry="80" fill="url(#wm-war)" />
		<g stroke="var(--color-mist-300)" stroke-opacity="0.45" stroke-width="3">
			<path d="M930 790l30-70M960 800l-10-80M1030 795l20-75M1060 805l-25-70" />
		</g>

		<!-- right-bottom: moon gate portal (Bí Cảnh) -->
		<circle cx="1380" cy="700" r="110" fill="url(#wm-portal)" class="pulse" />
		<path d="M1330 790v-90a50 50 0 0 1 100 0v90" fill="none" stroke="var(--color-spirit-violet)" stroke-width="5" />
		<path d="M1310 790h140" stroke="var(--color-mist-300)" stroke-opacity="0.5" stroke-width="4" />

		<!-- drifting cloud sea (two bands) -->
		<g class="cloud-a" fill="url(#wm-cloud)">
			<ellipse cx="200" cy="640" rx="320" ry="60" /><ellipse cx="900" cy="620" rx="380" ry="54" /><ellipse cx="1500" cy="650" rx="300" ry="58" />
			<ellipse cx="1900" cy="640" rx="320" ry="60" />
		</g>
		<g class="cloud-b" fill="url(#wm-cloud)">
			<ellipse cx="0" cy="860" rx="420" ry="80" /><ellipse cx="760" cy="880" rx="460" ry="70" /><ellipse cx="1500" cy="870" rx="420" ry="76" />
			<ellipse cx="2100" cy="860" rx="420" ry="80" />
		</g>

		<!-- spirit motes -->
		{#each motes as m, i (i)}
			<circle class="mote" cx={m.x} cy={m.y} r={m.r} fill="var(--color-spirit-cyan)" style:animation-delay="{m.d}s" />
		{/each}
	</svg>
	{/if}

	<!-- location plaques (desktop/tablet: over the art) -->
	{#if !cover}
	<ul class="hidden sm:block">
		{#each mapLocations as l (l.screen)}
			{@const s = screens[l.screen]}
			<li class="absolute w-max -translate-x-1/2 -translate-y-1/2" style:left="{l.x}%" style:top="{l.y}%">
				<button type="button" class="plaque" class:is-soon={s.status === 'soon'} onclick={() => enter(l.screen)}>
					<span class="ring"><Icon name={s.status === 'soon' ? 'lock' : s.icon} size={18} /></span>
					<span class="text-left">
						<span class="block font-display text-lg leading-tight font-bold">{l.title}</span>
						<span class="block text-xs text-mist-300">{l.subtitle}</span>
					</span>
				</button>
			</li>
		{/each}
	</ul>
	{/if}
</div>
</div>

<!-- mobile: the map is a banner, places are a list (thumb-friendly) -->
{#if !cover}
<ul class="mt-3 grid grid-cols-2 gap-2 sm:hidden">
	{#each mapLocations as l (l.screen)}
		{@const s = screens[l.screen]}
		<li>
			<button type="button" class="plaque w-full" class:is-soon={s.status === 'soon'} onclick={() => enter(l.screen)}>
				<span class="ring"><Icon name={s.status === 'soon' ? 'lock' : s.icon} size={18} /></span>
				<span class="min-w-0 text-left">
					<span class="block truncate font-display text-base leading-tight font-bold">{l.title}</span>
					<span class="block truncate text-xs text-mist-300">{l.subtitle}</span>
				</span>
			</button>
		</li>
	{/each}
</ul>
{/if}

<style>
	/* The map keeps 16:9 (plaques are positioned in %) and fits whichever side is tighter. */
	.map {
		width: 100%;
		aspect-ratio: 16 / 9;
		border: 1px solid color-mix(in oklab, var(--color-gold-500) 25%, transparent);
	}
	.fit-box.is-cover,
	.fit-box.is-cover .map {
		width: 100%;
		height: 100%;
		aspect-ratio: auto;
		border: 0;
	}
	@media (min-width: 640px) {
		.fit-box:not(.is-cover) {
			height: 100%;
			container-type: size;
			display: grid;
			place-items: center;
		}
		.fit-box:not(.is-cover) .map {
			width: min(100cqw, 177.78cqh);
		}
	}
	.layer {
		position: absolute;
		inset: -3%;
		width: 106%;
		height: 106%;
		object-fit: cover;
		pointer-events: none;
		/* deeper layers move less; transform only */
		transform: translate(calc(var(--px) * var(--d) * -6px), calc(var(--py) * var(--d) * -3px));
		transition: transform 400ms var(--ease-qi);
	}
	.landmark {
		position: absolute;
		width: 15%;
		transform: translate(-50%, -50%);
		pointer-events: none;
		filter: drop-shadow(0 6px 14px var(--color-ink-950));
	}
	.landmark.is-soon {
		filter: grayscale(1) brightness(0.6);
	}
	@media (prefers-reduced-motion: reduce) {
		.layer {
			transform: none;
		}
	}
	.plaque {
		display: flex;
		align-items: center;
		gap: 0.6rem;
		padding: 0.4rem 0.9rem 0.4rem 0.4rem;
		color: var(--color-gold-300);
		border: 1px solid color-mix(in oklab, var(--color-gold-500) 55%, transparent);
		background: linear-gradient(180deg, color-mix(in oklab, var(--color-ink-800) 92%, transparent), color-mix(in oklab, var(--color-ink-950) 92%, transparent));
		box-shadow: 0 8px 24px -10px var(--color-ink-950);
		border-radius: 2px;
		transition:
			transform 200ms var(--ease-qi),
			box-shadow 200ms;
	}
	.plaque:hover,
	.plaque:focus-visible {
		transform: translateY(-2px);
		box-shadow:
			0 8px 24px -10px var(--color-ink-950),
			0 0 22px -2px color-mix(in oklab, var(--color-gold-300) 55%, transparent);
	}
	.plaque.is-soon {
		color: var(--color-mist-500);
		border-color: color-mix(in oklab, var(--color-mist-500) 40%, transparent);
	}
	.ring {
		display: grid;
		width: 2.25rem;
		height: 2.25rem;
		flex-shrink: 0;
		place-items: center;
		border-radius: 9999px;
		border: 1px solid currentColor;
		background: color-mix(in oklab, var(--color-ink-950) 70%, transparent);
	}
	.rays {
		animation: rays 14s ease-in-out infinite;
	}
	@keyframes rays {
		0%,
		100% {
			opacity: 0.55;
			transform: rotate(-2deg);
		}
		50% {
			opacity: 1;
			transform: rotate(2deg);
		}
	}
	.cloud-a {
		animation: drift 60s linear infinite;
	}
	.cloud-b {
		animation: drift 90s linear infinite reverse;
	}
	@keyframes drift {
		from {
			transform: translateX(0);
		}
		to {
			transform: translateX(-400px);
		}
	}
	.pulse {
		animation: pulse 5s ease-in-out infinite;
		transform-box: fill-box;
		transform-origin: center;
	}
	@keyframes pulse {
		0%,
		100% {
			opacity: 0.45;
			transform: scale(0.95);
		}
		50% {
			opacity: 0.9;
			transform: scale(1.05);
		}
	}
	.mote {
		animation: rise 7s linear infinite;
		opacity: 0;
	}
	@keyframes rise {
		0% {
			opacity: 0;
			transform: translateY(0);
		}
		20% {
			opacity: 0.9;
		}
		100% {
			opacity: 0;
			transform: translateY(-140px);
		}
	}
</style>
