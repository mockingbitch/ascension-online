<!--
	Thế Giới scene overlay: place plaques (hotspots), landmarks and lantern glows at the world def's anchors, plus
	the cheap ambient life of §4.3 — three drifting cloud bands (parallax by speed), flickering lanterns, light rays
	breathing over Tử Tiêu Cung, peach petals drifting at Đào Hoa Đình (scene particles), a flock of birds crossing now
	and then, and a glow under a place on hover/focus or when it is suggested (breakthrough ready → Động Phủ).
	Full: all of it · Lite: 1 slow band, slow glow, few petals, rare birds · Off: static. Anchored things hide while the
	camera zooms toward a place (they would not follow the CSS camera tween).
-->
<script lang="ts">
	import MapPlaque from '$lib/components/MapPlaque.svelte';
	import WorldMap from '$lib/components/WorldMap.svelte';
	import { art } from '$lib/game/art';
	import { mapLocations, screens } from '$lib/game/nav';
	import { WORLD_LANTERNS } from '../defs/world';
	import { petal } from '$lib/engine/fx/presets';
	import { regions } from '$lib/engine/layout/regions.svelte';
	import { relaxLabels } from '$lib/engine/layout/regions';
	import { game } from '$lib/game/store.svelte';
	import { every, type SceneViewProps } from '../view.svelte';

	let { scene, quality, phase }: SceneViewProps = $props();

	const hasArt = $derived(scene.def.layers.some((l) => !!art(l.art)));
	const tl = $derived(scene.project({ x: 0, y: 0 }));
	const br = $derived(scene.project({ x: 1, y: 1 }));
	const settled = $derived(phase === 'idle' || phase === 'reveal');
	const lanterns = Object.keys(WORLD_LANTERNS).map((id, i) => ({ id, delay: (i * 0.77) % 2.9, dur: 2.2 + ((i * 0.61) % 1.6), violet: id.endsWith('gate') }));
	const places = mapLocations.map((l) => ({ ...l, landmarkArt: art(`landmarks/${l.landmark}`) }));

	// a place worth a visit glows (server state: breakthrough ready / idle qi full → Động Phủ)
	const suggested = $derived.by(() => {
		const ch = game.character;
		if (game.preview?.ready) return 'cultivate';
		if (ch && Date.now() - Date.parse(ch.lastCultivatedAt) >= ch.offlineCapSeconds * 1000) return 'cultivate';
		return null;
	});
	// plaques stay inside the stage (clear of HUD / rail / the chat dock) and never on top of each other (UI polish U2):
	// anchors are projected by the renderer, then nudged apart with the measured plaque sizes (pure relaxLabels).
	let root = $state<HTMLDivElement>();
	const dense = $derived(scene.unit < 9); // map narrower than ~900 px: title-only plaques
	const sizes = $state<Record<string, { w: number; h: number }>>({});
	const plaqueAt = $derived.by(() => {
		const pts = mapLocations.map((l) => ({ id: l.screen, p: scene.at(`loc.${l.screen}`) }));
		const el = root;
		if (!el) return new Map<string, { x: number; y: number }>();
		const o = el.getBoundingClientRect();
		const st = regions.stage;
		const local = (r: { left: number; right: number; top: number; bottom: number }) => ({ left: r.left - o.left, right: r.right - o.left, top: r.top - o.top, bottom: r.bottom - o.top });
		const bounds = st.right > st.left ? local(st) : { left: 0, top: 0, right: o.width, bottom: o.height };
		const placed = pts.filter((x) => x.p && sizes[x.id]);
		const out = relaxLabels(
			placed.map((x) => ({ x: x.p!.x, y: x.p!.y, w: sizes[x.id]!.w, h: sizes[x.id]!.h })),
			{ left: Math.max(bounds.left, 0), top: Math.max(bounds.top, 0), right: Math.min(bounds.right, o.width), bottom: Math.min(bounds.bottom, o.height) },
			{ avoid: [...(regions.chat ? [regions.chat] : []), ...[...document.querySelectorAll('[data-scene-avoid]')].map((e) => e.getBoundingClientRect())].map(local) }
		);
		const m = new Map<string, { x: number; y: number }>();
		placed.forEach((x, i) => m.set(x.id, out[i]!));
		for (const x of pts) if (x.p && !m.has(x.id)) m.set(x.id, x.p);
		return m;
	});
	const hallAt = $derived(scene.at('loc.hall'));
	const garden = $derived(scene.at('loc.companion'));

	// peach petals drifting down around Đào Hoa Đình
	$effect(() =>
		scene.fx.addEmitter(
			every(
				() => (quality === 'full' ? 650 : quality === 'lite' ? 2200 : 0),
				() => {
					const g = garden;
					if (!g) return;
					const u = scene.unit;
					scene.fx.burst(petal, g.x + (Math.random() - 0.6) * u * 12, g.y - u * (6 + Math.random() * 6), 1);
				}
			)
		)
	);
