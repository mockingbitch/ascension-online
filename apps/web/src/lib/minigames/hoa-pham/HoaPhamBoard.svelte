<!--
	Hóa Phàm — the playable field (plain canvas 2D, one requestAnimationFrame loop, no engine). Touch-first: swipe to
	cut; works with a mouse (press and drag). Runs the deterministic simulator at a fixed tick (params.tickMs); each
	tick the pointer position (while pressed) is sampled by the Recorder, which applies the game's brush rules and
	writes the delta-encoded stroke log. It decides nothing: at the end it hands the log + its end state to the page,
	which sends it to the server; only the server's replay verdict counts.
-->
<script lang="ts">
	import { onMount } from 'svelte';
	import Icon from '$lib/components/Icon.svelte';
	import type { HoaPhamParams, TrialClaim } from '$lib/game/types';
	import { drawField, readPalette, type Palette, type Piece, type TrailPoint } from './render';
	import { HoaPhamGame, Recorder, type EndReason } from './sim';

	interface Props {
		params: HoaPhamParams;
		seed: number;
		/** practice: local seed, nothing is sent */
		practice?: boolean;
		onend: (claim: TrialClaim, local: { won: boolean; reason: EndReason; score: number; cuts: number }) => void;
	}
	let { params, seed, practice = false, onend }: Props = $props();

	// svelte-ignore state_referenced_locally
	const g = new HoaPhamGame(params, seed);
	// svelte-ignore state_referenced_locally
	const rec = new Recorder(params);

	let canvas = $state<HTMLCanvasElement>();
	let host = $state<HTMLDivElement>();
	let boxW = $state(0);
	let boxH = $state(0);
	let pal: Palette | null = null;

	// HUD (updated once per frame from the simulator; small component)
	let score = $state(0);
	let tick = $state(0);
	let lanternsLeft = $state(0);
	let charge = $state(0);
	let calmLeft = $state(0);
	let combo = $state(0);
	let paused = $state(false);
	let countdown = $state(3);
	let ended = $state(false);
	let reducedMotion = $state(false);

	// Display-only effects.
	let pieces: Piece[] = [];
	let trail: TrailPoint[] = [];
	let hurt = 0;
	let comboShow = $state(0);

	const scale = $derived(Math.max(1.5, Math.min(boxW / params.width, boxH / params.height)));
	const fieldW = $derived(Math.floor(params.width * scale));
	const fieldH = $derived(Math.floor(params.height * scale));
	const ticksLeft = $derived(Math.max(0, params.timeLimitTicks - tick));
	const timeText = $derived.by(() => {
		const s = Math.ceil((ticksLeft * params.tickMs) / 1000);
		return `${Math.floor(s / 60)}:${String(s % 60).padStart(2, '0')}`;
	});
	const holdText = $derived.by(() => {
		if (score < params.passScore || tick >= params.minPassTicks) return '';
		return `Giữ tâm thêm ${Math.ceil(((params.minPassTicks - tick) * params.tickMs) / 1000)}s`;
	});
	const calmPct = $derived(calmLeft > 0 ? (calmLeft / params.calm.durationTicks) * 100 : (charge / params.calm.chargeMax) * 100);
	const scorePct = $derived(Math.min(100, (score / params.passScore) * 100));

	function syncHud() {
		score = g.score;
		tick = g.tick;
		lanternsLeft = g.lanternsLeft;
		charge = g.charge;
		calmLeft = g.calmLeft;
	}

	// Pointer state (raw, fractional grid units); sampled once per tick.
	let pointer: { x: number; y: number } | null = null;
	let strokeNo = 0;

	function toGrid(e: PointerEvent): { x: number; y: number } {
		const r = canvas!.getBoundingClientRect();
		return { x: ((e.clientX - r.left) / r.width) * params.width, y: ((e.clientY - r.top) / r.height) * params.height };
	}
	function down(e: PointerEvent) {
		if (ended || paused || countdown > 0 || !e.isPrimary) return;
		e.preventDefault();
		canvas?.setPointerCapture(e.pointerId);
		pointer = toGrid(e);
	}
	function move(e: PointerEvent) {
		if (pointer && e.isPrimary) pointer = toGrid(e);
	}
	function up(e: PointerEvent) {
		if (!e.isPrimary) return;
		pointer = null;
	}

	function tickOnce() {
		const wasIn = rec.inStroke;
		const pt = rec.sample(g.tick, pointer);
		if (pt && !wasIn) strokeNo++;
		g.step(pt !== null, pt?.[0] ?? 0, pt?.[1] ?? 0);
		if (pt) trail.push({ x: pt[0] * scale, y: pt[1] * scale, age: 0, stroke: strokeNo });
		const ev = g.events;
		if (ev.cut.length > 0) {
			combo = g.strokeCuts;
			comboShow = combo >= 2 ? 1 : comboShow;
		}
		if (ev.lantern.length > 0) hurt = 1;
		if (!reducedMotion) {
			const k = scale / 32;
			for (const c of ev.cut) {
				const len = Math.hypot(c.dx, c.dy) || 1;
				const nx = -c.dy / len;
				const ny = c.dx / len;
				const rot = Math.atan2(c.dy, c.dx);
				for (const half of [-1, 1] as const) {
					pieces.push({ type: c.obj.type, id: c.obj.id, x: c.obj.x * k, y: c.obj.y * k, vx: nx * half * 0.05, vy: ny * half * 0.05 - 0.02,
						rot, half, life: 1 });
				}
			}
		}
	}

	function draw(alpha: number, dt: number, t: number) {
		if (!canvas || !pal) return;
		const ctx = canvas.getContext('2d');
		if (!ctx) return;
		const dpr = window.devicePixelRatio || 1;
		ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
		for (const p of pieces) {
			p.x += p.vx * dt;
			p.y += p.vy * dt;
			p.vy += 0.0004 * dt;
			p.life -= dt / 450;
		}
		pieces = pieces.filter((p) => p.life > 0);
		for (const tp of trail) tp.age += dt;
		trail = trail.filter((tp) => tp.age < 300);
		hurt = Math.max(0, hurt - dt / 400);
		comboShow = Math.max(0, comboShow - dt / 900);
		drawField(ctx, g, scale, pal, { alpha, pieces, trail, hurt, reducedMotion, timeMs: t });
	}

	$effect(() => {
		if (!canvas) return;
		const dpr = window.devicePixelRatio || 1;
		canvas.width = fieldW * dpr;
		canvas.height = fieldH * dpr;
		draw(0, 0, performance.now());
	});

	function finish() {
		if (ended) return;
		if (g.tick === 0) g.step(false); // endTick must be ≥ 1
		ended = true;
		pointer = null;
		syncHud();
		const reason: EndReason = g.over ? (g.reason as EndReason) : 'gave_up';
		// Only strokes that ended before the end tick count; the recorder never logs at or past g.tick.
		onend(
			{ endTick: g.tick, lines: g.cuts, score: g.score, stateHash: g.hash(), inputs: [], strokes: rec.strokes.map((s) => s.slice()) },
			{ won: g.won, reason, score: g.score, cuts: g.cuts }
		);
	}

	export function giveUp() {
		finish();
	}

	function onkey(e: KeyboardEvent) {
		if (ended) return;
		if (e.code === 'KeyP' || e.code === 'Escape') {
			e.preventDefault();
			if (!e.repeat) togglePause();
		}
	}
	function togglePause() {
		paused = !paused;
		pointer = null;
	}

	onMount(() => {
		pal = readPalette(host ?? document.documentElement);
		const mq = window.matchMedia('(prefers-reduced-motion: reduce)');
		reducedMotion = mq.matches;
		const onmq = () => (reducedMotion = mq.matches);
		mq.addEventListener('change', onmq);
		const onvis = () => {
			if (document.hidden && !ended) {
				paused = true; // never let ticks run unseen
				pointer = null;
			}
		};
		document.addEventListener('visibilitychange', onvis);
		window.addEventListener('blur', onvis);

		syncHud();
		let raf = 0;
		let last = performance.now();
		let acc = 0;
		let cdLeft = 3000;
		const frame = (t: number) => {
			const dt = Math.min(250, t - last);
			last = t;
			if (!paused && !ended) {
				if (cdLeft > 0) {
					cdLeft -= dt;
					countdown = Math.max(0, Math.ceil(cdLeft / 1000));
				} else {
					acc += dt;
					while (acc >= params.tickMs && !g.over) {
						acc -= params.tickMs;
						tickOnce();
					}
				}
			}
			syncHud();
			draw(paused || ended ? 0 : Math.min(1, acc / params.tickMs), dt, t);
			if (g.over) finish();
			if (!ended) raf = requestAnimationFrame(frame);
		};
		raf = requestAnimationFrame(frame);
		return () => {
			cancelAnimationFrame(raf);
			mq.removeEventListener('change', onmq);
			document.removeEventListener('visibilitychange', onvis);
			window.removeEventListener('blur', onvis);
		};
	});
