<!--
	The game: one fixed screen (no page scroll). HUD on top, the cultivation scene in the middle, world chat docked
	on the right (desktop) or one tap away (mobile), dock at the bottom. Features open as windows over the scene
	(shallow routing → Back closes them). All numbers come from the server; the qi bar interpolates for display only.
-->
<script lang="ts">
	import { page } from '$app/state';
	import { MediaQuery } from 'svelte/reactivity';
	import { feed, startRealtime } from '$lib/api/realtime.svelte';
	import BreakthroughCelebration from '$lib/components/BreakthroughCelebration.svelte';
	import BreakthroughPanel from '$lib/components/BreakthroughPanel.svelte';
	import ChatPanel from '$lib/components/ChatPanel.svelte';
	import CultivationScene from '$lib/components/CultivationScene.svelte';
	import DailyTasks from '$lib/components/DailyTasks.svelte';
	import GameDock from '$lib/components/GameDock.svelte';
	import GameHud from '$lib/components/GameHud.svelte';
	import GameWindow from '$lib/components/GameWindow.svelte';
	import Icon from '$lib/components/Icon.svelte';
	import LeaderboardWindow from '$lib/components/LeaderboardWindow.svelte';
	import ProfileWindow from '$lib/components/ProfileWindow.svelte';
	import QiBar from '$lib/components/QiBar.svelte';
	import RankMedal from '$lib/components/RankMedal.svelte';
	import RealmBadge from '$lib/components/RealmBadge.svelte';
	import Toaster from '$lib/components/Toaster.svelte';
	import VipWindow from '$lib/components/VipWindow.svelte';
	import { chat } from '$lib/game/chat.svelte';
	import { formatBp, formatPoints, ratePerDay } from '$lib/game/format';
	import { game } from '$lib/game/store.svelte';
	import { closeWindow, openWindow } from '$lib/game/windows';

	let { data } = $props();

	$effect(() => game.init(data.catalogue, data.character!));
	$effect(() => startRealtime(true));

	const cat = $derived(data.catalogue);
	const ch = $derived(game.character ?? data.character!);
	const win = $derived(page.state.win);
	const wide = new MediaQuery('min-width: 1024px'); // chat docked beside the scene
	$effect(() => {
		if (wide.current && win?.kind === 'chat') closeWindow(); // resized to desktop: chat is already visible
	});

	// Display-only interpolation between server snapshots, bounded by the server-reported rooms
	// (bình cảnh, idle bucket, daily ceiling). Minor stages advance only on the server.
	let shown = $state(0);
	let snapshotAt = Date.now();
	const isLast = $derived(ch.capMilli > 0);
	$effect(() => {
		snapshotAt = Date.now();
		const t = ch.today;
		const room = Math.max(0, Math.min(ch.bottleneckRoomMilli, t.buckets.idle.capMilli - t.buckets.idle.milli, t.ceilingMilli - t.pointsMilli));
		const ceil = isLast ? ch.cultivationMilli + room : Math.min(ch.requiredMilli, ch.cultivationMilli + room);
		shown = ch.cultivationMilli;
		const id = setInterval(() => {
			const secs = (Date.now() - snapshotAt) / 1000;
			shown = Math.min(ceil, Math.floor(ch.cultivationMilli + (ch.idleRateMilliPerHour * secs) / 3600));
		}, 1000);
		return () => clearInterval(id);
	});

	const realm = $derived(cat.realmOf(ch.stageId));
	const aura = $derived(realm?.visual.aura ?? 'jade');
	const stageName = $derived.by(() => {
		const s = cat.stages.get(ch.stageId);
		if (!s || !realm) return '';
		return s.name.startsWith(realm.name) ? s.name.slice(realm.name.length).trim() : s.name;
	});
	const atBottleneck = $derived(isLast && shown >= ch.requiredMilli);
	const pv = $derived(game.preview);
	const btLabel = $derived(pv?.tribulation ? `Độ ${pv.tribulation.name}` : 'Đột phá');
	const lastChat = $derived(chat.messages.slice(-2));

	async function attemptFromWindow() {
		if (await game.attempt()) closeWindow();
	}
	const titles: Record<string, string> = {
		tasks: 'Nhiệm vụ ngày',
		leaderboard: 'Thiên Bảng',
		vip: 'Tiên Duyên',
		breakthrough: 'Đột phá',
		chat: 'Truyền âm · Thế Giới',
		profile: 'Đạo hữu'
	};
</script>

<svelte:head><title>Tiên Đạo</title></svelte:head>

