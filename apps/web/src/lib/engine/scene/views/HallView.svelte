<!--
	Tử Tiêu Cung scene overlay: Hồng Quân on the dais with his aura, the listeners of the player's điện on its cushions
	(up to 60, decisions 2026-11-10: everyone in a điện is seated; server seats laid out by the def's hallSeats rule,
	rows depth-scaled), chat bubbles over their heads and the linh khí stream from the dais to the player's cushion —
	all positioned through the renderer. The player's own figure is always drawn (raised above every other seat) and
	highlighted (halo, gold ring, "Bạn" tag). Names are styled by realm tier (RealmName: fx_tier 1–6). Seat, bubble and listening data come from the hall store
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
	import RealmName from '$lib/components/RealmName.svelte';
	import SeatFigure from '$lib/components/SeatFigure.svelte';
	import { art } from '$lib/game/art';
	import { hall } from '$lib/game/lecture.svelte';
	import { MAX_BUBBLES, PHONE_BUBBLES, coveredLabels, focusAround, figureRect, headPoint, labelRects, planBubbles, planNameLabels, tailPoints, type LabelInput } from '$lib/game/hallOverlay';
	import { regions } from '$lib/engine/layout/regions.svelte';
	import { intersects, type Rect } from '$lib/engine/layout/regions';
	import { game } from '$lib/game/store.svelte';
	import { HALL_DEFAULT_SEATS, fitTransform, hallSeats, type SeatFit } from '../defs/tu_tieu_cung';
	import { settings } from '$lib/engine/settings.svelte';
	import { every, type SceneViewProps } from '../view.svelte';

	let { scene, quality }: SceneViewProps = $props();

	const sage = art('characters/hong-quan');
	const phone = new MediaQuery('max-width: 639px');
	// phones and short landscape screens: at most PHONE_BUBBLES bubbles, smaller and clamped to two lines
	const cramped = new MediaQuery('(max-width: 639px), (max-height: 480px)');

	const cat = $derived(game.catalogue);
	const me = $derived(game.character);
	// short landscape screens: the seat block is squeezed into the free floor above the subtitles and right of the chat
	// dock (measured below; 2D only — the 3D hall builds its cushions at the def's fixed anchors)
	let fit = $state<SeatFit | null>(null);
	$effect(() => {
		hall.seatFit = scene.renderer.kind === '3d' ? null : fit;
	});
	const seats = $derived(hallSeats(hall.state?.capacity || HALL_DEFAULT_SEATS, scene.renderer.kind === '3d' ? null : fit));
	const placed = $derived(seats.map((s) => ({ ...s, p: scene.project(s) })));
	const mySeat = $derived(hall.state?.me.inHall && hall.state.me.seat >= 0 ? placed[hall.state.me.seat] : undefined);
	const listening = $derived((hall.insight?.phase ?? 'listening') === 'listening');
	const sageAt = $derived(scene.at('hong_quan'));
	// depth order inside the hall's own stacking context: lower on the floor = in front (normalised scene y × 100)
	const depth = (y: number, mine = false) => Math.round(y * 100) + (mine ? 50 : 0);
	// the hovered/focused seat rises above every other seat (and the sage), so its hover card is never covered by a
	// seat in front of it — each seat is its own stacking context (z-index + transform), so the card's local z can't do it
	const LABELS_Z = depth(1, true) + 5; // name labels: over every seat (a label on a nearer sitter is faded)
	const TAILS_Z = depth(0); // bubble tails: under the seats (and so under the labels and my "Bạn" tag)
	const BUBBLES_Z = depth(1, true) + 10; // speech bubbles over labels (they never cover one: covered labels hide)
	const LIFTED = depth(1, true) + 20; // the hovered/tapped seat's card over everything
	/** sitter size in scene units (× the row's depth scale; my own × 1.3) */
	const SEAT_BASE = 5.6;
	const sizeOf = (scale: number, mine: boolean) => scene.unit * SEAT_BASE * scale * (mine ? 1.3 : 1);
	// phones (< 640 px): only some names until tapped (hallOverlay.phoneShows)
	let tapped = $state<string | null>(null);
	let lifted = $state<number | null>(null);
	let tapTimer: ReturnType<typeof setTimeout> | undefined;
	function tap(id: string | undefined) {
		if (!id) return;
		tapped = tapped === id ? null : id;
		clearTimeout(tapTimer);
		if (tapped) tapTimer = setTimeout(() => (tapped = null), 5000);
	}

	// boxes to keep clear, in the hall box's px: the page's overlays (header, subtitles: [data-scene-avoid]), the chat
	// dock and my "Bạn" tag; bounds = the stage region. Re-read each second (the hall clock) and when seats move.
	let root = $state<HTMLDivElement>();
	const frame = $derived.by(() => {
		void hall.now;
		void placed;
		const el = root;
		if (!el || typeof document === 'undefined') return null;
		const o = el.getBoundingClientRect();
		const local = (r: Rect | DOMRect): Rect => ({ left: r.left - o.left, right: r.right - o.left, top: r.top - o.top, bottom: r.bottom - o.top });
		const st = regions.stage;
		const b = st.right > st.left ? local(st) : { left: 0, top: 0, right: o.width, bottom: o.height };
		const bounds = { left: Math.max(b.left, 0), top: Math.max(b.top, 0), right: Math.min(b.right, o.width), bottom: Math.min(b.bottom, o.height) };
		const avoid = [...document.querySelectorAll('[data-scene-avoid]')].map((e) => local(e.getBoundingClientRect())).filter((r) => r.right > r.left);
		// what is always there (the page overlays + the chat toggle button) vs the whole chat dock box, whose upper part
		// only holds a few seconds of chat peeks: labels / bubbles keep off the whole box, the seat fit and my pin only
		// off the permanent part
		const fixed = [...avoid];
		const toggle = document.querySelector('[data-chat-toggle]')?.getBoundingClientRect();
		if (toggle && toggle.width > 0) fixed.push(local(toggle));
		if (regions.chat && regions.chat.right > regions.chat.left) avoid.push(local(regions.chat));
		// my "Bạn" tag(s): on the cushion, and the pinned copy when the cushion is below the visible stage
		const meTags = [...el.querySelectorAll('[data-me-tag], [data-me-pin]')].map((e) => local(e.getBoundingClientRect())).filter((r) => r.right > r.left);
		const seatTagEl = el.querySelector('li.is-mine [data-me-tag]');
		const seatTag = seatTagEl ? local(seatTagEl.getBoundingClientRect()) : null;
		return { bounds, avoid, fixed, meTags, seatTag };
	});
	const fontPx = $derived(Math.max(10, Math.min(12, scene.unit * 0.95)));
	// the seat under the pointer / tapped open and its nearest neighbours show their names ("Quan trọng")
	const focusId = $derived(tapped ?? (lifted !== null ? (hall.seats[lifted]?.characterId ?? null) : null));
	const focus = $derived(
		focusAround(
			focusId,
			placed.flatMap((s) => {
				const l = hall.seats[s.seat];
				return l && s.p ? [{ id: l.characterId, x: s.p.x, y: s.p.y }] : [];
			})
		)
	);
	const labels = $derived.by(() => {
		const f = frame;
		if (!f) return [];
		const items: LabelInput[] = [];
		const myState = hall.state?.me;
		for (const s of placed) {
			const l = hall.seats[s.seat];
			if (!l || !s.p) continue;
			const mine = l.characterId === me?.id;
			items.push({
				id: l.characterId, x: s.p.x, y: s.p.y, size: sizeOf(s.scale, mine), name: l.name, mine,
				partner: !!myState?.partnerId && l.characterId === myState.partnerId,
				sameFaction: !!myState?.factionId && l.factionId === myState.factionId,
				fxTier: cat?.realmOf(l.stageId)?.visual.fxTier ?? 1,
				spokeAt: hall.spoke[l.characterId] ?? 0
			});
		}
		return planNameLabels(items, { mode: settings.hallNames, now: hall.now, focus, fontPx, obstacles: [...f.avoid, ...f.meTags], bounds: f.bounds });
	});
	const byId = $derived(new Map(Object.values(hall.seats).map((l) => [l.characterId, l])));

	// chat bubbles: measured, then laid out (newest MAX_BUBBLES, apart, off the overlays) with a tail to the speaker
	const bubbleSize = $state<Record<number, { w: number; h: number }>>({});
	const bubbleList = $derived.by(() => {
		const out: { charId: string; seat: (typeof placed)[number]; b: (typeof hall.bubbles)[string] }[] = [];
		for (const [charId, b] of Object.entries(hall.bubbles)) {
			const l = byId.get(charId);
			const seat = l ? placed[l.seat] : undefined;
			if (seat?.p) out.push({ charId, seat, b });
		}
		return out;
	});
	// the occupied seat block (figures) — phones keep bubbles out of it when they can
	const seatBlock = $derived.by(() => {
		let r: Rect | null = null;
		for (const s of placed) {
			const l = hall.seats[s.seat];
			if (!l || !s.p) continue;
			const f = figureRect({ id: l.characterId, x: s.p.x, y: s.p.y, size: sizeOf(s.scale, l.characterId === me?.id) });
			r = r ? { left: Math.min(r.left, f.left), right: Math.max(r.right, f.right), top: Math.min(r.top, f.top), bottom: Math.max(r.bottom, f.bottom) } : f;
		}
		return r;
	});
	// the occupied sitters' heads + bodies (bubbles keep off them when there is room)
	const figures = $derived(
		placed.flatMap((s) => {
			const l = hall.seats[s.seat];
			return l && s.p ? [figureRect({ id: l.characterId, x: s.p.x, y: s.p.y, size: sizeOf(s.scale, l.characterId === me?.id) })] : [];
		})
	);
	const bubblePlan = $derived.by(() => {
		const f = frame;
		if (!f) return new Map();
		const lr = labelRects(labels);
		const block = seatBlock;
		// free space to try besides "above the speaker": above the seat block on both sides of Hồng Quân, and beside it
		const spots = block
			? [
					{ x: block.left + (block.right - block.left) * 0.2, y: block.top - 40 },
					{ x: block.right - (block.right - block.left) * 0.2, y: block.top - 40 },
					{ x: block.left - 90, y: (block.top + block.bottom) / 2 },
					{ x: block.right + 90, y: (block.top + block.bottom) / 2 }
				]
			: [];
		const plans = planBubbles(
			bubbleList.map(({ charId, seat, b }) => {
				const h = headPoint({ id: charId, x: seat.p!.x, y: seat.p!.y, size: sizeOf(seat.scale, charId === me?.id) });
				return { id: b.id, ax: h.x, ay: h.y, w: bubbleSize[b.id]?.w ?? 0, h: bubbleSize[b.id]?.h ?? 0, at: b.at };
			}),
			f.bounds,
			[...f.avoid, ...f.meTags],
			{
				max: cramped.current ? PHONE_BUBBLES : MAX_BUBBLES,
				// labels are soft obstacles (a bubble looks for free room first); phones also keep off the seat block
				// soft obstacles, strongest first: names + sitters (phones: the whole seat block), then names only
				soft: cramped.current && block ? [[...lr, block], lr] : [[...lr, ...figures], lr],
				spots
			}
		);
		return new Map(plans.map((p) => [p.id, p]));
	});
	// my cushion can fall below the visible stage (short landscape screens crop the hall's bottom rows): then a pinned
	// copy of my figure + "Bạn" tag sits at the bottom edge of what is visible, above the subtitles / chat dock
	// measure the free floor on short screens: the subtitles (collapsed) and the chat dock, in painting coordinates
	$effect(() => {
		void hall.now;
		void scene.unit;
		const el = root;
		if (!el || scene.renderer.kind === '3d') return;
		untrack(() => {
			const short = innerHeight <= 480;
			if (!short) {
				if (fit) fit = null;
				return;
			}
			const subs = document.querySelector('[data-hall-subs]');
			if (subs?.hasAttribute('data-open')) return; // expanded on purpose: keep the layout until it collapses
			const p0 = scene.project({ x: 0, y: 0 });
			const p1 = scene.project({ x: 1, y: 1 });
			if (!p0 || !p1 || p1.x === p0.x || p1.y === p0.y) return;
			const o = el.getBoundingClientRect();
			const toY = (screen: number) => (screen - o.top - p0.y) / (p1.y - p0.y);
			const toX = (screen: number) => (screen - o.left - p0.x) / (p1.x - p0.x);
			const st = regions.stage;
			let bottom = Math.min(o.bottom, st.bottom > st.top ? st.bottom : o.bottom);
			if (subs) bottom = Math.min(bottom, subs.getBoundingClientRect().top);
			const base: SeatFit = { bottom: toY(bottom - 30) };
			// the header info box over the top-left: the first row's heads stay below it
			const head = document.querySelector('.overlay-box[data-scene-avoid]')?.getBoundingClientRect();
			if (head && head.bottom > o.top) base.top = toY(head.bottom + scene.unit * 3.5);
			// the chat toggle button in the bottom-left corner: keep the block above it or beside it — whichever leaves the
			// bigger sitters (the dock's chat peeks above the button come and go: not a reason to shrink the hall)
			const tg = document.querySelector('[data-chat-toggle]')?.getBoundingClientRect();
			let next = base;
			if (tg && tg.width > 0 && tg.top < bottom) {
				const above: SeatFit = { ...base, bottom: Math.min(base.bottom!, toY(tg.top - 30)) };
				const beside: SeatFit = { ...base, left: toX(tg.right + 26) };
				next = fitTransform(above).room >= fitTransform(beside).room ? above : beside;
			}
			const keys = ['top', 'bottom', 'left'] as const;
			const same = !!fit && keys.every((k) => Math.abs((fit![k] ?? -1) - (next[k] ?? -1)) < 0.004);
			if (!same) fit = next;
		});
	});

	const PIN_W = 96;
	const PIN_H = 44;
	const mePin = $derived.by(() => {
		const f = frame;
		const s = mySeat;
		if (!f || !s?.p) return null;
		const size = sizeOf(s.scale, true);
		const box = (x: number, y: number): Rect => ({ left: x - PIN_W / 2, right: x + PIN_W / 2, top: y - PIN_H, bottom: y });
		const free = (r: Rect) => r.left >= f.bounds.left && r.right <= f.bounds.right && !f.fixed.some((a) => intersects(a, r));
		// my figure's head and my seat's "Bạn" tag visible and clear of the overlays → no pin (it is only a fallback)
		const headBox: Rect = { left: s.p.x - size * 0.2, right: s.p.x + size * 0.2, top: s.p.y - 0.8 * size, bottom: s.p.y - 0.3 * size };
		const t = f.seatTag;
		const inB = (r: Rect) => r.top >= f.bounds.top && r.bottom <= f.bounds.bottom && r.left >= f.bounds.left && r.right <= f.bounds.right;
		if (t && inB(t) && inB(headBox) && !f.fixed.some((a) => intersects(a, t) || intersects(a, headBox))) return null;
		// else: on the bottom edge, as near my seat's x as possible; failing that, step up past what blocks it
		for (let y = f.bounds.bottom - 4; y > f.bounds.top + PIN_H; y -= 12)
			for (let dx = 0; dx <= f.bounds.right - f.bounds.left; dx += 8)
				for (const x of dx ? [s.p.x - dx, s.p.x + dx] : [s.p.x]) if (free(box(x, y))) return { x, y };
		return null;
	});
	// a label a bubble covers even partly is hidden completely (never half-covered or peeking out)
	const covered = $derived(
		coveredLabels(
			labels,
			[...bubblePlan.values()].filter((p) => p.visible && bubbleSize[p.id]).map((p) => ({ x: p.x, y: p.y, w: bubbleSize[p.id]!.w, h: bubbleSize[p.id]!.h }))
		)
	);
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
				return `transform: translate(-50%, -50%) translateY(${(1 - t) * 6}px) scale(${s})`;
			}
		};
	};