</script>

<svelte:window onkeydown={onkey} />

<div class="hoapham grid h-full min-h-0 grid-cols-[minmax(0,1fr)] gap-3 lg:grid-cols-[minmax(0,1fr)_240px]" bind:this={host}>
	<div class="flex min-h-0 flex-col gap-2">
		<!-- HUD -->
		<div class="hud flex min-w-0 items-center gap-2.5 px-2.5 py-1.5 text-sm sm:gap-4 sm:px-3" aria-live="off">
			<div class="min-w-0 shrink-0">
				<p class="eyebrow whitespace-nowrap">Trần duyên</p>
				<p class="text-xl leading-tight font-semibold text-jade-300 tabular-nums">
					{score}<span class="text-sm text-mist-500">/{params.passScore}</span>
				</p>
				<span class="bar" role="progressbar" aria-valuemin={0} aria-valuemax={params.passScore} aria-valuenow={score} aria-label="Điểm trần duyên">
					<span style:width="{scorePct}%"></span>
				</span>
			</div>
			<div class="shrink-0">
				<p class="eyebrow whitespace-nowrap">Còn lại</p>
				<p class="text-xl leading-tight font-semibold tabular-nums" class:text-cinnabar-300={ticksLeft * params.tickMs < 20000} class:text-mist-100={ticksLeft * params.tickMs >= 20000}>
					{timeText}
				</p>
			</div>
			<div class="shrink-0" aria-label="Tâm đăng còn {lanternsLeft} trên {params.lanterns}">
				<p class="eyebrow whitespace-nowrap">Tâm đăng</p>
				<p class="flex h-7 items-center gap-1">
					{#each Array.from({ length: params.lanterns }, (_, i) => i) as i (i)}
						<span class="lamp" class:is-out={i >= lanternsLeft} aria-hidden="true"></span>
					{/each}
				</p>
			</div>
			<div class="calm ml-auto flex min-w-0 flex-col items-end gap-1" class:is-active={calmLeft > 0}>
				<span class="text-xs font-semibold whitespace-nowrap">{calmLeft > 0 ? 'Tâm tĩnh' : 'Tĩnh'}</span>
				<span class="meter" role="progressbar" aria-valuemin={0} aria-valuemax={100} aria-valuenow={Math.round(calmPct)} aria-label="Tâm tĩnh">
					<span style:width="{calmPct}%"></span>
				</span>
			</div>
		</div>

		<!-- Field -->
		<div class="relative min-h-0 flex-1" bind:clientWidth={boxW} bind:clientHeight={boxH}>
			<div class="absolute inset-0 grid place-items-center">
				<div class="frame relative" style:width="{fieldW}px" style:height="{fieldH}px">
					<canvas
						bind:this={canvas}
						style:width="{fieldW}px"
						style:height="{fieldH}px"
						aria-label="Hóa phàm: vuốt để chém nút trần duyên, tránh đèn chân tâm"
						onpointerdown={down}
						onpointermove={move}
						onpointerup={up}
						onpointercancel={up}
						onlostpointercapture={up}
						oncontextmenu={(e) => e.preventDefault()}
					></canvas>
					{#if comboShow > 0 && combo >= 2 && !paused && !ended}
						<p class="combo font-display" style:opacity={reducedMotion ? 1 : comboShow}>Liên đoạn ×{combo}</p>
					{/if}
					{#if holdText && !paused && !ended}
						<p class="hold" aria-live="polite">Đủ trần duyên — {holdText}</p>
					{/if}
					{#if countdown > 0 && !paused}
						<div class="veil grid place-items-center p-4 text-center">
							<div>
								<p class="font-display text-6xl font-bold text-gold-300">{countdown}</p>
								<p class="mt-2 text-sm text-mist-300">Vuốt qua nút trần duyên để chém. Đừng chạm đèn chân tâm.</p>
							</div>
						</div>
					{:else if paused && !ended}
						<div class="veil grid place-items-center p-4 text-center">
							<div>
								<p class="font-display text-3xl font-bold text-gold-300">Tạm dừng</p>
								<p class="mt-1 text-sm text-mist-300">Thời gian thử luyện vẫn trôi theo giờ máy chủ.</p>
								<button type="button" class="btn btn-jade mt-4" onclick={() => (paused = false)}><Icon name="lotus" size={16} /> Tiếp tục</button>
							</div>
						</div>
					{/if}
				</div>
			</div>
		</div>

		<div class="flex items-center gap-2 lg:hidden">
			<button type="button" class="btn btn-ghost h-10 flex-1 text-sm" disabled={ended} onclick={togglePause}>{paused ? 'Tiếp tục' : 'Tạm dừng'}</button>
			{#if practice}<p class="flex-1 text-xs text-mist-500">Luyện tập — không tính</p>{/if}
		</div>
	</div>

	<!-- Desktop side panel -->
	<aside class="hidden space-y-3 lg:block">
		<div class="side space-y-2 p-3 text-sm text-mist-300">
			<p class="eyebrow mb-1">Cách chém</p>
			<p><span class="dot thread"></span> Nút trần duyên: 1 điểm</p>
			<p><span class="dot fibre"></span> Tơ vương (nhỏ): 2 điểm</p>
			<p><span class="dot net"></span> Lưới nghiệp: 3 điểm, cần 2 nét</p>
			<p><span class="dot lamp-dot"></span> Đèn chân tâm: <b class="text-cinnabar-300">đừng chém</b></p>
			<p class="text-xs text-mist-500">Một nét chém nhiều nút được cộng thêm (liên đoạn). Nét phải dứt khoát; rê chậm không đứt.</p>
		</div>
		<div class="side space-y-1 p-3 text-sm text-mist-300">
			<p class="eyebrow mb-1">Điều khiển</p>
			<p>Nhấn giữ chuột và kéo (hoặc vuốt) để chém</p>
			<p><kbd>P</kbd> / <kbd>Esc</kbd> tạm dừng</p>
		</div>
		<button type="button" class="btn btn-ghost w-full" disabled={ended} onclick={togglePause}>{paused ? 'Tiếp tục' : 'Tạm dừng'}</button>
		{#if practice}<p class="text-xs text-mist-500">Luyện tập: thế cục ngẫu nhiên, không gửi máy chủ, không tính.</p>{/if}
	</aside>
</div>

<style>
	.hud,
	.side {
		border: 1px solid color-mix(in oklab, var(--color-gold-500) 22%, transparent);
		background: color-mix(in oklab, var(--color-ink-950) 55%, transparent);
	}
	.frame {
		box-shadow:
			0 0 0 1px color-mix(in oklab, var(--color-gold-500) 40%, transparent),
			0 0 0 5px var(--color-ink-950),
			0 0 0 6px color-mix(in oklab, var(--color-gold-500) 18%, transparent),
			0 10px 40px -10px color-mix(in oklab, var(--color-spirit-cyan) 25%, transparent);
	}
	.frame canvas {
		display: block;
		touch-action: none;
		user-select: none;
		-webkit-user-select: none;
		cursor: crosshair;
	}
	.bar,
	.meter {
		display: block;
		height: 4px;
		border-radius: 2px;
		background: var(--color-ink-600);
		overflow: hidden;
	}
	.bar {
		width: 64px;
		margin-top: 2px;
	}
	.meter {
		width: 48px;
	}
	.bar > span,
	.meter > span {
		display: block;
		height: 100%;
		transition: width 150ms var(--ease-qi, ease);
	}
	.bar > span {
		background: var(--color-jade-300);
	}
	.meter > span {
		background: var(--color-spirit-cyan);
	}
	.calm {
		color: var(--color-mist-300);
	}
	.calm.is-active {
		color: var(--color-spirit-cyan);
	}
	.lamp {
		width: 12px;
		height: 16px;
		border-radius: 45%;
		background: radial-gradient(circle at 50% 60%, var(--color-gold-300), var(--color-gold-500) 70%);
		box-shadow: 0 0 8px -1px color-mix(in oklab, var(--color-gold-300) 70%, transparent);
	}
	.lamp.is-out {
		background: var(--color-ink-600);
		box-shadow: none;
		outline: 1px dashed var(--color-mist-500);
	}
	.combo {
		position: absolute;
		left: 50%;
		top: 18%;
		transform: translateX(-50%);
		font-size: 1.6rem;
		font-weight: 700;
		color: var(--color-gold-300);
		pointer-events: none;
		white-space: nowrap;
		text-shadow: 0 0 12px var(--color-ink-950);
	}
	.hold {
		position: absolute;
		left: 50%;
		top: 8px;
		transform: translateX(-50%);
		padding: 2px 10px;
		white-space: nowrap;
		font-size: 0.8rem;
		color: var(--color-jade-300);
		background: color-mix(in oklab, var(--color-ink-950) 80%, transparent);
		border: 1px solid color-mix(in oklab, var(--color-jade-500) 50%, transparent);
		pointer-events: none;
	}
	.veil {
		position: absolute;
		inset: 0;
		background: color-mix(in oklab, var(--color-ink-950) 72%, transparent);
	}
	.dot {
		display: inline-block;
		width: 10px;
		height: 10px;
		margin-right: 6px;
		border-radius: 50%;
		border: 2px solid currentColor;
		vertical-align: -1px;
	}
	.dot.thread {
		color: var(--color-cinnabar-300);
	}
	.dot.fibre {
		color: var(--color-jade-300);
	}
	.dot.net {
		color: var(--color-spirit-violet);
	}
	.dot.lamp-dot {
		border: none;
		background: var(--color-gold-300);
	}
	kbd {
		display: inline-block;
		min-width: 1.5em;
		padding: 0 4px;
		border: 1px solid var(--color-ink-600);
		border-radius: 3px;
		font-size: 0.75rem;
		text-align: center;
		color: var(--color-mist-100);
	}
</style>
