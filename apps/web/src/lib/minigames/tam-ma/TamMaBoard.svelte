<!--
	Vượt Tâm Ma — the playable board (plain canvas 2D, one requestAnimationFrame loop, no engine).
	Runs the deterministic simulator at a fixed tick (params.tickMs) and records every input as [tick, action].
	It decides nothing: when the game ends (or the player gives up) it hands the input log + its end state to the page,
	which sends it to the server; only the server's replay verdict counts.
-->
<script lang="ts">
	import { onMount } from 'svelte';
	import Icon from '$lib/components/Icon.svelte';
	import type { TamMaParams, TrialClaim } from '$lib/game/types';
	import { drawBoard, drawPiece, readPalette, type Palette } from './render';
	import { Act, TamMaGame, windowRoom, type EndReason } from './sim';

	interface Props {
		params: TamMaParams;
		seed: number;
		/** practice: local seed, nothing is sent */
		practice?: boolean;
		/** paused (bindable: the mode frame pauses on Esc/Back; the board pauses itself on P, hidden tab, its buttons) */
		paused?: boolean;
		/** frozen without the pause veil (the mode's 3-2-1 countdown runs over the board) */
		hold?: boolean;
		/** the board's own countdown before the first tick (0 when the mode frame counts down) */
		countdownMs?: number;
		onend: (claim: TrialClaim, local: { won: boolean; reason: EndReason }) => void;
	}
	let { params, seed, practice = false, paused = $bindable(false), hold = false, countdownMs = 3000, onend }: Props = $props();

	// svelte-ignore state_referenced_locally
	const g = new TamMaGame(params, seed);
	const inputs: [number, number][] = [];
	let queue: number[] = [];

	let board = $state<HTMLCanvasElement>();
	let nextA = $state<HTMLCanvasElement>(); // mobile HUD
	let nextB = $state<HTMLCanvasElement>(); // desktop side panel
	let host = $state<HTMLDivElement>();
	let boxW = $state(0);
	let boxH = $state(0);
	let pal: Palette | null = null;

	// HUD (updated once per frame from the simulator; small component)
	let lines = $state(0);
	let tick = $state(0);
	let ticksLeft = $state(0);
	let charge = $state(0);
	let calmLeft = $state(0);
	// svelte-ignore state_referenced_locally
	let countdown = $state(Math.ceil(countdownMs / 1000));
	let ended = $state(false);
	let reducedMotion = $state(false);
	let clearFlash = 0;
	let riseFx = 0;

	const cell = $derived(Math.max(10, Math.floor(Math.min(boxW / params.width, boxH / params.height))));
	const timeText = $derived.by(() => {
		const s = Math.ceil((ticksLeft * params.tickMs) / 1000);
		return `${Math.floor(s / 60)}:${String(s % 60).padStart(2, '0')}`;
	});
	// Pass needs the lines AND having held on until minPassTicks: show how long is left once the lines are done.
	const holdText = $derived.by(() => {
		if (lines < params.passLines || tick >= params.minPassTicks) return '';
		const s = Math.ceil(((params.minPassTicks - tick) * params.tickMs) / 1000);
		return `Giữ tâm thêm ${s}s`;
	});
	const calmReady = $derived(charge >= params.calm.chargeMax && calmLeft === 0);
	const calmPct = $derived(calmLeft > 0 ? (calmLeft / params.calm.durationTicks) * 100 : (charge / params.calm.chargeMax) * 100);

	function syncHud() {
		lines = g.lines;
		tick = g.tick;
		ticksLeft = Math.max(0, params.timeLimitTicks - g.tick);
		charge = g.charge;
		calmLeft = g.calmLeft;
	}

	function draw() {
		if (!board || !pal) return;
		const ctx = board.getContext('2d');
		if (!ctx) return;
		const dpr = window.devicePixelRatio || 1;
		ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
		drawBoard(ctx, g, cell, pal, { clearFlash, rise: riseFx, reducedMotion });
		for (const cv of [nextA, nextB]) {
			const nctx = cv?.getContext('2d');
			if (!nctx) continue;
			nctx.setTransform(dpr, 0, 0, dpr, 0, 0);
			drawPiece(nctx, g, g.next, 56, pal);
		}
	}

	// Backing store follows the CSS size × devicePixelRatio.
	$effect(() => {
		if (!board) return;
		const dpr = window.devicePixelRatio || 1;
		board.width = params.width * cell * dpr;
		board.height = params.height * cell * dpr;
		for (const cv of [nextA, nextB]) {
			if (!cv) continue;
			cv.width = 56 * dpr;
			cv.height = 56 * dpr;
		}
		draw();
	});

	// Rules v2: never log more than maxInputsPerWindow inputs per window (a human cadence; the server rejects more).
	// Excess presses (fast key repeat) wait in a short queue; beyond it they are dropped, never sent late in bulk.
	const QUEUE_MAX = 3;
	function tickOnce() {
		let batch = queue.splice(0, windowRoom(params, inputs, g.tick));
		if (inputs.length + batch.length > params.maxInputs) batch = batch.slice(0, Math.max(0, params.maxInputs - inputs.length));
		for (const a of batch) inputs.push([g.tick, a]);
		g.step(batch);
		if (g.lastCleared > 0) clearFlash = 1;
		if (g.lastRiseTick === g.tick - 1) riseFx = 1;
	}

	function finish() {
		if (ended) return;
		if (g.tick === 0) g.step([]); // endTick must be ≥ 1
		ended = true;
		syncHud();
		draw();
		const reason: EndReason = g.over ? (g.reason as EndReason) : 'gave_up';
		onend({ endTick: g.tick, lines: g.lines, score: g.score, stateHash: g.hash(), inputs: inputs.slice() }, { won: g.won, reason });
	}

	export function giveUp() {
		finish();
	}

	function push(a: number) {
		if (ended || paused || hold || countdown > 0 || queue.length >= QUEUE_MAX) return;
		queue.push(a);
	}

	const keymap: Record<string, { a: number; repeat: boolean }> = {
		ArrowLeft: { a: Act.Left, repeat: true },
		KeyA: { a: Act.Left, repeat: true },
		ArrowRight: { a: Act.Right, repeat: true },
		KeyD: { a: Act.Right, repeat: true },
		ArrowDown: { a: Act.SoftDrop, repeat: true },
		KeyS: { a: Act.SoftDrop, repeat: true },
		ArrowUp: { a: Act.RotateCW, repeat: false },
		KeyX: { a: Act.RotateCW, repeat: false },
		KeyW: { a: Act.RotateCW, repeat: false },
		KeyZ: { a: Act.RotateCC, repeat: false },
		Space: { a: Act.HardDrop, repeat: false },
		KeyC: { a: Act.Calm, repeat: false },
		ShiftLeft: { a: Act.Calm, repeat: false },
		ShiftRight: { a: Act.Calm, repeat: false }
	};

	function onkey(e: KeyboardEvent) {
		if (ended) return;
		const target = e.target as HTMLElement | null;
		if (target && (target.tagName === 'INPUT' || target.tagName === 'TEXTAREA' || target.isContentEditable)) return;
		if (e.code === 'KeyP') {
			e.preventDefault();
			if (!e.repeat) paused = !paused;
			return;
		}
		const m = keymap[e.code];
		if (!m) return;
		e.preventDefault();
		if (e.repeat && !m.repeat) return;
		push(m.a);
	}

	// Touch buttons: press = one input; holding move/soft-drop repeats (180 ms delay, then every 90 ms).
	let holdTimer: ReturnType<typeof setTimeout> | undefined;
	let holdEvery: ReturnType<typeof setInterval> | undefined;
	function press(a: number, repeat: boolean) {
		return (e: PointerEvent) => {
			e.preventDefault();
			push(a);
			release();
			if (repeat) holdTimer = setTimeout(() => (holdEvery = setInterval(() => push(a), 90)), 180);
		};
	}
	function release() {
		clearTimeout(holdTimer);
		clearInterval(holdEvery);
	}

	onMount(() => {
		pal = readPalette(host ?? document.documentElement);
		const mq = window.matchMedia('(prefers-reduced-motion: reduce)');
		reducedMotion = mq.matches;
		const onmq = () => (reducedMotion = mq.matches);
		mq.addEventListener('change', onmq);
		const onvis = () => {
			if (document.hidden && !ended) paused = true; // never let ticks run unseen
		};
		document.addEventListener('visibilitychange', onvis);
		window.addEventListener('blur', onvis);

		syncHud();
		let raf = 0;
		let last = performance.now();
		let acc = 0;
		let cdLeft = countdownMs;
		const frame = (t: number) => {
			const dt = Math.min(250, t - last);
			last = t;
			if (!paused && !ended && !hold) {
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
			clearFlash = Math.max(0, clearFlash - dt / 350);
			riseFx = Math.max(0, riseFx - dt / 250);
			syncHud();
			draw();
			if (g.over) finish();
			if (!ended) raf = requestAnimationFrame(frame);
		};
		raf = requestAnimationFrame(frame);
		return () => {
			cancelAnimationFrame(raf);
			release();
			mq.removeEventListener('change', onmq);
			document.removeEventListener('visibilitychange', onvis);
			window.removeEventListener('blur', onvis);
		};
	});

	const controls = [
		{ a: Act.Left, label: 'Sang trái', icon: '◀', repeat: true },
		{ a: Act.RotateCW, label: 'Xoay', icon: '↻', repeat: false },
		{ a: Act.Right, label: 'Sang phải', icon: '▶', repeat: true },
		{ a: Act.SoftDrop, label: 'Rơi nhanh', icon: '▼', repeat: true },
		{ a: Act.HardDrop, label: 'Hạ khối', icon: '⤓', repeat: false }
	] as const;
</script>

<svelte:window onkeydown={onkey} />

<div class="tamma grid h-full min-h-0 grid-cols-[minmax(0,1fr)] gap-3 lg:grid-cols-[minmax(0,1fr)_240px]" bind:this={host}>
	<div class="flex min-h-0 flex-col gap-2">
		<!-- HUD -->
		<div class="hud flex min-w-0 items-center gap-2.5 px-2.5 py-1.5 text-sm sm:gap-3 sm:px-3" aria-live="off">
			<div class="shrink-0">
				<p class="eyebrow whitespace-nowrap">Đã trừ</p>
				<p class="text-xl leading-tight font-semibold text-jade-300 tabular-nums">
					{lines}<span class="text-sm text-mist-500">/{params.passLines}</span>
				</p>
			</div>
			<div class="shrink-0">
				<p class="eyebrow whitespace-nowrap">Còn lại</p>
				<p class="text-xl leading-tight font-semibold tabular-nums" class:text-cinnabar-300={ticksLeft * params.tickMs < 30000} class:text-mist-100={ticksLeft * params.tickMs >= 30000}>
					{timeText}
				</p>
			</div>
			<button
				type="button"
				class="calm ml-auto flex min-w-0 items-center gap-1.5 px-2 py-1"
				class:is-ready={calmReady}
				class:is-active={calmLeft > 0}
				disabled={!calmReady || paused || ended}
				onclick={() => push(Act.Calm)}
				aria-label="Tĩnh tâm: đóng băng tạp niệm"
			>
				<span class="meter" role="progressbar" aria-valuemin={0} aria-valuemax={100} aria-valuenow={Math.round(calmPct)} aria-label="Tĩnh tâm">
					<span style:width="{calmPct}%"></span>
				</span>
				<span class="text-xs font-semibold whitespace-nowrap">{calmLeft > 0 ? 'Đang tĩnh' : 'Tĩnh tâm'}</span>
			</button>
			<div class="next lg:hidden" aria-label="Tạp niệm kế tiếp">
				<canvas bind:this={nextA} style:width="56px" style:height="56px"></canvas>
			</div>
		</div>

		<!-- Board -->
		<div class="relative min-h-0 flex-1" bind:clientWidth={boxW} bind:clientHeight={boxH}>
			<div class="absolute inset-0 grid place-items-center">
				<div class="frame relative" style:width="{params.width * cell}px" style:height="{params.height * cell}px">
					<canvas
						bind:this={board}
						style:width="{params.width * cell}px"
						style:height="{params.height * cell}px"
						aria-label="Thức hải: bàn tâm ma"
					></canvas>
					{#if holdText && !paused && !ended}
						<p class="hold" aria-live="polite">Đủ tạp niệm — {holdText}</p>
					{/if}
					{#if countdown > 0 && !paused}
						<div class="veil grid place-items-center"><p class="font-display text-6xl font-bold text-gold-300">{countdown}</p></div>
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

		<!-- Touch controls -->
		<div class="pad grid grid-cols-5 gap-1.5 lg:hidden">
			{#each controls as c (c.a)}
				<button
					type="button"
					class="padbtn"
					aria-label={c.label}
					onpointerdown={press(c.a, c.repeat)}
					onpointerup={release}
					onpointerleave={release}
					onpointercancel={release}
					oncontextmenu={(e) => e.preventDefault()}
				>
					<span aria-hidden="true">{c.icon}</span>
				</button>
			{/each}
		</div>
	</div>

	<!-- Desktop side panel -->
	<aside class="hidden space-y-3 lg:block">
		<div class="side p-3">
			<p class="eyebrow">Kế tiếp</p>
			<div class="mt-2 grid place-items-center"><canvas bind:this={nextB} style:width="56px" style:height="56px"></canvas></div>
		</div>
		<div class="side space-y-1 p-3 text-sm text-mist-300">
			<p class="eyebrow mb-1">Phím</p>
			<p><kbd>←</kbd> <kbd>→</kbd> di chuyển</p>
			<p><kbd>↑</kbd>/<kbd>X</kbd> xoay · <kbd>Z</kbd> xoay ngược</p>
			<p><kbd>↓</kbd> rơi nhanh · <kbd>Space</kbd> hạ khối</p>
			<p><kbd>C</kbd> tĩnh tâm · <kbd>P</kbd> tạm dừng</p>
		</div>
		<div class="flex gap-2">
			<button type="button" class="btn btn-ghost flex-1" disabled={ended} onclick={() => (paused = !paused)}>
				{paused ? 'Tiếp tục' : 'Tạm dừng'}
			</button>
		</div>
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
	}
	.veil {
		position: absolute;
		inset: 0;
		background: color-mix(in oklab, var(--color-ink-950) 72%, transparent);
	}
	.calm {
		border: 1px solid color-mix(in oklab, var(--color-spirit-cyan) 30%, transparent);
		color: var(--color-mist-300);
		min-height: 40px;
		touch-action: manipulation;
	}
	.calm.is-ready {
		color: var(--color-spirit-cyan);
		border-color: var(--color-spirit-cyan);
		animation: calm-glow 1.8s ease-in-out infinite;
	}
	.calm.is-active {
		color: var(--color-spirit-cyan);
	}
	.calm:disabled:not(.is-active) {
		opacity: 0.75;
	}
	@keyframes calm-glow {
		50% {
			box-shadow: 0 0 14px -2px color-mix(in oklab, var(--color-spirit-cyan) 60%, transparent);
		}
	}
	.meter {
		display: block;
		flex-shrink: 0;
		width: 36px;
		height: 6px;
		border-radius: 3px;
		background: var(--color-ink-600);
		overflow: hidden;
	}
	.meter > span {
		display: block;
		height: 100%;
		background: var(--color-spirit-cyan);
		transition: width 150ms var(--ease-qi, ease);
	}
	.next canvas {
		display: block;
	}
	.padbtn {
		height: 52px;
		border: 1px solid color-mix(in oklab, var(--color-gold-500) 30%, transparent);
		background: color-mix(in oklab, var(--color-ink-800) 70%, transparent);
		color: var(--color-gold-300);
		font-size: 1.35rem;
		touch-action: none;
		user-select: none;
		-webkit-user-select: none;
	}
	.padbtn:active {
		background: color-mix(in oklab, var(--color-jade-700) 60%, transparent);
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