</script>

<div bind:this={root} class="world" data-quality={quality} class:is-moving={!settled} style:--u="{scene.unit}px">
	{#if !hasArt && tl && br}
		<div class="fallback" style:left="{tl.x}px" style:top="{tl.y}px" style:width="{br.x - tl.x}px" style:height="{br.y - tl.y}px">
			<WorldMap />
		</div>
	{/if}

	<div class="anchored">
		{#each places as l (l.screen)}
			{#if l.landmarkArt}
				{@const p = scene.project({ x: l.x / 100, y: l.y / 100 - 0.09 })}
				{#if p}
					<img
						src={l.landmarkArt.src}
						srcset={l.landmarkArt.srcset}
						alt=""
						class="landmark"
						class:is-soon={screens[l.screen].status === 'soon'}
						style:left="{p.x}px"
						style:top="{p.y}px"
						draggable="false"
					/>
				{/if}
			{/if}
		{/each}
		{#each lanterns as ln (ln.id)}
			{@const p = scene.at(ln.id)}
			{#if p}<span class="lantern" class:is-violet={ln.violet} style:left="{p.x}px" style:top="{p.y}px" style:--delay="{ln.delay}s" style:--dur="{ln.dur}s"></span>{/if}
		{/each}
	</div>

	{#if hallAt}
		<!-- Tử Tiêu Cung: light rays fanning up from the palace, breathing -->
		<div class="rays anchored" style:left="{hallAt.x}px" style:top="{hallAt.y}px" aria-hidden="true"><span></span></div>
	{/if}

	<!-- now and then a small flock crosses the sky (transform only) -->
	<div class="flight" aria-hidden="true">
		<svg class="flock" viewBox="0 0 60 24" fill="none" stroke="currentColor" stroke-width="1.4" stroke-linecap="round">
			<path class="wing" d="M4 10q4-4 8 0q4-4 8 0" />
			<path class="wing w2" d="M24 16q3-3 6 0q3-3 6 0" />
			<path class="wing w3" d="M40 6q3-3 6 0q3-3 6 0" />
		</svg>
	</div>

	<!-- cloud sea drifting over the map: three bands at different speeds read as depth -->
	<div class="clouds" aria-hidden="true">
		<span class="band b1"></span>
		<span class="band b2"></span>
		<span class="band b3"></span>
	</div>

	<ul class="plaques anchored" aria-label="Địa điểm trên bản đồ">
		{#each mapLocations as l (l.screen)}
			{@const p = plaqueAt.get(l.screen)}
			{#if p}
				<li
					style:left="{p.x}px"
					style:top="{p.y}px"
					class:is-suggested={suggested === l.screen}
					data-place={l.screen}
					bind:clientWidth={() => sizes[l.screen]?.w ?? 0, (w) => (sizes[l.screen] = { w, h: sizes[l.screen]?.h ?? 0 })}
					bind:clientHeight={() => sizes[l.screen]?.h ?? 0, (h) => (sizes[l.screen] = { w: sizes[l.screen]?.w ?? 0, h })}
				>
					<span class="spot" aria-hidden="true"></span>
					<MapPlaque screen={l.screen} title={l.title} subtitle={l.subtitle} {dense} />
				</li>
			{/if}
		{/each}
	</ul>
</div>

<style>
	.world,
	.anchored,
	.clouds {
		position: absolute;
		inset: 0;
	}
	.fallback {
		position: absolute;
	}
	.anchored {
		transition: opacity var(--dur-fast) var(--ease-qi);
	}
	.is-moving .anchored {
		opacity: 0;
	}
	.is-moving .plaques {
		visibility: hidden;
	}

	/* plaques: desktop/tablet only — phones get the place list under the map (page UI) */
	.plaques li {
		position: absolute;
		width: max-content;
		transform: translate(-50%, -50%);
	}
	@media (max-width: 639px) {
		.plaques {
			display: none;
		}
	}
	/* hotspot glow: under the plaque, shown on hover/focus; a suggested place breathes */
	.spot {
		position: absolute;
		left: 50%;
		top: 50%;
		width: calc(var(--u) * 16);
		aspect-ratio: 1;
		border-radius: 50%;
		background: radial-gradient(circle, color-mix(in oklab, var(--color-gold-300) 45%, transparent), transparent 65%);
		transform: translate(-50%, -50%) scale(0.8);
		opacity: 0;
		pointer-events: none;
		transition:
			opacity var(--dur-fast) var(--ease-qi),
			transform var(--dur-fast) var(--ease-qi);
		z-index: var(--z-under);
	}
	.plaques li:hover .spot,
	.plaques li:focus-within .spot {
		opacity: 1;
		transform: translate(-50%, -50%) scale(1);
	}
	.plaques li.is-suggested .spot {
		opacity: 0.8;
	}
	[data-quality='full'] .plaques li.is-suggested .spot,
	[data-quality='lite'] .plaques li.is-suggested .spot {
		animation: suggest 2.4s var(--ease-in-out-sine) infinite;
	}
	@keyframes suggest {
		50% {
			opacity: 0.35;
			transform: translate(-50%, -50%) scale(0.9);
		}
	}

	/* Tử Tiêu Cung light rays */
	.rays {
		position: absolute;
		inset: auto;
		width: calc(var(--u) * 26);
		height: calc(var(--u) * 22);
		transform: translate(-50%, -88%);
		pointer-events: none;
		mix-blend-mode: screen;
	}
	.rays span {
		position: absolute;
		inset: 0;
		background: repeating-conic-gradient(
			from -40deg at 50% 100%,
			color-mix(in oklab, var(--color-gold-300) 30%, transparent) 0 5deg,
			transparent 5deg 14deg
		);
		mask-image: radial-gradient(ellipse at 50% 100%, var(--color-ink-950) 15%, transparent 70%);
		clip-path: polygon(0 0, 100% 0, 50% 100%);
		opacity: 0.55;
		transform-origin: 50% 100%;
	}
	[data-quality='full'] .rays span {
		animation: rays 5.5s var(--ease-in-out-sine) infinite;
	}
	[data-quality='lite'] .rays span {
		animation: rays-lite 8s var(--ease-in-out-sine) infinite;
	}
	@keyframes rays {
		50% {
			opacity: 0.9;
			transform: scale(1.06, 1.1);
		}
	}
	@keyframes rays-lite {
		50% {
			opacity: 0.8;
		}
	}

	/* birds: a full-width track moves across; the flock is only visible in the first part of the cycle */
	.flight {
		position: absolute;
		left: 0;
		top: 14%;
		width: 100%;
		height: calc(var(--u) * 3);
		pointer-events: none;
		opacity: 0;
		color: var(--color-ink-950);
	}
	.flock {
		height: 100%;
		width: auto;
		transform: translateX(-100%);
	}
	[data-quality='full'] .flight {
		animation: cross 34s linear 6s infinite;
	}
	[data-quality='lite'] .flight {
		animation: cross 80s linear 20s infinite;
	}
	[data-quality='full'] .wing {
		animation: flap 0.5s var(--ease-in-out-sine) infinite alternate;
		transform-box: fill-box;
		transform-origin: 50% 100%;
	}
	.w2 {
		animation-delay: 0.15s !important;
	}
	.w3 {
		animation-delay: 0.3s !important;
	}
	@keyframes cross {
		0% {
			opacity: 0;
			transform: translate(0, 0);
		}
		2%,
		30% {
			opacity: 0.75;
		}
		32%,
		100% {
			opacity: 0;
			transform: translate(115%, -40%);
		}
	}
	@keyframes flap {
		to {
			transform: scaleY(0.4);
		}
	}

	.landmark {
		position: absolute;
		width: calc(var(--u) * 15);
		transform: translate(-50%, -50%);
		filter: drop-shadow(0 6px 14px var(--color-ink-950));
	}
	.landmark.is-soon {
		filter: grayscale(1) brightness(0.6);
	}

	/* lantern glows: soft additive dots over lit windows */
	.lantern {
		position: absolute;
		width: calc(var(--u) * 2.4);
		aspect-ratio: 1;
		margin: calc(var(--u) * -1.2);
		border-radius: 50%;
		background: radial-gradient(circle, color-mix(in oklab, var(--color-gold-300) 75%, transparent), transparent 65%);
		mix-blend-mode: screen;
		opacity: 0.7;
	}
	.lantern.is-violet {
		background: radial-gradient(circle, color-mix(in oklab, var(--color-spirit-violet) 75%, transparent), transparent 65%);
	}
	[data-quality='full'] .lantern {
		animation: flicker var(--dur) steps(1, end) var(--delay) infinite, glow calc(var(--dur) * 1.7) var(--ease-in-out-sine) infinite alternate;
	}
	[data-quality='lite'] .lantern {
		animation: glow 4s var(--ease-in-out-sine) infinite alternate;
	}
	@keyframes flicker {
		0%, 100% { opacity: 0.75; }
		12% { opacity: 0.5; }
		16% { opacity: 0.85; }
		47% { opacity: 0.62; }
		51% { opacity: 0.9; }
		78% { opacity: 0.58; }
	}
	@keyframes glow {
		from { transform: scale(0.9); }
		to { transform: scale(1.12); }
	}

	/* cloud bands: 200%-wide strips of soft ellipses translated by -50% (seamless loop), transform only */
	.clouds {
		overflow: hidden;
	}
	.band {
		position: absolute;
		left: 0;
		width: 200%;
		background-repeat: repeat-x;
		background-size: 50% 100%;
		will-change: transform;
	}
	.b1 {
		top: 52%;
		height: 22%;
		opacity: 0.55;
		background-image:
			radial-gradient(ellipse 22% 40% at 18% 55%, color-mix(in oklab, var(--color-mist-100) 16%, transparent), transparent 70%),
			radial-gradient(ellipse 28% 32% at 62% 45%, color-mix(in oklab, var(--color-mist-100) 12%, transparent), transparent 70%);
	}
	.b2 {
		top: 70%;
		height: 26%;
		opacity: 0.6;
		background-image:
			radial-gradient(ellipse 30% 40% at 30% 50%, color-mix(in oklab, var(--color-mist-100) 14%, transparent), transparent 70%),
			radial-gradient(ellipse 20% 34% at 80% 60%, color-mix(in oklab, var(--color-mist-100) 12%, transparent), transparent 70%);
	}
	.b3 {
		top: 84%;
		height: 24%;
		opacity: 0.7;
		background-image: radial-gradient(ellipse 34% 46% at 50% 55%, color-mix(in oklab, var(--color-mist-100) 18%, transparent), transparent 70%);
	}
	[data-quality='full'] .b1 {
		animation: drift 140s linear infinite;
	}
	[data-quality='full'] .b2 {
		animation: drift 95s linear infinite reverse;
	}
	[data-quality='full'] .b3 {
		animation: drift 60s linear infinite;
	}
	[data-quality='lite'] .b1,
	[data-quality='lite'] .b3 {
		display: none;
	}
	[data-quality='lite'] .b2 {
		animation: drift 160s linear infinite;
	}
	@keyframes drift {
		from { transform: translate3d(0, 0, 0); }
		to { transform: translate3d(-50%, 0, 0); }
	}
</style>
