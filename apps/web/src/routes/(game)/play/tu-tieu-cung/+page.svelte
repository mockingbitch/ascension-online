<!--
	Tử Tiêu Cung: Hồng Quân lectures all day. Online players sit on the hall's cushions (server-assigned seats,
	GET /lecture/state + WS lecture.enter/leave), hall chat shows as speech bubbles, and each ngộ đạo moment pays the
	listening time the SERVER measured. Script playback, countdowns, the progress ring and the listening clock are
	display only (server clock offset). The ngộ đạo control has exactly one state: listening | ready | cooldown.
-->
<script lang="ts">
	import { onDestroy, onMount } from 'svelte';
	import { MediaQuery } from 'svelte/reactivity';
	import { fade, fly } from 'svelte/transition';
	import { ApiError } from '$lib/api/client';
	import ChatPanel from '$lib/components/ChatPanel.svelte';
	import Icon from '$lib/components/Icon.svelte';
	import ScreenFrame from '$lib/components/ScreenFrame.svelte';
	import { art } from '$lib/game/art';
	import { formatPoints } from '$lib/game/format';
	import { hall } from '$lib/game/lecture.svelte';
	import { screens } from '$lib/game/nav';
	import { game } from '$lib/game/store.svelte';
	import { toast } from '$lib/game/toasts.svelte';
	import type { InsightResult } from '$lib/game/types';

	const bg = art('scenes/tu-tieu-cung');
	const sage = art('characters/hong-quan');
	const sitter = art('characters/listener');
	const wide = new MediaQuery('min-width: 1024px'); // the shell docks the chat beside the screen
	const phone = new MediaQuery('max-width: 639px');

	// Hồng Quân's dais in the art (percent of the 16:9 painting): particles start here.
	const DAIS = { x: 50, y: 24 };
	// 4 rows × 9 cushions in the hall's perspective (percent of the 16:9 painting). Row 0 is nearest the dais.
	// Rows stay above ~75% so the subtitle band never covers a listener.
	const ROWS = [
		{ y: 43, from: 35, to: 65, scale: 0.62 },
		{ y: 52, from: 33, to: 67, scale: 0.72 },
		{ y: 62, from: 31, to: 69, scale: 0.83 },
		{ y: 72, from: 29, to: 71, scale: 0.94 }
	];
	const seatCount = $derived(hall.state?.seats ?? 36);
	const perRow = $derived(Math.ceil(seatCount / ROWS.length));
	// fill from the centre outwards so a half-empty hall still looks gathered around the aisle
	const centreOut = $derived(
		[...Array(perRow).keys()].sort((a, b) => Math.abs(a - (perRow - 1) / 2) - Math.abs(b - (perRow - 1) / 2) || a - b)
	);
	const layout = $derived(
		Array.from({ length: seatCount }, (_, i) => {
			const r = ROWS[Math.min(ROWS.length - 1, Math.floor(i / perRow))]!;
			const x = r.from + ((r.to - r.from) * (centreOut[i % perRow]! + 0.5)) / perRow;
			return { seat: i, x, y: r.y, scale: r.scale };
		})
	);

	const cat = $derived(game.catalogue);
	const me = $derived(game.character);
	const speaking = $derived(hall.speaking);
	const ins = $derived(hall.insight);
	const phase = $derived(ins?.phase ?? 'listening');
	const mySeat = $derived(hall.state?.me.inHall && hall.state.me.seat >= 0 ? layout[hall.state.me.seat] : undefined);
	const outside = $derived(Math.max(0, hall.listeners - Object.keys(hall.seats).length));
	const today = $derived(hall.state?.today);
	const capped = $derived(!!today && today.milli >= today.capMilli);

	let reveal = $state<InsightResult | null>(null);
	let notebookOpen = $state(false);

	const mmss = (ms: number) => {
		const s = Math.max(0, Math.ceil(ms / 1000));
		return `${String(Math.floor(s / 60)).padStart(2, '0')}:${String(s % 60).padStart(2, '0')}`;
	};
	// Ring geometry (SVG units)
	const R = 46;
	const C = 2 * Math.PI * R;

	onMount(() => void hall.open());
	onDestroy(() => hall.close());

	async function ngoDao() {
		try {
			const r = await hall.ngoDao();
			if (!r) return;
			reveal = r;
			if (r.earnedMilli > 0) toast('ok', `Ngộ đạo: +${formatPoints(r.earnedMilli)} tu vi${r.vipPoints ? `, +${r.vipPoints} điểm duyên` : ''}.`);
			void game.reload();
		} catch (e) {
			toast(e instanceof ApiError && e.status < 500 ? 'warn' : 'err', e instanceof ApiError ? e.message : 'Mất kết nối, thử lại.');
		}
	}

	async function openNotebook() {
		notebookOpen = true;
		if (!hall.notebook) await hall.loadNotebook().catch(() => toast('err', 'Không tải được Sổ Đạo Ngộ.'));
	}
