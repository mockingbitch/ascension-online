<!--
	Tử Tiêu Cung: Hồng Quân lectures all day. The hall, Hồng Quân, the cushions, bubbles and the qi stream are the
	tu_tieu_cung SCENE (SceneLayer + HallView, positions from the def's anchors); this page is the UI over it. Online players sit on the hall's cushions (server-assigned seats,
	GET /lecture/state + WS lecture.enter/leave), hall chat shows as speech bubbles, and each ngộ đạo moment pays the
	listening time the SERVER measured. Script playback, countdowns, the progress ring and the listening clock are
	display only (server clock offset). The ngộ đạo control has exactly one state: listening | ready | cooldown; it is
	the action bar's primary button (registered here with its progress ring). Chat is the shell's floating chat
	(Luận Đạo tab while here); rules live behind "?".
-->
<script lang="ts">
	import { onDestroy, onMount } from 'svelte';
	import { MediaQuery } from 'svelte/reactivity';
	import { fade } from 'svelte/transition';
	import { settings } from '$lib/engine/settings.svelte';
	import { ApiError } from '$lib/api/client';
	import EmptyState from '$lib/components/EmptyState.svelte';
	import GameWindow from '$lib/components/GameWindow.svelte';
	import HelpButton from '$lib/components/HelpButton.svelte';
	import Icon from '$lib/components/Icon.svelte';
	import SceneSlot from '$lib/engine/scene/SceneSlot.svelte';
	import { formatPoints } from '$lib/game/format';
	import { hall } from '$lib/game/lecture.svelte';
	import { screens } from '$lib/game/nav';
	import { usePrimaryAction } from '$lib/game/primary.svelte';
	import { game } from '$lib/game/store.svelte';
	import { toast } from '$lib/game/toasts.svelte';
	import type { InsightResult } from '$lib/game/types';

	const phone = new MediaQuery('max-width: 639px');

	const speaking = $derived(hall.speaking);
	const ins = $derived(hall.insight);
	const phase = $derived(ins?.phase ?? 'listening');
	const outside = $derived(Math.max(0, hall.listeners - Object.keys(hall.seats).length));
	const today = $derived(hall.state?.today);
	const capped = $derived(!!today && today.milli >= today.capMilli);

	let reveal = $state<InsightResult | null>(null);
	let notebookOpen = $state(false);

	// subtitles fade in line by line (lite: quicker, off: at once)
	const subFade = $derived(settings.fx === 'full' ? 650 : settings.fx === 'lite' ? 250 : 0);

	const mmss = (ms: number) => {
		const s = Math.max(0, Math.ceil(ms / 1000));
		return `${String(Math.floor(s / 60)).padStart(2, '0')}:${String(s % 60).padStart(2, '0')}`;
	};

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

	usePrimaryAction(() => ({
		id: 'ngo_dao',
		label: 'Ngộ đạo',
		icon: 'spark',
		hint: phase === 'ready' ? `còn ${mmss(ins?.msToClose ?? 0)}` : phase === 'cooldown' ? `sau ${mmss(ins?.msToNext ?? 0)}` : `${Math.round((ins?.progress ?? 0) * 100)}%`,
		ready: phase === 'ready',
		disabled: phase !== 'ready' || hall.busy,
		busy: hall.busy,
		progress: ins?.progress ?? 0,
		tone: 'gold',
		run: () => void ngoDao()
	}));

	async function openNotebook() {
		notebookOpen = true;
		if (!hall.notebook) await hall.loadNotebook().catch(() => toast('err', 'Không tải được Sổ Đạo Ngộ.'));
	}
</script>

<svelte:head><title>Tử Tiêu Cung · Tiên Đạo</title></svelte:head>

{#snippet subtitle()}
	{#if phase === 'ready' && speaking}
		{#key 'q' + ins?.slot}
			<div in:fade={{ duration: subFade }}>
				<p class="sub-label !text-gold-300">Hồng Quân hỏi · {speaking.script.title}</p>
				<p class="sub-text">{speaking.script.question}</p>
			</div>
		{/key}
	{:else if speaking}
		{#key speaking.script.id + ':' + speaking.line}
			<div in:fade={{ duration: subFade }}>
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

	<div class="hall-page flex h-full min-h-0 flex-col overflow-y-auto overscroll-contain">
		<!-- ngộ đạo status (the button itself is the action bar's primary) -->
		<section class="cta ui" data-phase={phase} aria-label="Ngộ đạo">
			<div class="min-w-0 flex-1">
				<p class="eyebrow hidden sm:block">Tử Tiêu Cung · Hồng Quân giảng đạo</p>
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

			<HelpButton topic="hall" />
		</section>

		<!-- scene slot: the hall painting covers it anchored centre-top (renderer); UI rides on top -->
		<SceneSlot class="stage" passthrough>
			<div class="bar ui">
				<div class="ml-auto flex gap-2">
					<button type="button" class="btn btn-ghost !px-3 !py-1.5 text-sm" onclick={openNotebook} aria-label="Sổ Đạo Ngộ">
						<Icon name="scroll" size={16} /> <span class="hidden sm:inline">Sổ Đạo Ngộ</span>
					</button>
					<a href={screens.timeline.href} class="btn btn-ghost hidden !px-3 !py-1.5 text-sm md:inline-flex"><Icon name="clock" size={16} /> Thiên Đạo Sử</a>
				</div>
			</div>
			{#if outside > 0}<p class="outside">+{outside} đạo hữu nghe ngoài điện</p>{/if}
			{#if !phone.current}<div class="subtitles" aria-live="polite">{@render subtitle()}</div>{/if}
		</SceneSlot>

		{#if phone.current}<div class="subtitles is-below ui" aria-live="polite">{@render subtitle()}</div>{/if}

	</div>

<!-- the ngộ đạo moment opened: a gold pulse runs around the screen edge (decorative; the status band says it) -->
{#if phase === 'ready'}
	{#key ins?.slot}<div class="edge-pulse" aria-hidden="true" data-edge-pulse></div>{/key}
{/if}

<!-- ngộ đạo reveal -->
<GameWindow local open={!!reveal} title="Ngộ đạo" icon="spark" onclose={() => (reveal = null)}>
	{#if reveal}
		<div class="text-center">
			{#if reveal.takeaway}
				<div class="teaching" data-teaching>
					<span class="rod" aria-hidden="true"></span>
					<p class="font-display text-2xl leading-snug">“{reveal.takeaway}”</p>
					<span class="rod" aria-hidden="true"></span>
				</div>
			{/if}
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
	{/if}
</GameWindow>

<!-- Sổ Đạo Ngộ -->
<GameWindow local open={notebookOpen} title="Sổ Đạo Ngộ" icon="scroll" onclose={() => (notebookOpen = false)}>
	<ol class="stagger space-y-3">
		{#if hall.notebook === null}
			<li class="text-mist-300">Đang mở sổ…</li>
		{:else}
			{#each hall.notebook as n (n.lectureId)}
				<li class="note p-3">
					<p class="font-display text-lg text-mist-100">{n.title} <span class="text-xs text-mist-300">· {n.theme} · {n.times} lần</span></p>
					<p class="mt-1 text-sm text-gold-300">“{n.takeaway}”</p>
				</li>
			{:else}
				<li><EmptyState icon="scroll" title="Sổ còn trống" hint="Ngồi nghe giảng và ngộ đạo khi Hồng Quân đặt câu hỏi." /></li>
			{/each}
		{/if}
	</ol>
</GameWindow>

<style>
	/* ── scene slot ── */
	.hall-page :global(.stage) {
		flex: 1 1 0;
		min-height: 12rem;
		container-type: size;
	}
	.bar {
		position: absolute;
		right: 0;
		top: 0;
		z-index: var(--z-local-1);
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 0.5rem;
		padding: 0.5rem 0.75rem;
		flex-wrap: wrap;
	}
	.outside {
		position: absolute;
		left: 0.5rem;
		top: 3.25rem;
		padding: 0.1rem 0.6rem;
		font-size: 0.75rem;
		color: var(--color-mist-300);
		background: color-mix(in oklab, var(--color-ink-950) 80%, transparent);
	}

	/* ── subtitles: bottom band of the scene (below the scene on phones) ── */
	.subtitles {
		position: absolute;
		/* the chat dock owns the stage's bottom-left corner (--chat-w): the band starts right of it */
		left: calc(var(--chat-w, 0px) + 0.5rem);
		right: 0;
		bottom: 0;
		z-index: var(--z-local-2);
		min-height: 5.5rem;
		padding: 2.5rem 1.25rem 0.85rem 0.75rem;
		background: linear-gradient(0deg, color-mix(in oklab, var(--color-ink-950) 94%, transparent) 35%, transparent);
	}
	.subtitles.is-below {
		position: static;
		left: auto;
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
		border-bottom: 1px solid color-mix(in oklab, var(--color-gold-500) 22%, transparent);
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

	/* ── phones: status band, the scene fills the rest, subtitles under it ── */
	@media (max-width: 639px) {
		.cta {
			gap: 0.75rem;
			padding: 0.5rem 0.75rem;
		}
		.facts {
			gap: 0.1rem 0.9rem;
		}
		.status {
			font-size: 0.85rem;
		}
	}
	/* ── ngộ đạo: edge pulse + the teaching on an unrolling scroll ── */
	.edge-pulse {
		position: fixed;
		inset: 0;
		z-index: var(--z-local-3);
		pointer-events: none;
		box-shadow:
			inset 0 0 0 2px color-mix(in oklab, var(--color-gold-300) 70%, transparent),
			inset 0 0 60px 6px color-mix(in oklab, var(--color-gold-300) 35%, transparent);
		opacity: 0;
	}
	:global(:root[data-fx='full']) .edge-pulse {
		animation: edge 1.4s var(--ease-in-out-sine) 3;
	}
	:global(:root[data-fx='lite']) .edge-pulse {
		animation: edge 1.4s var(--ease-in-out-sine) 1;
	}
	@keyframes edge {
		50% {
			opacity: 1;
		}
	}
	.teaching {
		display: flex;
		align-items: stretch;
		margin-inline: auto;
		max-width: 32rem;
		transform-origin: 50% 50%;
		color: var(--color-ink-950);
		filter: drop-shadow(0 8px 18px var(--color-ink-950));
	}
	.teaching p {
		flex: 1;
		padding: 1rem 1.25rem;
		background:
			var(--grain),
			radial-gradient(ellipse at 50% 50%, var(--color-mist-100), color-mix(in oklab, var(--color-mist-300) 85%, var(--color-gold-300)));
	}
	.teaching .rod {
		flex: none;
		width: 0.75rem;
		border-radius: 9999px;
		background: linear-gradient(90deg, var(--color-gold-700), var(--color-gold-300), var(--color-gold-700));
	}
	:global(:root[data-fx='full']) .teaching {
		animation: unroll 700ms var(--ease-qi) both;
	}
	:global(:root[data-fx='lite']) .teaching {
		animation: fade-in var(--dur-base) linear both;
	}
	@keyframes unroll {
		from {
			opacity: 0;
			transform: scaleX(0.06);
		}
		12% {
			opacity: 1;
			transform: scaleX(0.06);
		}
	}
	.note {
		border: 1px solid color-mix(in oklab, var(--color-gold-500) 20%, transparent);
		background: color-mix(in oklab, var(--color-ink-900) 80%, transparent);
	}
</style>
