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
	const short = new MediaQuery('max-height: 480px');

	const speaking = $derived(hall.speaking);
	const ins = $derived(hall.insight);
	const phase = $derived(ins?.phase ?? 'listening');
	const capacity = $derived(hall.capacity);
	const halls = $derived(hall.state?.halls ?? []);
	let pickerOpen = $state(false);
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

	// short screens: the subtitle line is folded to one line; a tap unfolds it for a while
	const SUB_OPEN_MS = 8000;
	let subsOpen = $state(false);
	let subsTimer: ReturnType<typeof setTimeout> | undefined;
	function toggleSubs() {
		subsOpen = !subsOpen;
		clearTimeout(subsTimer);
		if (subsOpen) subsTimer = setTimeout(() => (subsOpen = false), SUB_OPEN_MS);
	}

	const NAME_MODES = [
		{ id: 'important', label: 'Quan trọng' },
		{ id: 'all', label: 'Tất cả' },
		{ id: 'off', label: 'Ẩn' }
	] as const;

	// phones / short screens: one chip that cycles Quan trọng → Tất cả → Ẩn
	function cycleNames() {
		const i = NAME_MODES.findIndex((m) => m.id === settings.hallNames);
		settings.setHallNames(NAME_MODES[(i + 1) % NAME_MODES.length]!.id);
	}

	async function openPicker() {
		pickerOpen = true;
		hall.switchError = null;
		await hall.refresh();
	}

	async function pick(id: number) {
		if (await hall.switchHall(id)) {
			pickerOpen = false;
			toast('ok', `Đã sang Điện ${id}.`);
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
		<!-- scene slot: the hall painting covers it anchored centre-top (renderer). The header floats OVER the hall
		     (owner review 2026-11-10): a translucent ink gradient, compact, its content box marked data-scene-avoid so
		     seat labels and bubbles keep clear of it; the ngộ đạo button itself is the action bar's primary. -->
		<SceneSlot class="stage" passthrough>
			<header class="overlay" data-phase={phase} aria-label="Ngộ đạo">
				<div class="overlay-box ui" data-scene-avoid>
					<div class="info min-w-0 flex-1">
						<p class="hall-head" data-hall-head>
							{#if hall.hallId > 0}
								<button type="button" class="hall-name press" onclick={openPicker} aria-haspopup="dialog" aria-label="Điện {hall.hallId}, {hall.count} trên {capacity} đạo hữu — đổi điện">
									<Icon name="scroll" size={14} />
									<span>Điện {hall.hallId}</span>
									<span class="text-mist-300">·</span>
									<span class="tabular-nums">{hall.count}/{capacity} đạo hữu</span>
									<Icon name="chevron" size={14} />
								</button>
							{:else}
								<span class="eyebrow">Tử Tiêu Cung · Hồng Quân giảng đạo</span>
							{/if}
							<button type="button" class="names-cycle press" onclick={cycleNames} aria-label="Tên đạo hữu: {NAME_MODES.find((m) => m.id === settings.hallNames)?.label} — đổi" data-names-cycle>
								Tên: {NAME_MODES.find((m) => m.id === settings.hallNames)?.label}
							</button>
							<span class="global" data-global-count><span class="tabular-nums">{hall.globalListeners}</span> đạo hữu đang nghe khắp Hồng Hoang</span>
						</p>
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
						</dl>
						<div class="names" role="radiogroup" aria-label="Tên đạo hữu trong điện" data-names-toggle>
							<span class="names-label">Tên:</span>
							{#each NAME_MODES as m (m.id)}
								<button type="button" role="radio" aria-checked={settings.hallNames === m.id} class="name-mode" class:is-on={settings.hallNames === m.id} onclick={() => settings.setHallNames(m.id)}>{m.label}</button>
							{/each}
						</div>
					</div>

				</div>
				<div class="overlay-box tools ui" data-scene-avoid>
						<button type="button" class="tool press" onclick={openNotebook} aria-label="Sổ Đạo Ngộ">
							<Icon name="scroll" size={16} /> <span class="hidden lg:inline">Sổ Đạo Ngộ</span>
						</button>
						<a href={screens.timeline.href} class="tool press wide-only" aria-label="Thiên Đạo Sử"><Icon name="clock" size={16} /> <span class="hidden lg:inline">Thiên Đạo Sử</span></a>
						<HelpButton topic="hall" />
				</div>
			</header>
			{#if !phone.current}
				<!-- short landscape screens: one line; tap to read it all (folds back after SUB_OPEN_MS) -->
				{#if short.current}
					<div
						class="subtitles ui"
						aria-live="polite"
						data-scene-avoid
						data-hall-subs
						data-open={subsOpen || undefined}
						role="button"
						tabindex="0"
						aria-expanded={subsOpen}
						onclick={toggleSubs}
						onkeydown={(e) => (e.key === 'Enter' || e.key === ' ') && (e.preventDefault(), toggleSubs())}
					>
						{@render subtitle()}
					</div>
				{:else}
					<div class="subtitles" aria-live="polite" data-scene-avoid data-hall-subs>{@render subtitle()}</div>
				{/if}
			{/if}
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

<!-- điện picker: every open điện with its count; full ones are disabled (the server checks again) -->
<GameWindow local open={pickerOpen} title="Chọn điện" icon="scroll" onclose={() => (pickerOpen = false)}>
	<p class="text-sm text-mist-300">
		Mỗi điện có {capacity} bồ đoàn. Lời giảng và ngộ đạo như nhau ở mọi điện; Luận Đạo chỉ đạo hữu cùng điện thấy.
	</p>
	{#if hall.switchError}<p class="mt-2 text-sm text-cinnabar-300" role="alert">{hall.switchError}</p>{/if}
	<ul class="picker mt-3" data-hall-picker aria-label="Các điện">
		{#each halls as h (h.id)}
			{@const here = h.id === hall.hallId}
			{@const full = h.count >= capacity}
			<li>
				<button
					type="button"
					class="pick press"
					class:is-here={here}
					disabled={here || full || hall.switching}
					aria-busy={hall.switching}
					onclick={() => pick(h.id)}
				>
					<span class="font-semibold text-mist-100">Điện <span class="tabular-nums">{h.id}</span></span>
					<span class="meter" role="progressbar" aria-valuemin="0" aria-valuemax={capacity} aria-valuenow={h.count} aria-label="Điện {h.id}: {h.count}/{capacity}">
						<span style:width="{Math.min(100, (h.count / Math.max(1, capacity)) * 100)}%"></span>
					</span>
					<span class="tabular-nums text-sm text-mist-100">{h.count}/{capacity}</span>
					<span class="tag">{here ? 'Đang ở' : full ? 'Kín chỗ' : 'Còn chỗ'}</span>
				</button>
			</li>
		{:else}
			<li><EmptyState icon="scroll" title="Chưa có điện nào mở" hint="Vào Tử Tiêu Cung để mở điện đầu tiên." /></li>
		{/each}
	</ul>
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
	/* ── header overlay: floats over the hall, translucent ink gradient + light blur; only its box takes the pointer ── */
	.overlay {
		position: absolute;
		inset: 0 0 auto;
		z-index: var(--z-local-1);
		display: flex;
		align-items: flex-start;
		justify-content: space-between;
		gap: 0.5rem;
		padding: 0.45rem 0.6rem 1.6rem;
		background: linear-gradient(180deg, color-mix(in oklab, var(--color-ink-950) 82%, transparent) 0%, color-mix(in oklab, var(--color-ink-950) 55%, transparent) 60%, transparent);
		pointer-events: none;
	}
	.overlay[data-phase='ready'] {
		background: linear-gradient(180deg, color-mix(in oklab, var(--color-gold-700) 45%, var(--color-ink-950) 55%) 0%, color-mix(in oklab, var(--color-ink-950) 55%, transparent) 60%, transparent);
	}
	.overlay-box {
		display: flex;
		align-items: flex-start;
		gap: 0.75rem;
		min-width: 0;
		/* narrow enough to leave Hồng Quân (stage centre) clear on tablets; phones use the full width */
		max-width: min(100%, 23rem);
		padding: 0.35rem 0.6rem;
		border-radius: 0.6rem;
		background: color-mix(in oklab, var(--color-ink-950) 35%, transparent);
		backdrop-filter: blur(3px);
		-webkit-backdrop-filter: blur(3px);
		text-shadow: 0 1px 2px var(--color-ink-950);
	}
	.names-cycle {
		display: none;
		min-height: 1.6rem;
		padding: 0 0.5rem;
		font-size: 0.7rem;
		font-weight: 600;
		color: var(--color-mist-100);
		white-space: nowrap;
		border: 1px solid color-mix(in oklab, var(--color-gold-500) 35%, transparent);
		border-radius: 9999px;
		background: color-mix(in oklab, var(--color-ink-950) 55%, transparent);
	}
	@media (max-width: 639px), (max-height: 480px) {
		.names-cycle {
			display: inline-flex;
			align-items: center;
		}
		.names {
			display: none !important;
		}
	}
	.names {
		display: flex;
		align-items: center;
		gap: 0.15rem;
		margin-top: 0.25rem;
		font-size: 0.72rem;
	}
	.names-label {
		margin-right: 0.15rem;
		color: var(--color-mist-300);
	}
	.name-mode {
		min-height: 1.75rem;
		padding: 0 0.5rem;
		color: var(--color-mist-300);
		border: 1px solid color-mix(in oklab, var(--color-gold-500) 22%, transparent);
		border-radius: 9999px;
		background: color-mix(in oklab, var(--color-ink-950) 45%, transparent);
	}
	.name-mode.is-on {
		color: var(--color-ink-950);
		font-weight: 700;
		border-color: var(--color-gold-300);
		background: var(--color-gold-300);
	}
	.overlay-box.tools {
		align-items: center;
		gap: 0.35rem;
		flex-shrink: 0;
		padding: 0.2rem;
	}
	.tool {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		gap: 0.3rem;
		min-width: var(--tap, 44px);
		min-height: var(--tap, 44px);
		padding: 0 0.6rem;
		font-size: 0.85rem;
		font-weight: 600;
		color: var(--color-mist-100);
		border: 1px solid color-mix(in oklab, var(--color-gold-500) 35%, transparent);
		border-radius: 0.5rem;
		background: color-mix(in oklab, var(--color-ink-950) 55%, transparent);
	}
	/* ── điện header + picker ── */
	.hall-head {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: 0.15rem 0.75rem;
		margin-bottom: 0.2rem;
		font-size: 0.8rem;
	}
	.hall-name {
		display: inline-flex;
		align-items: center;
		gap: 0.3rem;
		min-height: 2rem;
		padding: 0.1rem 0.6rem;
		font-weight: 700;
		color: var(--color-gold-300);
		border: 1px solid color-mix(in oklab, var(--color-gold-500) 45%, transparent);
		border-radius: 9999px;
		background: color-mix(in oklab, var(--color-ink-950) 70%, transparent);
	}
	.global {
		color: var(--color-mist-300);
	}
	.global > span {
		font-weight: 700;
		color: var(--color-jade-300);
	}
	.picker {
		display: grid;
		gap: 0.5rem;
		max-height: min(60vh, 28rem);
		overflow-y: auto;
		overscroll-behavior: contain;
	}
	.pick {
		display: grid;
		grid-template-columns: 5.5rem 1fr auto auto;
		align-items: center;
		gap: 0.75rem;
		width: 100%;
		min-height: var(--tap, 44px);
		padding: 0.4rem 0.75rem;
		text-align: left;
		border: 1px solid color-mix(in oklab, var(--color-gold-500) 22%, transparent);
		background: color-mix(in oklab, var(--color-ink-900) 85%, transparent);
	}
	.pick:not(:disabled):hover {
		border-color: color-mix(in oklab, var(--color-gold-300) 60%, transparent);
	}
	.pick:disabled {
		opacity: 0.6;
		cursor: default;
	}
	.pick.is-here {
		opacity: 1;
		border-color: var(--color-gold-300);
	}
	.meter {
		height: 0.4rem;
		border-radius: 9999px;
		background: var(--color-ink-700);
		overflow: hidden;
	}
	.meter > span {
		display: block;
		height: 100%;
		background: linear-gradient(90deg, var(--color-jade-500), var(--color-jade-300));
	}
	.tag {
		font-size: 0.72rem;
		color: var(--color-mist-300);
	}
	@media (max-width: 420px) {
		.pick {
			grid-template-columns: 1fr auto auto;
		}
		.meter {
			grid-column: 1 / -1;
			grid-row: 2;
		}
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
	.facts > div {
		display: flex;
		align-items: baseline;
		gap: 0.3rem;
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
	.tool.wide-only {
		display: none;
	}
	@media (min-width: 768px) and (min-height: 481px) {
		.tool.wide-only {
			display: inline-flex;
		}
	}
	.hall-name,
	.global {
		white-space: nowrap;
	}
	.global {
		flex-basis: 100%;
	}
	@media (max-width: 639px) {
		.overlay-box {
			max-width: 100%;
		}
	}
	/* phones: the tools stack in a slim column so the info keeps its width */
	@media (max-width: 639px) {
		.names-cycle {
		display: none;
		min-height: 1.6rem;
		padding: 0 0.5rem;
		font-size: 0.7rem;
		font-weight: 600;
		color: var(--color-mist-100);
		white-space: nowrap;
		border: 1px solid color-mix(in oklab, var(--color-gold-500) 35%, transparent);
		border-radius: 9999px;
		background: color-mix(in oklab, var(--color-ink-950) 55%, transparent);
	}
	@media (max-width: 639px), (max-height: 480px) {
		.names-cycle {
			display: inline-flex;
			align-items: center;
		}
		.names {
			display: none !important;
		}
	}
	.names {
		display: flex;
		align-items: center;
		gap: 0.15rem;
		margin-top: 0.25rem;
		font-size: 0.72rem;
	}
	.names-label {
		margin-right: 0.15rem;
		color: var(--color-mist-300);
	}
	.name-mode {
		min-height: 1.75rem;
		padding: 0 0.5rem;
		color: var(--color-mist-300);
		border: 1px solid color-mix(in oklab, var(--color-gold-500) 22%, transparent);
		border-radius: 9999px;
		background: color-mix(in oklab, var(--color-ink-950) 45%, transparent);
	}
	.name-mode.is-on {
		color: var(--color-ink-950);
		font-weight: 700;
		border-color: var(--color-gold-300);
		background: var(--color-gold-300);
	}
	.overlay-box.tools {
			flex-direction: column;
		}
		.global {
			font-size: 0.68rem;
		}
	}
	/* short landscape screens: one dense block — status and facts run on in one line */
	@media (max-height: 480px) {
		.overlay-box:not(.tools) {
			max-width: calc(100% - 7.5rem);
		}
		.global {
			flex-basis: auto;
		}
		.info {
			display: flex;
			flex-wrap: wrap;
			align-items: baseline;
			column-gap: 0.8rem;
		}
		.info .hall-head {
			flex-basis: 100%;
			margin-bottom: 0.1rem;
		}
		.info .facts {
			margin-top: 0;
		}
		.hall-name {
			min-height: 1.6rem;
		}
		/* one compact line right of the chat dock (tap to read the whole line; it folds back by itself) — the seat
		   block is fitted above it (HallView measures [data-hall-subs]), so it never sits on figures */
		.subtitles {
			min-height: 0;
			padding: 0.3rem 0.75rem;
			cursor: pointer;
			background: color-mix(in oklab, var(--color-ink-950) 88%, transparent);
			border-top: 1px solid color-mix(in oklab, var(--color-gold-500) 20%, transparent);
		}
		.sub-label {
			display: none;
		}
		.sub-text {
			margin-top: 0;
			font-size: 0.85rem;
			display: -webkit-box;
			-webkit-line-clamp: 1;
			line-clamp: 1;
			-webkit-box-orient: vertical;
			overflow: hidden;
		}
		.subtitles[data-open] .sub-text {
			-webkit-line-clamp: 4;
			line-clamp: 4;
		}
	}
	/* phones and short landscape screens: tighter header */
	@media (max-width: 639px), (max-height: 480px) {
		.overlay {
			padding: 0.3rem 0.35rem 1.2rem;
		}
		.overlay-box {
			gap: 0.4rem;
			padding: 0.25rem 0.4rem;
		}
		.hall-head {
			font-size: 0.72rem;
		}
		.facts {
			gap: 0.05rem 0.8rem;
			margin-top: 0.15rem;
		}
		.facts dt {
			font-size: 0.66rem;
		}
		.facts dd {
			font-size: 0.8rem;
		}
		.status {
			font-size: 0.8rem;
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
