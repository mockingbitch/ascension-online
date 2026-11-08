<!--
	Hồng Hoang world map ART only: the painted map / its sky layer when the raster files exist, else one layered SVG
	(sky, moon, far peaks, drifting cloud sea, floating palace with light rays, cave peak, sect hall, peach pavilion,
	moon-gate portal, battlefield). Fills its box (slice). The playable map is the world scene (SceneLayer +
	WorldView: plaques via anchors); this component is the login backdrop and the scene's no-art fallback.
	Only transform/opacity animate (clouds, rays, motes).
-->
<script lang="ts">
	import { art } from '$lib/game/art';

	const motes = Array.from({ length: 22 }, (_, i) => ({
		x: (i * 73) % 1600,
		y: 180 + ((i * 131) % 520),
		r: 1.2 + (i % 3) * 0.8,
		d: (i * 0.53) % 7
	}));
	const stars = Array.from({ length: 60 }, (_, i) => ({ x: (i * 197) % 1600, y: (i * 89) % 330, r: 0.6 + (i % 4) * 0.35, o: 0.25 + (i % 5) * 0.12 }));

	// Raster art (docs/design/art-prompts.md §1–2): the painted map, else the sky layer, else the SVG below.
	const img = art('world/map') ?? art('world/sky');
</script>

<div class="map relative size-full overflow-hidden">
	{#if img}
		<img src={img.src} srcset={img.srcset} alt="" class="absolute inset-0 size-full object-cover" draggable="false" />
	{:else}
	<svg viewBox="0 0 1600 900" class="block h-full w-full" preserveAspectRatio="xMidYMid slice" aria-hidden="true">
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
</div>

<style>
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