</script>

<svelte:head><title>Tử Tiêu Cung · Tiên Đạo</title></svelte:head>

{#snippet subtitle()}
	{#if phase === 'ready' && speaking}
		{#key 'q' + ins?.slot}
			<div in:fade={{ duration: 450 }}>
				<p class="sub-label !text-gold-300">Hồng Quân hỏi · {speaking.script.title}</p>
				<p class="sub-text">{speaking.script.question}</p>
			</div>
		{/key}
	{:else if speaking}
		{#key speaking.script.id + ':' + speaking.line}
			<div in:fade={{ duration: 450 }}>
				<p class="sub-label">Lời giảng {speaking.line + 1}/{speaking.script.lines.length} · {speaking.script.title}</p>
				<p class="sub-text">{speaking.script.lines[speaking.line]!.text}</p>
			</div>
		{/key}
	{:else if hall.error}
		<p class="sub-text !text-cinnabar-300">{hall.error}</p>
	{:else}
		<p class="sub-label">Hồng Quân đang nhập định…</p>
	{/if}
{/snippet}

<ScreenFrame title="Tử Tiêu Cung" subtitle="Hồng Quân giảng đạo — ngồi nghe, ngộ đạo, cùng đạo hữu luận đạo" icon="scroll" bleed>
	{#snippet actions()}
		<button type="button" class="btn btn-ghost !px-3 !py-1.5 text-sm" onclick={openNotebook} aria-label="Sổ Đạo Ngộ">
			<Icon name="scroll" size={16} /> <span class="hidden sm:inline">Sổ Đạo Ngộ</span>
		</button>
		<a href={screens.timeline.href} class="btn btn-ghost hidden !px-3 !py-1.5 text-sm md:inline-flex"><Icon name="clock" size={16} /> Thiên Đạo Sử</a>
	{/snippet}

	<div class="flex h-full min-h-0 flex-col">
		<!-- scene: full width; the painting covers the box anchored centre-top, seats ride on the painting -->
		<div class="stage">
			<div class="art">
				{#if bg}<img class="bg" src={bg.src} srcset={bg.srcset} alt="" draggable="false" />{/if}

				<div class="sage" aria-hidden="true">
					<span class="aura"></span>
					{#if sage}
						<img src={sage.src} srcset={sage.srcset} alt="" draggable="false" />
					{:else}
						<span class="sage-fallback"><Icon name="spark" size={48} /></span>
					{/if}
				</div>

				<!-- linh khí drifting from Hồng Quân to the player while listening (hidden under reduced motion) -->
				{#if phase === 'listening' && mySeat}
					<div class="qi-stream" aria-hidden="true" style:--dx="{mySeat.x - DAIS.x}cqw" style:--dy="{(mySeat.y - DAIS.y) * 0.5625}cqw">
						{#each Array(9) as _, i (i)}
							<span class="mote" style:--delay="{(i * 0.47) % 3.2}s" style:--wobble="{((i * 37) % 5) - 2}cqw"></span>
						{/each}
					</div>
				{/if}

				<ul class="seats" aria-label="Đạo hữu đang nghe giảng">
					{#each layout as s (s.seat)}
						{@const l = hall.seats[s.seat]}
						{@const mine = !!l && l.characterId === me?.id}
						{@const b = l ? hall.bubbles[l.characterId] : undefined}
						<li
							class="seat"
							class:is-mine={mine}
							class:is-empty={!l}
							style:left="{s.x}%"
							style:top="{s.y}%"
							style:--s={mine ? s.scale * 1.3 : s.scale}
							style:z-index={Math.round(s.y) + (mine ? 50 : 0)}
						>
							{#if mine}<span class="halo" aria-hidden="true"></span>{/if}
							<span class="cushion" aria-hidden="true"></span>
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
										<span class="bubble" in:fly={{ y: 6, duration: 250 }} out:fade={{ duration: 300 }}>
											<strong class="text-gold-300">{l.name}:</strong>
											{b.text}
										</span>
									{/key}
								{/if}
							{/if}
						</li>
					{/each}
				</ul>
			</div>

			{#if outside > 0}<p class="outside">+{outside} đạo hữu nghe ngoài điện</p>{/if}
			{#if !phone.current}<div class="subtitles" aria-live="polite">{@render subtitle()}</div>{/if}
		</div>

		{#if phone.current}<div class="subtitles is-below" aria-live="polite">{@render subtitle()}</div>{/if}

		<!-- ngộ đạo: the one call to action on this screen (always visible above the chat on phones) -->
		<section class="cta" data-phase={phase} aria-label="Ngộ đạo">
			<div class="min-w-0 flex-1">
				<p class="status">
					{#if phase === 'ready'}
						<Icon name="spark" size={16} class="text-gold-300" /> Thời khắc ngộ đạo — còn
						<span class="tabular-nums">{mmss(ins?.msToClose ?? 0)}</span>
					{:else if phase === 'cooldown'}
						<Icon name="check" size={16} class="text-jade-300" /> Đã ngộ đạo — lần tới sau
						<span class="tabular-nums">{mmss(ins?.msToNext ?? 0)}</span>
					{:else}
						<Icon name="lotus" size={16} class="text-jade-300" />
						{hall.full ? 'Linh khí đã đầy' : 'Đang tích linh khí'}
						<span class="text-mist-300 tabular-nums">· {Math.round((ins?.progress ?? 0) * 100)}%</span>
					{/if}
				</p>
				<dl class="facts">
					<div>
						<dt>Đã nghe</dt>
						<dd class="tabular-nums">
							{mmss(hall.accruedSeconds * 1000)}{#if hall.full}<span class="text-mist-300">&nbsp;(tối đa)</span>{/if}
						</dd>
					</div>
					<div>
						<dt>Tu vi hôm nay</dt>
						<dd class="tabular-nums">
							{formatPoints(today?.milli ?? 0)}/{formatPoints(today?.capMilli ?? 0)}{#if capped}<span class="text-mist-300">&nbsp;· đã đủ</span>{/if}
						</dd>
					</div>
					<div>
						<dt>Tốc độ</dt>
						<dd class="tabular-nums">+{formatPoints(hall.state?.rateMilliPerHour ?? 0)} tu vi/giờ</dd>
					</div>
					<div class="hidden sm:block">
						<dt>Đang nghe</dt>
						<dd class="tabular-nums">{hall.listeners} đạo hữu</dd>
					</div>
				</dl>
			</div>

			<div class="orb-wrap">
				<svg class="progress-ring" viewBox="0 0 100 100" aria-hidden="true">
					<circle class="prog-track" cx="50" cy="50" r={R} />
					<circle class="prog-fill" cx="50" cy="50" r={R} stroke-dasharray={C} stroke-dashoffset={C * (1 - (ins?.progress ?? 0))} />
				</svg>
				<button type="button" class="orb" disabled={phase !== 'ready' || hall.busy} onclick={ngoDao} aria-describedby="orb-hint">
					<span class="orb-title">Ngộ Đạo</span>
					<span id="orb-hint" class="orb-sub tabular-nums">
						{#if phase === 'ready'}{capped ? 'ghi vào sổ' : 'nhận tu vi'}{:else}{mmss(ins?.msToNext ?? 0)}{/if}
					</span>
				</button>
			</div>
		</section>

		{#if !wide.current}
			<div class="chat-dock min-h-0 flex-1"><ChatPanel compact /></div>
		{/if}
	</div>
</ScreenFrame>

<!-- ngộ đạo reveal -->
{#if reveal}
	<div class="overlay" transition:fade={{ duration: 200 }} role="presentation" onclick={() => (reveal = null)}>
		<div
			class="panel max-w-md p-6 text-center"
			role="dialog"
			aria-modal="true"
			aria-label="Ngộ đạo"
			tabindex="-1"
			onclick={(e) => e.stopPropagation()}
			onkeydown={(e) => e.key === 'Escape' && (reveal = null)}
		>
			<p class="text-sm font-semibold text-gold-500">Ngộ đạo</p>
			{#if reveal.takeaway}<p class="mt-3 font-display text-2xl leading-snug text-gold-300">“{reveal.takeaway}”</p>{/if}
			<p class="mt-4 text-sm text-mist-300">
				Nghe giảng {mmss(reveal.presenceSeconds * 1000)} ·
				{#if reveal.earnedMilli > 0}<strong class="text-jade-300">+{formatPoints(reveal.earnedMilli)} tu vi</strong>{:else}tu vi hôm nay từ giảng đạo đã đủ{/if}
				{#if reveal.vipPoints > 0}· <strong class="text-gold-300">+{reveal.vipPoints} điểm duyên</strong>{/if}
			</p>
			{#if reveal.rawMilli > reveal.earnedMilli}
				<p class="mt-1 text-xs text-mist-300">Một phần không được cộng vì đã chạm hạn mức hoạt động hôm nay hoặc bình cảnh.</p>
			{/if}
			<button type="button" class="btn btn-jade mt-5" onclick={() => (reveal = null)}>Ghi nhớ lời dạy</button>
		</div>
	</div>
{/if}

<!-- Sổ Đạo Ngộ -->
{#if notebookOpen}
	<div class="overlay" transition:fade={{ duration: 200 }} role="presentation" onclick={() => (notebookOpen = false)}>
		<div
			class="panel w-full max-w-lg p-5"
			role="dialog"
			aria-modal="true"
			aria-label="Sổ Đạo Ngộ"
			tabindex="-1"
			onclick={(e) => e.stopPropagation()}
			onkeydown={(e) => e.key === 'Escape' && (notebookOpen = false)}
		>
			<div class="flex items-center justify-between">
				<h2 class="font-display text-2xl font-bold text-gold-300">Sổ Đạo Ngộ</h2>
				<button type="button" class="btn btn-ghost !px-2.5" onclick={() => (notebookOpen = false)} aria-label="Đóng"><Icon name="cross" size={16} /></button>
			</div>
			<p class="mt-1 text-xs text-mist-300">Những bài giảng bạn đã ngộ, theo thứ tự lần đầu.</p>
			<ol class="mt-4 max-h-[60vh] space-y-3 overflow-y-auto">
				{#if hall.notebook === null}
					<li class="text-mist-300">Đang mở sổ…</li>
				{:else}
					{#each hall.notebook as n (n.lectureId)}
						<li class="note p-3">
							<p class="font-display text-lg text-mist-100">{n.title} <span class="text-xs text-mist-300">· {n.theme} · {n.times} lần</span></p>
							<p class="mt-1 text-sm text-gold-300">“{n.takeaway}”</p>
						</li>
					{:else}
						<li class="text-mist-300">Chưa có lời dạy nào. Ngồi nghe giảng và ngộ đạo khi Hồng Quân đặt câu hỏi.</li>
					{/each}
				{/if}
			</ol>
		</div>
	</div>
{/if}

<style>
	/* ── scene ── */
	.stage {
		position: relative;
		flex: 1 1 0;
		min-height: 12rem;
		overflow: hidden;
		container-type: size;
		background: radial-gradient(ellipse at 50% 20%, color-mix(in oklab, var(--color-spirit-violet) 30%, var(--color-ink-900)), var(--color-ink-950) 70%);
	}
	/* The 16:9 painting covers the box (= object-fit: cover; object-position: center top) and carries the seats. */
	.art {
		position: absolute;
		left: 50%;
		top: 0;
		width: max(100cqw, 177.78cqh);
		aspect-ratio: 16 / 9;
		transform: translateX(-50%);
		container-type: inline-size;
	}
	.bg {
		position: absolute;
		inset: 0;
		width: 100%;
		height: 100%;
		object-fit: cover;
		object-position: center top;
	}
	.sage {
		position: absolute;
		left: 50%;
		bottom: 73%;
		width: 13cqw;
		transform: translateX(-50%);
		z-index: 30;
	}
	.sage img {
		position: relative;
		width: 100%;
		filter: drop-shadow(0 0 1.2cqw color-mix(in oklab, var(--color-spirit-violet) 60%, transparent));
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
		animation: breathe 6s var(--ease-qi) infinite;
	}
	.sage-fallback {
		display: grid;
		place-items: center;
		aspect-ratio: 1;
		color: var(--color-gold-300);
	}

	/* linh khí motes: start on the dais, drift to the player's cushion */
	.qi-stream {
		position: absolute;
		left: 50%;
		top: 24%;
		z-index: 25;
	}
	.mote {
		position: absolute;
		left: -0.25cqw;
		top: -0.25cqw;
		width: 0.5cqw;
		height: 0.5cqw;
		border-radius: 50%;
		background: var(--color-spirit-cyan);
		box-shadow: 0 0 0.8cqw var(--color-spirit-cyan);
		opacity: 0;
		animation: drift 3.2s linear var(--delay) infinite;
	}
	@keyframes drift {
		0% {
			opacity: 0;
			transform: translate(0, 0) scale(0.6);
		}
		15% {
			opacity: 0.9;
		}
		50% {
			transform: translate(calc(var(--dx) * 0.5 + var(--wobble)), calc(var(--dy) * 0.45)) scale(1);
		}
		85% {
			opacity: 0.7;
		}
		100% {
			opacity: 0;
			transform: translate(var(--dx), var(--dy)) scale(0.4);
		}
	}
	@media (prefers-reduced-motion: reduce) {
		.qi-stream {
			display: none;
		}
	}

	/* seats */
	.seats {
		position: absolute;
		inset: 0;
	}
	.seat {
		position: absolute;
		width: calc(6.2cqw * var(--s));
		aspect-ratio: 1;
		transform: translate(-50%, -88%);
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
		font-size: clamp(0.65rem, 1cqw, 0.85rem);
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
		transition: opacity 150ms;
		z-index: 120;
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
		max-width: min(16rem, 26cqw);
		margin-bottom: 0.25rem;
		padding: 0.3rem 0.55rem;
		font-size: clamp(0.68rem, 0.95cqw, 0.85rem);
		line-height: 1.3;
		color: var(--color-mist-100);
		border: 1px solid color-mix(in oklab, var(--color-gold-500) 45%, transparent);
		border-radius: 10px;
		background: color-mix(in oklab, var(--color-ink-950) 90%, transparent);
		transform: translateX(-50%);
		z-index: 110;
	}
	.outside {
		position: absolute;
		left: 0.5rem;
		top: 0.5rem;
		padding: 0.1rem 0.6rem;
		font-size: 0.75rem;
		color: var(--color-mist-300);
		background: color-mix(in oklab, var(--color-ink-950) 80%, transparent);
	}

	/* ── subtitles: bottom band of the scene (below the scene on phones) ── */
	.subtitles {
		position: absolute;
		inset-inline: 0;
		bottom: 0;
		z-index: 200;
		min-height: 5.5rem;
		padding: 2.5rem 1.25rem 0.85rem;
		background: linear-gradient(0deg, color-mix(in oklab, var(--color-ink-950) 94%, transparent) 35%, transparent);
	}
	.subtitles.is-below {
		position: static;
		min-height: 4.75rem;
		padding: 0.6rem 0.9rem;
		background: var(--color-ink-950);
		border-bottom: 1px solid color-mix(in oklab, var(--color-gold-500) 15%, transparent);
	}
	.sub-label {
		font-size: 0.78rem;
		font-weight: 600;
		color: var(--color-mist-300);
	}
	.sub-text {
		margin-top: 0.15rem;
		max-width: 60rem;
		font-family: var(--font-display);
		font-size: clamp(1rem, 1.9cqw, 1.4rem);
		line-height: 1.35;
		color: var(--color-mist-100);
		text-shadow: 0 1px 3px var(--color-ink-950);
	}
	.subtitles.is-below .sub-text {
		font-size: 1rem;
	}

	/* ── ngộ đạo CTA ── */
	.cta {
		display: flex;
		flex-shrink: 0;
		align-items: center;
		gap: 1rem;
		padding: 0.75rem 1rem;
		border-top: 1px solid color-mix(in oklab, var(--color-gold-500) 22%, transparent);
		background: linear-gradient(180deg, var(--color-ink-900), var(--color-ink-950));
	}
	.cta[data-phase='ready'] {
		background: linear-gradient(180deg, color-mix(in oklab, var(--color-gold-500) 14%, var(--color-ink-900)), var(--color-ink-950));
	}
	.status {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: 0.35rem;
		font-size: 0.95rem;
		font-weight: 600;
		color: var(--color-mist-100);
	}
	.facts {
		display: flex;
		flex-wrap: wrap;
		gap: 0.2rem 1.25rem;
		margin-top: 0.35rem;
	}
	.facts dt {
		font-size: 0.72rem;
		color: var(--color-mist-300);
	}
	.facts dd {
		font-size: 0.9rem;
		font-weight: 600;
		color: var(--color-mist-100);
	}
	.orb-wrap {
		position: relative;
		width: 6rem;
		height: 6rem;
		flex-shrink: 0;
	}
	.progress-ring {
		position: absolute;
		inset: 0;
		transform: rotate(-90deg);
	}
	.progress-ring circle {
		fill: none;
		stroke-width: 5;
	}
	.prog-track {
		stroke: var(--color-ink-700);
	}
	.prog-fill {
		stroke: var(--color-jade-500);
		stroke-linecap: round;
		transition: stroke-dashoffset 1s linear;
	}
	.cta[data-phase='ready'] .prog-fill {
		stroke: var(--color-gold-300);
	}
	.cta[data-phase='cooldown'] .prog-fill {
		stroke: var(--color-ink-600);
	}
	.orb {
		position: absolute;
		inset: 9px;
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		border-radius: 50%;
		color: var(--color-mist-300);
		border: 1px solid var(--color-ink-600);
		background: radial-gradient(circle at 50% 35%, var(--color-ink-700), var(--color-ink-900));
		transition:
			transform 200ms var(--ease-qi),
			color 200ms;
	}
	.orb-title {
		font-family: var(--font-display);
		font-size: 1.15rem;
		font-weight: 700;
		line-height: 1.1;
	}
	.orb-sub {
		font-size: 0.72rem;
		font-weight: 600;
	}
	.cta[data-phase='ready'] .orb {
		color: var(--color-ink-950);
		border-color: var(--color-gold-300);
		background: radial-gradient(circle at 50% 30%, var(--color-gold-300), var(--color-gold-500) 70%, var(--color-gold-700));
		box-shadow:
			0 0 18px color-mix(in oklab, var(--color-gold-300) 70%, transparent),
			0 0 42px color-mix(in oklab, var(--color-jade-300) 35%, transparent);
		animation: pulse 2.2s var(--ease-qi) infinite;
		cursor: pointer;
	}
	.cta[data-phase='ready'] .orb:hover {
		transform: scale(1.04);
	}
	.cta[data-phase='cooldown'] .orb {
		opacity: 0.7;
	}
	@keyframes pulse {
		0%,
		100% {
			transform: scale(1);
		}
		50% {
			transform: scale(1.05);
		}
	}
	@keyframes breathe {
		0%,
		100% {
			opacity: 0.7;
			transform: scale(1);
		}
		50% {
			opacity: 1;
			transform: scale(1.04);
		}
	}

	.chat-dock {
		min-height: 9rem;
		border-top: 1px solid color-mix(in oklab, var(--color-gold-500) 22%, transparent);
	}

	/* ── phones: scene keeps 16:9 at full width, subtitles under it, CTA then chat fill the rest ── */
	@media (max-width: 639px) {
		.stage {
			flex: none;
			min-height: 0;
			aspect-ratio: 16 / 9;
		}
		.sage {
			width: 15cqw;
		}
		.cta {
			gap: 0.75rem;
			padding: 0.6rem 0.75rem;
		}
		.orb-wrap {
			width: 5rem;
			height: 5rem;
		}
		.orb-title {
			font-size: 1rem;
		}
		.facts {
			gap: 0.1rem 0.9rem;
		}
		.status {
			font-size: 0.85rem;
		}
	}
	/* tablets: give the scene room, the chat a fixed band */
	@media (min-width: 640px) and (max-width: 1023px) {
		.chat-dock {
			flex: none;
			height: 15rem;
		}
	}

	.overlay {
		position: fixed;
		inset: 0;
		z-index: 80;
		display: grid;
		place-items: center;
		padding: 1rem;
		background: color-mix(in oklab, var(--color-ink-950) 70%, transparent);
	}
	.note {
		border: 1px solid color-mix(in oklab, var(--color-gold-500) 20%, transparent);
		background: color-mix(in oklab, var(--color-ink-900) 80%, transparent);
	}
</style>
