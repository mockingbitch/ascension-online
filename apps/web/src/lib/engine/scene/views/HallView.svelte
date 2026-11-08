<!--
	Tử Tiêu Cung scene overlay: Hồng Quân on the dais with his aura, the listeners on the cushions (server seats laid
	out by the def's hallSeats rule), chat bubbles over their heads and the linh khí stream from the dais to the
	player's cushion — all positioned through the renderer. Seat, bubble and listening data come from the hall store
	(server state); the page owns the subtitles, the ngộ đạo control and the chat.
	Ambient: aura breathes (full: scale + glow, lite: glow, off: static). The qi stream (dais → my cushion) and the smoke
	puff when a new listener's seat appears are particles of the scene field (full: dense, lite: sparse, off: none);
	speech bubbles pop in with a light bounce (lite: fade, off: instant).
	3D (GĐ7, renderer=3d): the 3D hall draws Hồng Quân and the cushions itself (at the same anchors), so the DOM sage
	(kept invisible, as a measurable point) and the DOM cushions of seats the def knows are hidden; listeners, cards, bubbles and the qi stream stay DOM/canvas.
-->
<script lang="ts">
	import { untrack } from 'svelte';
	import { fade } from 'svelte/transition';
	import { mote, smokePuff } from '$lib/engine/fx/presets';
	import { MediaQuery } from 'svelte/reactivity';
	import Icon from '$lib/components/Icon.svelte';
	import { art } from '$lib/game/art';
	import { hall } from '$lib/game/lecture.svelte';
	import { game } from '$lib/game/store.svelte';
	import { HALL_DEFAULT_SEATS, hallSeats } from '../defs/tu_tieu_cung';
	import { every, type SceneViewProps } from '../view.svelte';

	let { scene, quality }: SceneViewProps = $props();

	const sage = art('characters/hong-quan');
	const sitter = art('characters/listener');
	const phone = new MediaQuery('max-width: 639px');

	const cat = $derived(game.catalogue);
	const me = $derived(game.character);
	const seats = $derived(hallSeats(hall.state?.seats ?? HALL_DEFAULT_SEATS));
	const placed = $derived(seats.map((s) => ({ ...s, p: scene.project(s) })));
	const mySeat = $derived(hall.state?.me.inHall && hall.state.me.seat >= 0 ? placed[hall.state.me.seat] : undefined);
	const listening = $derived((hall.insight?.phase ?? 'listening') === 'listening');
	const sageAt = $derived(scene.at('hong_quan'));
	// depth order inside the hall's own stacking context: lower on the floor = in front (normalised scene y × 100)
	const depth = (y: number, mine = false) => Math.round(y * 100) + (mine ? 50 : 0);
	const sageDepth = $derived(depth(scene.def.anchors['hong_quan']?.y ?? 0));
	const dais = $derived(scene.at('dais'));
	const stream = $derived(listening && mySeat?.p && dais ? { from: dais, to: mySeat.p } : null);
	const qiMote = { ...mote, size: [1.4, 2.6] as [number, number] };
	const is3d = $derived(scene.renderer.kind === '3d');

	// linh khí from Hồng Quân's dais to the player's cushion while listening (path particles, slight wobble)
	$effect(() => {
		if (!stream) return;
		return scene.fx.addEmitter(
			every(
				() => (quality === 'full' ? 340 : quality === 'lite' ? 900 : 0),
				() => {
					const s = stream;
					if (s) scene.fx.path(qiMote, { from: s.from, to: { x: s.to.x, y: s.to.y - scene.unit * 3 }, ms: 3200, jitter: scene.unit * 2 }, 1);
				}
			)
		);
	});

	// a listener sits down (WS lecture.enter after the first state): a puff of smoke where the seat appears
	let known: Set<string> | null = null;
	$effect(() => {
		const now = Object.entries(hall.seats);
		if (!hall.state) return;
		untrack(() => {
			const ids = new Set(now.map(([, l]) => l.characterId));
			if (known)
				for (const [seat, l] of now) {
					if (known.has(l.characterId)) continue;
					const p = placed[Number(seat)]?.p;
					if (p) scene.fx.burst(smokePuff, p.x, p.y - scene.unit * 2);
				}
			known = ids;
		});
	});

	const pop = (_node: Element) => {
		if (quality === 'off') return { duration: 0 };
		const lite = quality === 'lite';
		return {
			duration: lite ? 200 : 420,
			css: (t: number) => {
				// light bounce 0.6 → 1.1 → 1 (full); fade only (lite)
				const s = lite ? 1 : t < 0.7 ? 0.6 + (0.5 * t) / 0.7 : 1.1 - (0.1 * (t - 0.7)) / 0.3;
				return `opacity: ${Math.min(1, t * 2)}; transform: translateX(-50%) translateY(${(1 - t) * 6}px) scale(${s})`;
			}
		};
	};
</script>

<div class="hall" data-quality={quality} data-renderer={is3d ? '3d' : undefined} style:--u="{scene.unit}px">
	{#if sageAt}
		<div class="sage" class:is-phone={phone.current} class:is-3d={is3d} style:left="{sageAt.x}px" style:top="{sageAt.y}px" style:z-index={sageDepth} aria-hidden="true">
			<span class="aura"></span>
			{#if sage}
				<img src={sage.src} srcset={sage.srcset} alt="" draggable="false" />
			{:else}
				<span class="sage-fallback"><Icon name="spark" size={48} /></span>
			{/if}
		</div>
	{/if}


	{#if is3d && dais}
		<!-- 3D: the qi source as a measurable point (the 3D hall draws the dais itself) -->
		<span class="probe" data-anchor="dais" style:left="{dais.x}px" style:top="{dais.y}px" aria-hidden="true"></span>
	{/if}

	<ul class="seats" aria-label="Đạo hữu đang nghe giảng">
		{#each placed as s (s.seat)}
			{@const l = hall.seats[s.seat]}
			{@const mine = !!l && l.characterId === me?.id}
			{@const b = l ? hall.bubbles[l.characterId] : undefined}
			{#if s.p}
				<li
					class="seat"
					class:is-mine={mine}
					class:is-empty={!l}
					style:left="{s.p.x}px"
					style:top="{s.p.y}px"
					style:--s={mine ? s.scale * 1.3 : s.scale}
					style:z-index={depth(s.y, mine)}
				>
					{#if mine}<span class="halo" aria-hidden="true"></span>{/if}
					{#if !is3d || !scene.def.anchors[`seat.${s.seat}`]}<span class="cushion" aria-hidden="true"></span>{/if}
					{#if l}
						{#if sitter}
							<img class="sitter" src={sitter.src} srcset={sitter.srcset} alt="" draggable="false" />
						{:else}
							<span class="sitter-fallback" aria-hidden="true"></span>
						{/if}
						{#if mine}
							<span class="me-tag">Bạn · {me?.name}</span>
						{:else}
							<span class="card" aria-hidden="true">
								<strong>{l.name}</strong>
								<span>{cat?.stageLabel(l.stageId) ?? l.stageId}</span>
							</span>
						{/if}
						<span class="sr-only">{l.name}, {cat?.stageLabel(l.stageId) ?? ''}</span>
						{#if b}
							{#key b.id}
								<span class="bubble" in:pop out:fade={{ duration: quality === 'off' ? 0 : 300 }}>
									<strong class="text-gold-300">{l.name}:</strong>
									{b.text}
								</span>
							{/key}
						{/if}
					{/if}
				</li>
			{/if}
		{/each}
	</ul>
</div>

<style>
	.hall,
	.seats {
		position: absolute;
		inset: 0;
	}
	/* Hồng Quân: feet on the `hong_quan` anchor */
	.sage {
		position: absolute;
		width: calc(var(--u) * 13);
		transform: translate(-50%, -100%);
	}
	/* 3D: the 3D hall draws Hồng Quân; the element stays (invisible) as the anchor's measurable point */
	.sage.is-3d,
	.probe {
		visibility: hidden;
	}
	.probe {
		position: absolute;
	}
	.sage.is-phone {
		width: calc(var(--u) * 15);
	}
	.sage img {
		position: relative;
		width: 100%;
		filter: drop-shadow(0 0 calc(var(--u) * 1.2) color-mix(in oklab, var(--color-spirit-violet) 60%, transparent));
	}
	.aura {
		position: absolute;
		inset: -12% -18% 0;
		border-radius: 50%;
		background: radial-gradient(
			circle at 50% 55%,
			color-mix(in oklab, var(--color-gold-300) 35%, transparent),
			color-mix(in oklab, var(--color-spirit-violet) 25%, transparent) 45%,
			transparent 70%
		);
		opacity: 0.85;
	}
	[data-quality='full'] .aura {
		animation: breathe 6s var(--ease-in-out-sine) infinite;
	}
	[data-quality='lite'] .aura {
		animation: glow 6s var(--ease-in-out-sine) infinite;
	}
	.sage-fallback {
		display: grid;
		place-items: center;
		aspect-ratio: 1;
		color: var(--color-gold-300);
	}


	/* seats */
	.seat {
		position: absolute;
		width: calc(var(--u) * 6.2 * var(--s));
		aspect-ratio: 1;
		transform: translate(-50%, -88%);
		pointer-events: auto; /* hover card */
	}
	.cushion {
		position: absolute;
		left: 8%;
		right: 8%;
		bottom: 2%;
		height: 26%;
		border-radius: 50%;
		background: radial-gradient(ellipse at 50% 35%, color-mix(in oklab, var(--color-spirit-violet) 45%, var(--color-ink-800)), var(--color-ink-950));
		box-shadow: 0 0 0 1px color-mix(in oklab, var(--color-gold-500) 30%, transparent);
	}
	.seat.is-empty .cushion {
		opacity: 0.5;
	}
	.halo {
		position: absolute;
		left: -20%;
		right: -20%;
		bottom: -8%;
		height: 42%;
		border-radius: 50%;
		background: radial-gradient(ellipse, color-mix(in oklab, var(--color-gold-300) 65%, transparent), transparent 70%);
	}
	[data-quality='full'] .halo {
		animation: breathe 3s var(--ease-qi) infinite;
	}
	.sitter,
	.sitter-fallback {
		position: absolute;
		inset: 0;
		width: 100%;
		filter: drop-shadow(0 0 2px color-mix(in oklab, var(--color-mist-100) 35%, transparent));
	}
	.sitter-fallback {
		inset: 25% 25% 10%;
		width: auto;
		border-radius: 50% 50% 35% 35%;
		background: var(--color-ink-800);
	}
	.seat.is-mine .sitter {
		filter: drop-shadow(0 0 5px var(--color-gold-300));
	}
	.me-tag {
		position: absolute;
		left: 50%;
		top: 100%;
		margin-top: 0.15rem;
		transform: translateX(-50%);
		padding: 0.05rem 0.5rem;
		font-size: clamp(0.65rem, calc(var(--u) * 1), 0.85rem);
		font-weight: 700;
		white-space: nowrap;
		color: var(--color-ink-950);
		border-radius: 9999px;
		background: linear-gradient(180deg, var(--color-gold-300), var(--color-gold-500));
		box-shadow: 0 0 10px color-mix(in oklab, var(--color-gold-300) 50%, transparent);
	}
	.card {
		position: absolute;
		left: 50%;
		bottom: 100%;
		display: flex;
		flex-direction: column;
		margin-bottom: 0.2rem;
		padding: 0.2rem 0.5rem;
		font-size: 0.72rem;
		line-height: 1.3;
		white-space: nowrap;
		color: var(--color-mist-300);
		border: 1px solid color-mix(in oklab, var(--color-gold-500) 35%, transparent);
		background: color-mix(in oklab, var(--color-ink-950) 92%, transparent);
		transform: translateX(-50%);
		opacity: 0;
		pointer-events: none;
		transition: opacity var(--dur-fast);
		z-index: var(--z-local-3);
	}
	.card strong {
		color: var(--color-mist-100);
	}
	.seat:hover .card {
		opacity: 1;
	}
	.bubble {
		position: absolute;
		left: 50%;
		bottom: 100%;
		width: max-content;
		max-width: min(16rem, calc(var(--u) * 26));
		margin-bottom: 0.25rem;
		padding: 0.3rem 0.55rem;
		font-size: clamp(0.68rem, calc(var(--u) * 0.95), 0.85rem);
		line-height: 1.3;
		color: var(--color-mist-100);
		border: 1px solid color-mix(in oklab, var(--color-gold-500) 45%, transparent);
		border-radius: 10px;
		background: color-mix(in oklab, var(--color-ink-950) 90%, transparent);
		transform: translateX(-50%);
		z-index: var(--z-local-2);
	}
	@keyframes breathe {
		0%, 100% { opacity: 0.7; transform: scale(1); }
		50% { opacity: 1; transform: scale(1.04); }
	}
	@keyframes glow {
		0%, 100% { opacity: 0.7; }
		50% { opacity: 1; }
	}
</style>
