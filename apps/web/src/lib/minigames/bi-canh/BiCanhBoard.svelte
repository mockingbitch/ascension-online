<!--
	Bí Cảnh — the playable realm (plain canvas 2D, one requestAnimationFrame loop, no engine). Keyboard: WASD / arrow keys
	(8 directions). Touch / mouse: press anywhere and drag — a floating joystick; the drag direction picks one of 8
	directions (dead zone + hysteresis so a thumb near a sector edge does not jitter). The phi kiếm fights on its own.
	Runs the deterministic simulator at a fixed tick; each tick the Recorder writes the wanted direction into the RLE
	move log under the cadence rule. It decides nothing: at the end it hands the log + its end state to the page, which
	sends it to the server; only the server's replay verdict and loot count.
-->
<script lang="ts">
	import { onMount } from 'svelte';
	import Icon from '$lib/components/Icon.svelte';
	import type { BiCanhParams, SecretRealmClaim } from '$lib/game/types';
	import { drawField, readPalette, type Burst, type Palette } from './render';
	import { BiCanhGame, Recorder, SUB, type EndReason } from './sim';

	interface Props {
		params: BiCanhParams;
		seed: number;
		practice?: boolean;
		onend: (claim: SecretRealmClaim, local: { won: boolean; reason: EndReason; score: number; kills: number; qi: number }) => void;
	}
	let { params, seed, practice = false, onend }: Props = $props();

	// svelte-ignore state_referenced_locally
	const g = new BiCanhGame(params, seed);
	// svelte-ignore state_referenced_locally
	const rec = new Recorder(params);

	let canvas = $state<HTMLCanvasElement>();
	let host = $state<HTMLDivElement>();
	let boxW = $state(0);
	let boxH = $state(0);
	let pal: Palette | null = null;

	let score = $state(0);
	let tick = $state(0);
	let hp = $state(0);
	let qi = $state(0);
	let level = $state(0);
	let paused = $state(false);
	let countdown = $state(3);
	let ended = $state(false);
	let reducedMotion = $state(false);
	let levelFlash = $state(0);
	let stick = $state<{ ox: number; oy: number; x: number; y: number } | null>(null);

	let bursts: Burst[] = [];
	let hurt = 0;

	const scale = $derived(Math.max(0.3, Math.min(boxW / params.arena.width, boxH / params.arena.height)));
	const fieldW = $derived(Math.floor(params.arena.width * scale));
	const fieldH = $derived(Math.floor(params.arena.height * scale));
	const ticksLeft = $derived(Math.max(0, params.timeLimitTicks - tick));
	const timeText = $derived.by(() => {
		const s = Math.ceil((ticksLeft * params.tickMs) / 1000);
		return `${Math.floor(s / 60)}:${String(s % 60).padStart(2, '0')}`;
	});
	const nextQi = $derived(params.levels[level + 1]?.qi ?? null);
	const qiPct = $derived.by(() => {
		if (nextQi === null) return 100;
		const from = params.levels[level]!.qi;
		return Math.min(100, ((qi - from) / (nextQi - from)) * 100);
	});

	function syncHud() {
		score = g.score;
		tick = g.tick;
		hp = g.hp;
		qi = g.qi;
		level = g.level;
	}

	// ───── input → wanted direction (0..8) ─────
	const keys = new Set<string>();
	let stickDir = 0;
	const KEYMAP: Record<string, [number, number]> = {
		KeyW: [0, -1], ArrowUp: [0, -1], KeyS: [0, 1], ArrowDown: [0, 1], KeyA: [-1, 0], ArrowLeft: [-1, 0], KeyD: [1, 0], ArrowRight: [1, 0]
	};
	// dx,dy ∈ {-1,0,1} → direction 0..8 (1 = N, clockwise).
	const DIRS: Record<string, number> = { '0,-1': 1, '1,-1': 2, '1,0': 3, '1,1': 4, '0,1': 5, '-1,1': 6, '-1,0': 7, '-1,-1': 8 };

	function keyDir(): number {
		let x = 0;
		let y = 0;
		for (const k of keys) {
			const v = KEYMAP[k];
			if (v) {
				x += v[0];
				y += v[1];
			}
		}
		x = Math.sign(x);
		y = Math.sign(y);
		return DIRS[`${x},${y}`] ?? 0;
	}

	function wanted(): number {
		return stick ? stickDir : keyDir();
	}

	/** Sector of a drag vector with hysteresis: keep the current direction while within ±30° of its centre. */
	function sector(dx: number, dy: number, cur: number): number {
		const len = Math.hypot(dx, dy);
		if (len < 14) return 0; // dead zone (css px)
		const ang = (Math.atan2(dx, -dy) * 180) / Math.PI; // 0 = up, clockwise
		const norm = (a: number) => ((a % 360) + 360) % 360;
		if (cur > 0) {
			const centre = (cur - 1) * 45;
			const diff = Math.abs(((norm(ang - centre) + 180) % 360) - 180);
			if (diff <= 30) return cur;
		}
		return 1 + (Math.round(norm(ang) / 45) % 8);
	}

	function down(e: PointerEvent) {
		if (ended || paused || countdown > 0 || !e.isPrimary) return;
		e.preventDefault();
		canvas?.setPointerCapture(e.pointerId);
		const r = canvas!.getBoundingClientRect();
		stick = { ox: e.clientX - r.left, oy: e.clientY - r.top, x: e.clientX - r.left, y: e.clientY - r.top };
		stickDir = 0;
	}
	function move(e: PointerEvent) {
		if (!stick || !e.isPrimary) return;
		const r = canvas!.getBoundingClientRect();
		stick = { ...stick, x: e.clientX - r.left, y: e.clientY - r.top };
		stickDir = sector(stick.x - stick.ox, stick.y - stick.oy, stickDir);
	}
	function up(e: PointerEvent) {
		if (!e.isPrimary) return;
		stick = null;
		stickDir = 0;
	}

	function tickOnce() {
		g.step(rec.sample(wanted()));
		const ev = g.events;
		if (ev.hurt) hurt = 1;
		if (ev.levelUp) levelFlash = 1;
		if (!reducedMotion) {
			const k = scale / SUB;
			for (const kd of ev.killed) bursts.push({ x: kd.x * k, y: kd.y * k, life: 1, color: pal?.cyan ?? '' });
		}
	}

	function draw(dt: number, t: number) {
		if (!canvas || !pal) return;
		const ctx = canvas.getContext('2d');
		if (!ctx) return;
		const dpr = window.devicePixelRatio || 1;
		ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
		for (const b of bursts) b.life -= dt / 380;
		bursts = bursts.filter((b) => b.life > 0);
		hurt = Math.max(0, hurt - dt / 450);
		levelFlash = Math.max(0, levelFlash - dt / 1400);
		drawField(ctx, g, scale, pal, { alpha: 0, bursts, hurt, reducedMotion, timeMs: t });
	}

	$effect(() => {
		if (!canvas) return;
		const dpr = window.devicePixelRatio || 1;
		canvas.width = fieldW * dpr;
		canvas.height = fieldH * dpr;
		draw(0, performance.now());
	});

	function finish() {
		if (ended) return;
		if (g.tick === 0) g.step(rec.sample(0)); // endTick must be ≥ 1
		ended = true;
		stick = null;
		keys.clear();
		syncHud();
		const reason: EndReason = g.over ? (g.reason as EndReason) : 'gave_up';
		onend(
			{ endTick: g.tick, kills: g.kills, score: g.score, stateHash: g.hash(), moves: rec.runs.map((r) => [r[0], r[1]] as [number, number]) },
			{ won: g.won, reason, score: g.score, kills: g.kills, qi: g.qi }
		);
	}

	export function giveUp() {
		finish();
	}

	function onkeydown(e: KeyboardEvent) {
		if (ended) return;
		if (e.code === 'KeyP' || e.code === 'Escape') {
			e.preventDefault();
			if (!e.repeat) togglePause();
			return;
		}
		if (KEYMAP[e.code]) {
			e.preventDefault();
			keys.add(e.code);
		}
	}
	function onkeyup(e: KeyboardEvent) {
		keys.delete(e.code);
	}
	function togglePause() {
		paused = !paused;
		stick = null;
		keys.clear();
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
				stick = null;
				keys.clear();
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
			draw(dt, t);
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

<svelte:window {onkeydown} {onkeyup} />

<div class="bicanh grid h-full min-h-0 grid-cols-[minmax(0,1fr)] gap-3 lg:grid-cols-[minmax(0,1fr)_240px]" bind:this={host}>
	<div class="flex min-h-0 flex-col gap-2">
		<div class="hud flex min-w-0 items-center gap-2.5 px-2.5 py-1.5 text-sm sm:gap-4 sm:px-3" aria-live="off">
			<div class="shrink-0" aria-label="Hộ thể còn {hp} trên {params.player.hp}">
				<p class="eyebrow whitespace-nowrap">Hộ thể</p>
				<p class="flex h-7 items-center gap-1">
					{#each Array.from({ length: params.player.hp }, (_, i) => i) as i (i)}
						<span class="ward" class:is-out={i >= hp} aria-hidden="true"></span>
					{/each}
				</p>
			</div>
			<div class="shrink-0">
				<p class="eyebrow whitespace-nowrap">Còn lại</p>
				<p class="text-xl leading-tight font-semibold tabular-nums" class:text-jade-300={ticksLeft * params.tickMs < 20000} class:text-mist-100={ticksLeft * params.tickMs >= 20000}>
					{timeText}
				</p>
			</div>
			<div class="min-w-0 shrink-0">
				<p class="eyebrow whitespace-nowrap">Điểm</p>
				<p class="text-xl leading-tight font-semibold text-gold-300 tabular-nums">{score}</p>
			</div>
			<div class="qi ml-auto flex min-w-0 flex-col items-end gap-1" class:is-flash={levelFlash > 0}>
				<span class="text-xs font-semibold whitespace-nowrap">Kiếm ý {level + 1}</span>
				<span class="meter" role="progressbar" aria-valuemin={0} aria-valuemax={100} aria-valuenow={Math.round(qiPct)} aria-label="Linh khí tới kiếm ý kế tiếp">
					<span style:width="{qiPct}%"></span>
				</span>
			</div>
		</div>

		<div class="relative min-h-0 flex-1" bind:clientWidth={boxW} bind:clientHeight={boxH}>
			<div class="absolute inset-0 grid place-items-center">
				<div class="frame relative" style:width="{fieldW}px" style:height="{fieldH}px">
					<canvas
						bind:this={canvas}
						style:width="{fieldW}px"
						style:height="{fieldH}px"
						aria-label="Bí cảnh: di chuyển để né tà khí, phi kiếm tự xuất, gom linh khí"
						onpointerdown={down}
						onpointermove={move}
						onpointerup={up}
						onpointercancel={up}
						onlostpointercapture={up}
						oncontextmenu={(e) => e.preventDefault()}
					></canvas>
					{#if stick}
						<span class="stick-base" style:left="{stick.ox}px" style:top="{stick.oy}px" aria-hidden="true"></span>
						<span
							class="stick-knob"
							style:left="{stick.ox + Math.max(-36, Math.min(36, stick.x - stick.ox))}px"
							style:top="{stick.oy + Math.max(-36, Math.min(36, stick.y - stick.oy))}px"
							aria-hidden="true"
						></span>
					{/if}
					{#if levelFlash > 0 && !paused && !ended}
						<p class="flash font-display" style:opacity={reducedMotion ? 1 : levelFlash}>Kiếm ý tăng tiến</p>
					{/if}
					{#if countdown > 0 && !paused}
						<div class="veil grid place-items-center p-4 text-center">
							<div>
								<p class="font-display text-6xl font-bold text-gold-300">{countdown}</p>
								<p class="mt-2 text-sm text-mist-300">Nhấn giữ rồi kéo để di chuyển (hoặc WASD / phím mũi tên). Phi kiếm tự xuất.</p>
							</div>
						</div>
					{:else if paused && !ended}
						<div class="veil grid place-items-center p-4 text-center">
							<div>
								<p class="font-display text-3xl font-bold text-gold-300">Tạm dừng</p>
								<p class="mt-1 text-sm text-mist-300">Thời gian phiên vẫn trôi theo giờ máy chủ.</p>
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

	<aside class="hidden space-y-3 lg:block">
		<div class="side space-y-2 p-3 text-sm text-mist-300">
			<p class="eyebrow mb-1">Tà khí</p>
			{#each params.enemies as e (e.id)}
				<p><span class="dot {e.id}"></span> {e.name}: {e.points} điểm{e.hp > 1 ? `, ${e.hp} kiếm` : ''}</p>
			{/each}
			<p class="text-xs text-mist-500">Hạ tà khí rơi linh khí. Linh khí nâng kiếm ý: phi kiếm nhanh hơn, rồi kiếm trận xoay quanh thân.</p>
		</div>
		<div class="side space-y-1 p-3 text-sm text-mist-300">
			<p class="eyebrow mb-1">Điều khiển</p>
			<p><kbd>W</kbd><kbd>A</kbd><kbd>S</kbd><kbd>D</kbd> / phím mũi tên</p>
			<p>Hoặc nhấn giữ và kéo trên trận đồ</p>
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
			0 10px 40px -10px color-mix(in oklab, var(--color-spirit-violet) 25%, transparent);
	}
	.frame canvas {
		display: block;
		touch-action: none;
		user-select: none;
		-webkit-user-select: none;
		cursor: crosshair;
	}
	.meter {
		display: block;
		width: 56px;
		height: 4px;
		border-radius: 2px;
		background: var(--color-ink-600);
		overflow: hidden;
	}
	.meter > span {
		display: block;
		height: 100%;
		background: var(--color-spirit-cyan);
		transition: width 150ms var(--ease-qi, ease);
	}
	.qi {
		color: var(--color-mist-300);
	}
	.qi.is-flash {
		color: var(--color-gold-300);
	}
	.ward {
		width: 10px;
		height: 14px;
		clip-path: polygon(50% 0, 100% 25%, 85% 100%, 15% 100%, 0 25%);
		background: var(--color-jade-300);
	}
	.ward.is-out {
		background: var(--color-ink-600);
	}
	.stick-base,
	.stick-knob {
		position: absolute;
		border-radius: 50%;
		transform: translate(-50%, -50%);
		pointer-events: none;
	}
	.stick-base {
		width: 84px;
		height: 84px;
		border: 1px solid color-mix(in oklab, var(--color-gold-500) 50%, transparent);
		background: color-mix(in oklab, var(--color-ink-950) 30%, transparent);
	}
	.stick-knob {
		width: 34px;
		height: 34px;
		background: color-mix(in oklab, var(--color-gold-300) 55%, transparent);
	}
	.flash {
		position: absolute;
		left: 50%;
		top: 12%;
		transform: translateX(-50%);
		font-size: 1.4rem;
		font-weight: 700;
		color: var(--color-gold-300);
		pointer-events: none;
		white-space: nowrap;
		text-shadow: 0 0 12px var(--color-ink-950);
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
		vertical-align: -1px;
		background: var(--color-spirit-violet);
	}
	.dot.hoa_linh {
		background: var(--color-cinnabar-300);
	}
	.dot.thach_quy {
		background: var(--color-mist-500);
	}
	.dot.ta_tuong {
		background: var(--color-ink-600);
		outline: 2px solid var(--color-cinnabar-300);
	}
	kbd {
		display: inline-block;
		min-width: 1.5em;
		margin-right: 2px;
		padding: 0 4px;
		border: 1px solid var(--color-ink-600);
		border-radius: 3px;
		font-size: 0.75rem;
		text-align: center;
		color: var(--color-mist-100);
	}
</style>