<div class="shell flex h-dvh flex-col overflow-hidden">
	<GameHud />
	{#if feed.items[0]}
		<p class="ticker flex shrink-0 items-center gap-2 truncate px-3 py-1 text-sm text-gold-300" aria-live="polite">
			<Icon name="bolt" size={14} class="shrink-0 text-gold-500" />
			<span class="truncate">{feed.items[0].data.text}</span>
		</p>
	{/if}

	<div class="main grid min-h-0 flex-1 gap-3 p-2 sm:p-3 lg:grid-cols-[minmax(0,1fr)_340px] xl:grid-cols-[280px_minmax(0,1fr)_360px]">
		<!-- Left rail (xl): tasks + top of Thiên Bảng -->
		<aside class="hidden min-h-0 flex-col gap-3 overflow-y-auto xl:flex">
			<div class="panel p-4"><DailyTasks board={game.board} catalogue={cat} busy={game.busy} onclaim={game.claimTask} /></div>
			<section class="panel p-4" aria-labelledby="top-title">
				<h2 id="top-title" class="panel-title text-lg">Thiên Bảng</h2>
				<ol class="mt-2 space-y-1">
					{#each game.top as e (e.characterId)}
						<li>
							<button type="button" class="flex w-full items-center gap-2.5 px-1 py-1.5 text-left hover:bg-gold-500/5" onclick={() => openWindow({ kind: 'profile', characterId: e.characterId, name: e.name, stageId: e.stageId })}>
								<RankMedal rank={e.rank} size={30} />
								<span class="min-w-0 flex-1 truncate text-sm text-mist-100">{e.name}</span>
							</button>
						</li>
					{/each}
				</ol>
				<button class="mt-2 inline-flex items-center gap-1 text-sm text-jade-300 hover:underline" onclick={() => openWindow({ kind: 'leaderboard' })}>
					Xem toàn bảng <Icon name="arrow" size={14} />
				</button>
			</section>
		</aside>

		<!-- Stage -->
		<section class="stage panel relative flex min-h-0 flex-col px-3 pt-3 pb-3 sm:px-6" aria-labelledby="hall-title">
			<header class="pointer-events-none absolute inset-x-0 top-3 z-10 text-center">
				<p class="eyebrow">Đang bế quan tu luyện</p>
				<h1 id="hall-title" class="mt-1.5 flex flex-col items-center gap-1">
					{#if realm}<RealmBadge name={realm.name} aura={realm.visual.aura} size="lg" />{/if}
					<span class="font-display text-lg text-mist-100">{stageName}</span>
				</h1>
			</header>

			<div class="scene-box relative min-h-0 flex-1">
				<div class="scene-fit">
					<CultivationScene {aura} ready={atBottleneck} backlash={game.backlash} />
				</div>
				<div class="pointer-events-none absolute inset-0 flex items-center justify-center" aria-live="polite">
					{#each game.floats as f (f.id)}
						<span class="float font-display text-3xl font-bold text-jade-300">{f.text}</span>
					{/each}
				</div>
			</div>

			<div class="mx-auto w-full max-w-xl shrink-0">
				<QiBar current={shown} required={ch.requiredMilli} cap={ch.capMilli} {aura} label="Tu vi · ước tính" />
				<dl class="stats mt-2 grid grid-cols-4 border-y border-gold-500/15 py-1.5 text-center">
					<div>
						<dt class="eyebrow">Hôm nay</dt>
						<dd class="font-semibold tabular-nums" class:text-jade-300={ch.today.pointsMilli < ch.today.ceilingMilli} class:text-gold-300={ch.today.pointsMilli >= ch.today.ceilingMilli}>
							{formatPoints(ch.today.pointsMilli)}/{formatPoints(ch.today.ceilingMilli)}
						</dd>
					</div>
					<div><dt class="eyebrow">Treo máy</dt><dd class="font-semibold text-jade-300 tabular-nums">{ratePerDay(ch.idleRateMilliPerHour)}</dd></div>
					<div><dt class="eyebrow">Bế quan</dt><dd class="font-semibold text-mist-100 tabular-nums">{Math.round(ch.offlineCapSeconds / 3600)}h</dd></div>
					<div>
						<dt class="eyebrow">Tụ linh</dt>
						<dd class="font-semibold tabular-nums" class:text-jade-300={ch.offlineBonusSeconds > 0} class:text-mist-500={ch.offlineBonusSeconds <= 0}>
							{ch.offlineBonusSeconds > 0 ? `+${ch.offlineBonusSeconds / 3600}h` : '—'}
						</dd>
					</div>
				</dl>
				{#if ch.today.catchUp.active}
					<p class="mt-1 text-center text-xs text-gold-300">Thiên đạo bù đắp người đến sau: tu vi nhận ×{ch.today.catchUp.multiplierBp / 10000}.</p>
				{/if}

				<div class="mt-2.5 grid grid-cols-2 gap-2.5">
					<button class="btn btn-jade h-12 px-2 text-sm whitespace-nowrap sm:text-base" onclick={game.settle} disabled={game.busy} title="Số hiển thị là ước tính — máy chủ kết toán khi bạn thu hoạch.">
						<Icon name="lotus" size={18} /> Thu hoạch
					</button>
					<button class="btn btn-gold h-12 px-2 text-sm whitespace-nowrap sm:text-base" class:is-ready={pv?.ready && !game.busy} onclick={() => openWindow({ kind: 'breakthrough' })}>
						<Icon name="bolt" size={18} stroke={1.8} />
						{btLabel}{#if pv && !pv.missing.some((m) => m.kind === 'final_stage')}<span class="tabular-nums opacity-80">{formatBp(pv.successBp)}</span>{/if}
					</button>
				</div>

				<!-- Mobile/tablet: latest chat lines, tap to open the channel -->
				<button type="button" class="chat-peek mt-2.5 w-full px-3 py-1.5 text-left text-sm lg:hidden" onclick={() => openWindow({ kind: 'chat' })}>
					{#if lastChat.length}
						{#each lastChat as m (m.id)}
							<span class="block truncate"><span class="font-semibold" style:color="var(--color-aura-{cat.realmOf(m.stageId)?.visual.aura ?? 'none'})">{m.name}</span><span class="text-mist-500">:</span> <span class="text-mist-100">{m.text}</span></span>
						{/each}
					{:else}
						<span class="flex items-center gap-2 text-mist-500"><Icon name="chat" size={16} /> Kênh Thế Giới — chạm để trò chuyện</span>
					{/if}
				</button>
			</div>
		</section>

		<!-- Chat (lg+) -->
		{#if wide.current}
			<aside class="panel flex min-h-0 flex-col">
				<ChatPanel />
			</aside>
		{/if}
	</div>

	<GameDock />
</div>

<Toaster />

<GameWindow open={!!win} title={win ? (win.kind === 'profile' ? win.name : (titles[win.kind] ?? '')) : ''} size={win?.kind === 'leaderboard' || win?.kind === 'vip' ? 'lg' : 'md'} onclose={closeWindow}>
	{#if win?.kind === 'tasks'}
		<DailyTasks board={game.board} catalogue={cat} busy={game.busy} onclaim={game.claimTask} />
	{:else if win?.kind === 'leaderboard'}
		<LeaderboardWindow />
	{:else if win?.kind === 'vip'}
		<VipWindow />
	{:else if win?.kind === 'breakthrough'}
		<BreakthroughPanel preview={game.preview} catalogue={cat} failStreak={ch.failStreak} busy={game.busy} onattempt={attemptFromWindow} />
	{:else if win?.kind === 'chat' && !wide.current}
		<div class="-mx-5 -my-4 h-[70dvh]"><ChatPanel compact /></div>
	{:else if win?.kind === 'profile'}
		{#key win.characterId}
			<ProfileWindow characterId={win.characterId} name={win.name} stageId={win.stageId} messageId={win.messageId} />
		{/key}
	{/if}
</GameWindow>

{#if game.celebrate}
	{@const c = game.celebrate}
	{@const r = cat.realmOf(c.to)}
	<BreakthroughCelebration
		realmName={r?.name ?? ''}
		stageName={cat.stages.get(c.to)?.name ?? ''}
		aura={r?.visual.aura ?? 'jade'}
		major={c.kind === 'major'}
		tribulation={c.tribulation ? (game.preview?.tribulation?.name ?? 'Lôi Kiếp') : undefined}
		onclose={() => (game.celebrate = null)}
	/>
{/if}

<style>
	.shell {
		overscroll-behavior: none;
		user-select: none;
	}
	.shell :global(input),
	.shell :global(.msg) {
		user-select: text;
	}
	.ticker {
		background: linear-gradient(90deg, transparent, color-mix(in oklab, var(--color-gold-500) 10%, transparent) 15%, color-mix(in oklab, var(--color-gold-500) 10%, transparent) 85%, transparent);
	}
	/* The scene keeps 4:3 and fits whichever of width/height is tighter. */
	.scene-box {
		container-type: size;
		display: grid;
		place-items: center;
	}
	.scene-fit {
		width: min(100cqw, 133cqh, 520px);
	}
	.stats :global(.eyebrow) {
		font-size: 0.65rem;
	}
	.stats dd {
		font-size: 0.8rem;
		white-space: nowrap;
	}
	@media (min-width: 640px) {
		.stats dd {
			font-size: 0.95rem;
		}
	}
	.chat-peek {
		border: 1px solid color-mix(in oklab, var(--color-gold-500) 18%, transparent);
		background: color-mix(in oklab, var(--color-ink-950) 55%, transparent);
	}
	.float {
		position: absolute;
		animation: float-up 1.6s var(--ease-qi) forwards;
		text-shadow: 0 0 12px var(--color-jade-500);
	}
	@keyframes float-up {
		from {
			opacity: 0;
			transform: translateY(20px) scale(0.9);
		}
		20% {
			opacity: 1;
			transform: translateY(0) scale(1);
		}
		to {
			opacity: 0;
			transform: translateY(-60px);
		}
	}
	.is-ready {
		animation: ready-glow 2.4s ease-in-out infinite;
	}
	@keyframes ready-glow {
		0%,
		100% {
			box-shadow: inset 0 1px 0 color-mix(in oklab, white 35%, transparent);
		}
		50% {
			box-shadow:
				inset 0 1px 0 color-mix(in oklab, white 35%, transparent),
				0 0 22px 2px color-mix(in oklab, var(--color-gold-300) 45%, transparent);
		}
	}
</style>