</script>

<div bind:this={root} class="hall" data-quality={quality} data-renderer={is3d ? '3d' : undefined} style:--u="{scene.unit}px">
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
			{#if s.p}
				<li
					class="seat"
					class:is-mine={mine}
					class:is-empty={!l}
					class:is-tapped={!!l && tapped === l.characterId}
					style:left="{s.p.x}px"
					style:top="{s.p.y}px"
					style:--size="{sizeOf(s.scale, mine)}px"
					style:z-index={lifted === s.seat || (!!l && tapped === l.characterId) ? LIFTED : depth(s.y, mine)}
					onpointerenter={() => (lifted = s.seat)}
					onpointerleave={() => lifted === s.seat && (lifted = null)}
				>
					{#if mine}<span class="halo" aria-hidden="true"></span>{/if}
					{#if !is3d || !scene.def.anchors[`seat.${s.seat}`]}<span class="cushion" aria-hidden="true"></span>{/if}
					{#if l}
						<SeatFigure look={l.look} aura={cat?.realmOf(l.stageId)?.visual.aura ?? 'none'} />
						{#if !mine}
							<!-- tap (phones) opens the card and shows the name; hover does the same on desktop -->
							<button type="button" class="hit" data-sfx="off" aria-label="{l.name}, {cat?.stageLabel(l.stageId) ?? ''}" onclick={() => tap(l.characterId)}></button>
						{/if}
						{#if mine}
							<span class="me-tag" data-me-tag><span class="me-label">Bạn ·</span> <RealmName name={me?.name ?? l.name} stageId={me?.stageId ?? l.stageId} /></span>
						{:else}
							<span class="card" aria-hidden="true">
								<RealmName name={l.name} stageId={l.stageId} nameFx={undefined} />
								<span>{cat?.stageLabel(l.stageId) ?? l.stageId}</span>
							</span>
						{/if}
						{#if mine}<span class="sr-only">{l.name}, {cat?.stageLabel(l.stageId) ?? ''}</span>{/if}
					{/if}
				</li>
			{/if}
		{/each}
	</ul>

	{#if mePin && me}
		{@const l = hall.seats[hall.state?.me.seat ?? -1]}
		<div class="me-pin" style:left="{mePin.x}px" style:top="{mePin.y}px" style:width="{PIN_W}px" style:z-index={LIFTED} data-me-pin role="img" aria-label="Bạn, {me.name}: chỗ ngồi ở phía dưới khung nhìn">
			<span class="pin-fig" aria-hidden="true"><SeatFigure look={l?.look} aura={cat?.realmOf(me.stageId)?.visual.aura ?? 'none'} /></span>
			<span class="me-tag is-pin" data-me-tag title={me.name}><span class="me-label">Bạn ↓</span></span>
		</div>
	{/if}

	<!-- name labels: every listener on desktop (collisions faded / stacked), a chosen few on phones (hallOverlay) -->
	<div class="labels" style:z-index={LABELS_Z} style:--fs="{fontPx}px" aria-hidden="true">
		{#each labels as lb (lb.id)}
			{@const l = byId.get(lb.id)}
			{#if l && lb.state !== 'hidden' && !covered.has(lb.id)}
				<span class="nlabel" data-state={lb.state} style:left="{lb.x}px" style:top="{lb.y}px" style:max-width="{lb.w + 8}px" in:fade={{ duration: quality === 'off' ? 0 : 160 }} out:fade={{ duration: quality === 'off' ? 0 : 120 }}>
					<RealmName name={lb.text} stageId={l.stageId} />
				</span>
			{/if}
		{/each}
	</div>

	<!-- bubble tails sit under the seats, labels and my tag (a tail passing them never covers them) -->
	<div class="bubbles" style:z-index={TAILS_Z} aria-hidden="true">
		<svg class="tails" aria-hidden="true">
			{#each bubbleList as { b } (b.id)}
				{@const p = bubblePlan.get(b.id)}
				{@const sz = bubbleSize[b.id]}
				{#if p?.visible && sz}
					<polygon class="tail" points={tailPoints({ x: p.x, y: p.y, w: sz.w, h: sz.h, ax: p.ax, ay: p.ay })} />
				{/if}
			{/each}
		</svg>
	</div>

	<!-- speech bubbles: newest few, laid out apart and off the page overlays, a tail to the speaker -->
	<div class="bubbles" style:z-index={BUBBLES_Z}>
		{#each bubbleList as { charId, b } (b.id)}
			{@const l = byId.get(charId)}
			{@const p = bubblePlan.get(b.id)}
			{#if l}
				<span
					class="bubble"
					class:is-shown={!!p?.visible}
					data-bubble={b.id}
					style:left="{p?.x ?? -9999}px"
					style:top="{p?.y ?? -9999}px"
					bind:offsetWidth={() => bubbleSize[b.id]?.w ?? 0, (w) => (bubbleSize[b.id] = { w, h: bubbleSize[b.id]?.h ?? 0 })}
					bind:offsetHeight={() => bubbleSize[b.id]?.h ?? 0, (h) => (bubbleSize[b.id] = { w: bubbleSize[b.id]?.w ?? 0, h })}
					in:pop
					out:fade={{ duration: quality === 'off' ? 0 : 300 }}
				>
					<RealmName name={l.name} stageId={l.stageId} nameFx={b.nameFx} />:
					{b.text}
				</span>
			{/if}
		{/each}
	</div>
</div>

<style>
	.hall,
	.seats,
	.labels,
	.bubbles {
		position: absolute;
		inset: 0;
	}
	.labels,
	.bubbles {
		pointer-events: none;
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
		width: var(--size);
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
	/* "Bạn · name": a dark pill with a gold ring (the name keeps its realm-tier style, readable on ink) */
	.me-tag {
		position: absolute;
		left: 50%;
		top: 100%;
		margin-top: 0.15rem;
		transform: translateX(-50%);
		padding: 0.05rem 0.55rem;
		font-size: clamp(0.68rem, calc(var(--u) * 1), 0.85rem);
		white-space: nowrap;
		border: 1.5px solid var(--color-gold-300);
		border-radius: 9999px;
		background: color-mix(in oklab, var(--color-ink-950) 92%, transparent);
		box-shadow: 0 0 10px color-mix(in oklab, var(--color-gold-300) 50%, transparent);
	}
	.me-label {
		font-weight: 700;
		color: var(--color-gold-300);
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
	.seat:hover .card {
		opacity: 1;
	}
	/* name label: a small translucent ink pill above the head, name in its realm-tier style */
	.nlabel {
		position: absolute;
		transform: translate(-50%, -50%);
		overflow: hidden;
		padding: 0 0.35em;
		font-size: var(--fs);
		line-height: 1.45;
		white-space: nowrap;
		text-overflow: ellipsis;
		border-radius: 9999px;
		background: color-mix(in oklab, var(--color-ink-950) 55%, transparent);
		box-shadow: 0 0 6px 2px color-mix(in oklab, var(--color-ink-950) 35%, transparent);
		transition: opacity var(--dur-fast) var(--ease-qi);
	}
	.nlabel[data-state='faded'] {
		opacity: 0.3;
	}
	/* bubbles: placed by planBubbles (centre), measured; older than the newest few fade out */
	.bubble {
		position: absolute;
		width: max-content;
		max-width: min(16rem, calc(var(--u) * 26));
		padding: 0.3rem 0.55rem;
		font-size: clamp(0.68rem, calc(var(--u) * 0.95), 0.85rem);
		line-height: 1.3;
		color: var(--color-mist-100);
		border: 1px solid color-mix(in oklab, var(--color-gold-500) 45%, transparent);
		border-radius: 10px;
		background: color-mix(in oklab, var(--color-ink-950) 97%, transparent);
		transform: translate(-50%, -50%);
		opacity: 0;
		transition: opacity var(--dur-base) var(--ease-qi);
	}
	.bubble.is-shown {
		opacity: 1;
	}
	/* phones / short screens: smaller, at most two lines */
	@media (max-width: 639px), (max-height: 480px) {
		.bubble {
			display: -webkit-box;
			max-width: min(13rem, 70vw);
			padding: 0.2rem 0.45rem;
			font-size: 0.8rem;
			-webkit-line-clamp: 2;
			line-clamp: 2;
			-webkit-box-orient: vertical;
			overflow: hidden;
		}
	}
	.tails {
		position: absolute;
		inset: 0;
		width: 100%;
		height: 100%;
		overflow: visible;
	}
	.tail {
		fill: color-mix(in oklab, var(--color-ink-950) 97%, transparent);
		stroke: color-mix(in oklab, var(--color-gold-500) 45%, transparent);
		stroke-width: 1;
	}
	/* my cushion is below the visible stage: a pinned copy at the bottom edge */
	.me-pin {
		position: absolute;
		display: flex;
		align-items: flex-end;
		gap: 0.25rem;
		transform: translate(-50%, -100%);
		pointer-events: none;
	}
	.pin-fig {
		position: relative;
		width: 2.2rem;
		aspect-ratio: 1;
	}
	.me-tag.is-pin {
		position: static;
		transform: none;
		margin: 0 0 0.2rem;
	}
	.hit {
		position: absolute;
		inset: 10% 15% 5%;
		border-radius: 40%;
	}
	.seat.is-tapped .card {
		opacity: 1;
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
