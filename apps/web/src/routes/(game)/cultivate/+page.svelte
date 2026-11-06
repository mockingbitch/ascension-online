<script lang="ts">
	import { api, ApiError, newIdempotencyKey } from '$lib/api/client';
	import { feed } from '$lib/api/realtime.svelte';
	import BreakthroughCelebration from '$lib/components/BreakthroughCelebration.svelte';
	import BreakthroughPanel from '$lib/components/BreakthroughPanel.svelte';
	import CultivationScene from '$lib/components/CultivationScene.svelte';
	import DailyTasks from '$lib/components/DailyTasks.svelte';
	import Icon from '$lib/components/Icon.svelte';
	import Portrait from '$lib/components/Portrait.svelte';
	import QiBar from '$lib/components/QiBar.svelte';
	import RankMedal from '$lib/components/RankMedal.svelte';
	import RealmBadge from '$lib/components/RealmBadge.svelte';
	import { formatPoints, ratePerDay } from '$lib/game/format';
	import type {
		BreakthroughPreview,
		BreakthroughResult,
		CharacterView,
		DailyBoard,
		Inventory,
		LeaderboardEntry,
		VipStatus
	} from '$lib/game/types';

	let { data } = $props();
	const cat = $derived(data.catalogue);

	// Authoritative state from the server; replaced wholesale after every call.
	let ch = $derived<CharacterView>(data.character!); // writable derived: reset on navigation, overwritten by server responses
	let preview = $state<BreakthroughPreview | null>(null);
	let busy = $state(false);
	let message = $state<{ tone: 'ok' | 'warn' | 'err'; text: string } | null>(null);
	let celebrate = $state<BreakthroughResult | null>(null);
	let board = $state<DailyBoard | null>(null);
	let inv = $state<Inventory | null>(null);
	let vip = $state<VipStatus | null>(null);
	let top = $state<LeaderboardEntry[]>([]);
	let myRank = $state<number | null>(null);
	let backlash = $state(0); // bumps on a failed breakthrough to replay the scene's backlash effect

	const PHU = 'tu_linh_phu';
	const phuCount = $derived(inv?.items.find((i) => i.itemId === PHU)?.quantity ?? 0);
	const stones = $derived(inv?.currencies.spirit_stone ?? 0);

	// Display-only interpolation between server snapshots (corrected by the next server response). Bounded by the
	// server-reported rooms (bình cảnh, idle bucket, daily ceiling); minor stages advance only on the server.
	let shown = $state(0);
	let snapshotAt = Date.now();
	const isLast = $derived(ch.capMilli > 0);
	$effect(() => {
		snapshotAt = Date.now();
		const t = ch.today;
		const room = Math.max(
			0,
			Math.min(ch.bottleneckRoomMilli, t.buckets.idle.capMilli - t.buckets.idle.milli, t.ceilingMilli - t.pointsMilli)
		);
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

	// Ranking preview is decorative context: failures leave it empty, never block the screen.
	const loadRanks = () =>
		Promise.all([
			api.leaderboard(0, 3).then((p) => p.entries).catch(() => [] as LeaderboardEntry[]),
			api.myRank().then((r) => r.rank).catch(() => null)
		]).then(([t, r]) => ([top, myRank] = [t, r]));

	async function reload() {
		[ch, preview, board, inv, vip] = await Promise.all([
			api.character(),
			api.breakthroughPreview(),
			api.dailyTasks(),
			api.inventory(),
			api.vipStatus()
		]);
		void loadRanks();
	}
	$effect(() => {
		Promise.all([api.breakthroughPreview(), api.dailyTasks(), api.inventory(), api.vipStatus()]).then(
			([p, b, i, v]) => ([preview, board, inv, vip] = [p, b, i, v])
		);
		void loadRanks();
	});

	// Generic runner for small actions: show the server's message on error, then refresh authoritative state.
	async function run(action: () => Promise<string>) {
		busy = true;
		message = null;
		try {
			message = { tone: 'ok', text: await action() };
			await reload();
		} catch (e) {
			message = { tone: 'err', text: e instanceof ApiError ? e.message : String(e) };
		} finally {
			busy = false;
		}
	}

	const claimTask = (id: string) =>
		run(async () => {
			const t = await api.claimTask(id);
			return `Đã nhận thưởng "${t.name}".`;
		});
	const usePhu = () =>
		run(async () => {
			await api.useItem(PHU, fetch, newIdempotencyKey());
			return 'Tụ Linh Phù đã kích hoạt: lần bế quan tới được thêm 4 canh giờ.';
		});
	const claimGift = () =>
		run(async () => {
			const g = await api.claimVipGift();
			return `Nhận quà VIP: ${g.amount} linh thạch.`;
		});

	// Idempotency keys survive network errors so a retry replays the same intent; cleared on a definitive answer.
	let settleKey: string | null = null;
	let attemptKey: string | null = null;
	// 4xx = the server answered and wrote nothing; 5xx/network = unknown outcome, keep the key for the retry.
	const definitive = (e: unknown) => e instanceof ApiError && e.status < 500;

	async function settle() {
		busy = true;
		message = null;
		settleKey ??= newIdempotencyKey();
		try {
			const st = await api.settle(fetch, settleKey);
			settleKey = null;
			const notes = [
				st.hitOfflineCap && 'đã chạm giới hạn bế quan',
				st.hitDailyCap && 'đã chạm hạn mức hôm nay',
				st.hitBottleneck && st.capMilli > 0 && 'bình cảnh đã đầy',
				st.stagesAdvanced.length && `lên ${st.stagesAdvanced.length} tầng`
			].filter(Boolean);
			message = { tone: 'ok', text: `Thu được ${formatPoints(st.earnedMilli)} tu vi${notes.length ? ` (${notes.join(', ')})` : ''}.` };
			await reload();
		} catch (e) {
			if (definitive(e)) settleKey = null;
			message = { tone: 'err', text: e instanceof ApiError ? e.message : String(e) };
		} finally {
			busy = false;
		}
	}

	async function attempt() {
		busy = true;
		message = null;
		attemptKey ??= newIdempotencyKey(); // reused if we must retry after a network error
		// Guard bound from the preview (the shown loss + a short accrual window). The server refuses with
		// 409 quote_changed if the loss it would apply is larger; it never uses the value otherwise.
		const guard = preview?.tribulation && !preview.tribulation.guaranteed ? preview.tribulation.maxLossMilli : undefined;
		try {
			const r = await api.breakthroughAttempt(fetch, attemptKey, guard);
			attemptKey = null;
			if (r.success) {
				celebrate = r;
			} else {
				backlash++;
				message = {
					tone: 'warn',
					text: r.tribulation
						? `Độ kiếp thất bại — mất ${formatPoints(r.lossMilli)} tu vi, cảnh giới giữ nguyên. Thất bại ${r.failStreak} lần liên tiếp.`
						: 'Đột phá thất bại — tu vi vẫn còn nguyên. Tâm cảnh vững hơn: lần sau tỉ lệ sẽ tăng.'
				};
			}
			await reload();
		} catch (e) {
			if (definitive(e)) attemptKey = null;
			if (e instanceof ApiError && (e.code === 'not_ready' || e.code === 'quote_required')) {
				preview = e.details as BreakthroughPreview;
				message = { tone: 'warn', text: 'Chưa đủ điều kiện đột phá.' };
			} else if (e instanceof ApiError && e.code === 'quote_changed') {
				preview = e.details as BreakthroughPreview;
				message = {
					tone: 'warn',
					text: `Tu vi vừa thay đổi: nếu thất bại sẽ mất ${formatPoints(preview.tribulation?.lossMilli ?? 0)} tu vi. Xem lại rồi xác nhận.`
				};
			} else {
				message = { tone: 'err', text: e instanceof ApiError ? e.message : String(e) };
			}
		} finally {
			busy = false;
		}
	}
</script>

<svelte:head><title>Tu luyện · Tiên Đạo</title></svelte:head>

<div class="hall grid grid-cols-[minmax(0,1fr)] gap-5 lg:grid-cols-[300px_minmax(0,1fr)_300px]">
	<!-- ───── CENTER: the cultivation stage (first on mobile) ───── -->
	<section class="panel order-1 px-5 pt-4 pb-6 sm:px-8 lg:order-2" aria-labelledby="hall-title">
		<div class="relative">
			<header class="pointer-events-none absolute inset-x-0 top-0 z-10 text-center">
				<p class="eyebrow">Đang bế quan tu luyện</p>
				<h1 id="hall-title" class="mt-2 flex flex-col items-center gap-1.5">
					{#if realm}<RealmBadge name={realm.name} aura={realm.visual.aura} size="lg" />{/if}
					<span class="font-display text-xl text-mist-100">{stageName}</span>
				</h1>
			</header>
			<CultivationScene {aura} ready={atBottleneck} {backlash} />
		</div>

		<div class="mx-auto mt-1 max-w-xl">
			<QiBar current={shown} required={ch.requiredMilli} cap={ch.capMilli} {aura} label="Tu vi · ước tính" />
			<dl class="mt-3 grid grid-cols-2 border-y border-gold-500/15 py-2.5 text-center sm:grid-cols-4">
				<div>
					<dt class="eyebrow">Hôm nay</dt>
					<dd class="mt-0.5 text-lg font-semibold tabular-nums" class:text-jade-300={ch.today.pointsMilli < ch.today.ceilingMilli} class:text-gold-300={ch.today.pointsMilli >= ch.today.ceilingMilli}
						title="Điểm tu vi đã tích trong ngày (reset 0h). Treo máy {formatPoints(ch.today.buckets.idle.milli)}/{formatPoints(ch.today.buckets.idle.capMilli)} · giảng đạo/nhiệm vụ {formatPoints(ch.today.buckets.activity.milli)}/{formatPoints(ch.today.buckets.activity.capMilli)} · cộng đồng {formatPoints(ch.today.buckets.community.milli)}/{formatPoints(ch.today.buckets.community.capMilli)}">
						{formatPoints(ch.today.pointsMilli)}/{formatPoints(ch.today.ceilingMilli)}
					</dd>
				</div>
				<div class="sm:border-l sm:border-gold-500/15">
					<dt class="eyebrow">Treo máy</dt>
					<dd class="mt-0.5 text-lg font-semibold text-jade-300 tabular-nums">{ratePerDay(ch.idleRateMilliPerHour)}</dd>
				</div>
				<div class="sm:border-x sm:border-gold-500/15">
					<dt class="eyebrow">Bế quan tối đa</dt>
					<dd class="mt-0.5 text-lg font-semibold text-mist-100 tabular-nums">{Math.round(ch.offlineCapSeconds / 3600)} canh giờ</dd>
				</div>
				<div>
					<dt class="eyebrow">Tụ linh</dt>
					<dd class="mt-0.5 text-lg font-semibold tabular-nums" class:text-jade-300={ch.offlineBonusSeconds > 0} class:text-mist-500={ch.offlineBonusSeconds <= 0}>
						{ch.offlineBonusSeconds > 0 ? `+${ch.offlineBonusSeconds / 3600}h` : '—'}
					</dd>
				</div>
			</dl>

			{#if ch.today.catchUp.active}
				<p class="mt-2 text-center text-sm text-gold-300">Thiên đạo bù đắp người đến sau: tu vi nhận ×{ch.today.catchUp.multiplierBp / 10000}.</p>
			{/if}
			{#if ch.grantReserveMilli > 0}
				<p class="mt-1 text-center text-sm text-mist-300">Tiên duyên khởi đầu còn {formatPoints(ch.grantReserveMilli)} tu vi, sẽ nhập vào khi qua bình cảnh.</p>
			{/if}

			{#if message}
				<p
					role="status"
					class="toast mt-4 flex items-start gap-2 px-4 py-3 text-sm"
					class:tone-ok={message.tone === 'ok'}
					class:tone-warn={message.tone === 'warn'}
					class:tone-err={message.tone === 'err'}
				>
					<Icon name={message.tone === 'ok' ? 'spark' : message.tone === 'warn' ? 'clock' : 'cross'} size={18} />
					<span>{message.text}</span>
				</p>
			{/if}

			<div class="mt-4">
				<BreakthroughPanel {preview} catalogue={cat} failStreak={ch.failStreak} {busy} onattempt={attempt}>
					<button class="btn btn-jade h-12 w-full text-base" onclick={settle} disabled={busy} title="Số hiển thị là ước tính — máy chủ kết toán khi bạn thu hoạch.">
						<Icon name="lotus" size={18} /> Thu hoạch tu vi
					</button>
				</BreakthroughPanel>
			</div>
		</div>
	</section>

	<!-- ───── LEFT: character plate + resources + daily tasks ───── -->
	<aside class="order-2 space-y-5 lg:order-1">
		<section class="panel p-5" aria-label="Nhân vật">
			<div class="flex items-center gap-4">
				<Portrait {aura} size={80} />
				<div class="min-w-0">
					<p class="eyebrow">Đạo hiệu</p>
					<p class="line-clamp-2 font-display text-2xl leading-tight font-bold text-mist-100">{ch.name}</p>
					{#if vip}
						<a href="/vip" class="mt-1 inline-flex items-center gap-1 text-sm text-gold-300 hover:underline">
							<Icon name="seal" size={15} />{vip.benefits.name}
						</a>
					{/if}
				</div>
			</div>
			<dl class="mt-5 space-y-2.5 text-sm">
				<div class="flex items-center justify-between">
					<dt class="text-mist-300">Cảnh giới</dt>
					<dd class="font-medium text-mist-100">{cat.stageLabel(ch.stageId)}</dd>
				</div>
				<div class="flex items-center justify-between">
					<dt class="text-mist-300">Thiên Bảng</dt>
					<dd class="font-medium text-gold-300 tabular-nums">{myRank ? `Hạng ${myRank}` : '—'}</dd>
				</div>
			</dl>

			<h3 class="panel-title mt-5 text-base">Tài nguyên</h3>
			<ul class="mt-3 space-y-2 text-sm">
				<li class="flex items-center gap-2.5">
					<Icon name="stone" size={18} class="text-spirit-cyan" />
					<span class="flex-1 text-mist-300">Linh thạch</span>
					<span class="font-semibold text-mist-100 tabular-nums">{stones.toLocaleString('vi-VN')}</span>
				</li>
				<li class="flex items-center gap-2.5">
					<Icon name="talisman" size={18} class="text-gold-500" />
					<span class="flex-1 text-mist-300">Tụ Linh Phù</span>
					<span class="font-semibold text-mist-100 tabular-nums">{phuCount}</span>
				</li>
			</ul>
			{#if ch.offlineBonusSeconds <= 0 && phuCount > 0 && !ch.bonusUsedToday}
				<button class="btn btn-ghost mt-4 w-full" disabled={busy} onclick={usePhu}>
					<Icon name="talisman" size={16} /> Dùng Tụ Linh Phù
				</button>
			{/if}
			{#if vip && vip.benefits.dailyGift > 0}
				{#if vip.giftClaimedToday}
					<p class="mt-4 flex items-center gap-2 text-sm text-mist-500"><Icon name="check" size={16} /> Quà VIP hôm nay đã nhận</p>
				{:else}
					<button class="btn btn-ghost mt-4 w-full" disabled={busy} onclick={claimGift}>
						<Icon name="gift" size={16} /> Nhận quà VIP · {vip.benefits.dailyGift} linh thạch
					</button>
				{/if}
			{/if}
		</section>

		<div class="panel p-5"><DailyTasks {board} catalogue={cat} {busy} onclaim={claimTask} /></div>
	</aside>

	<!-- ───── RIGHT: world context ───── -->
	<aside class="order-3 space-y-5">
		<section class="panel p-5" aria-labelledby="rank-title">
			<h2 id="rank-title" class="panel-title">Thiên Bảng</h2>
			{#if top.length}
				<ol class="mt-3 space-y-1">
					{#each top as e (e.characterId)}
						{@const r = cat.realmOf(e.stageId)}
						<li class="flex items-center gap-3 px-1 py-2" class:me={e.characterId === ch.id}>
							<RankMedal rank={e.rank} size={36} />
							<div class="min-w-0 flex-1">
								<p class="truncate font-medium text-mist-100">{e.name}</p>
								<p class="truncate text-xs" style:color="var(--color-aura-{r?.visual.aura ?? 'none'})">{cat.stageLabel(e.stageId)}</p>
							</div>
						</li>
					{/each}
				</ol>
			{:else}
				<p class="mt-3 text-sm text-mist-500">Thiên Bảng còn trống.</p>
			{/if}
			<div class="mt-3 flex items-center justify-between border-t border-gold-500/15 pt-3 text-sm">
				<span class="text-mist-300">Hạng của bạn: <span class="font-semibold text-gold-300 tabular-nums">{myRank ?? '—'}</span></span>
				<a href="/leaderboard" class="inline-flex items-center gap-1 text-jade-300 hover:underline">Xem toàn bảng <Icon name="arrow" size={14} /></a>
			</div>
		</section>

		<section class="panel p-5" aria-labelledby="feed-title">
			<h2 id="feed-title" class="panel-title">Thiên hạ truyền âm</h2>
			{#if feed.items.length}
				<ul class="mt-3 space-y-3">
					{#each feed.items as a (a.id)}
						<li class="flex gap-2.5 text-sm">
							<Icon name="bolt" size={16} class="mt-0.5 text-gold-500" />
							<span class="text-mist-300">{a.data.text}</span>
						</li>
					{/each}
				</ul>
			{:else}
				<p class="mt-3 text-sm text-mist-500">Thiên địa tĩnh lặng — chưa có ai đột phá đại cảnh giới.</p>
			{/if}
		</section>
	</aside>
</div>

{#if celebrate}
	{@const r = cat.realmOf(celebrate.to)}
	<BreakthroughCelebration
		realmName={r?.name ?? ''}
		stageName={cat.stages.get(celebrate.to)?.name ?? ''}
		aura={r?.visual.aura ?? 'jade'}
		major={celebrate.kind === 'major'}
		tribulation={celebrate.tribulation ? (preview?.tribulation?.name ?? 'Lôi Kiếp') : undefined}
		onclose={() => (celebrate = null)}
	/>
{/if}

<style>
	.me {
		background: linear-gradient(90deg, color-mix(in oklab, var(--color-gold-500) 14%, transparent), transparent);
	}
	.toast {
		border-left: 2px solid currentColor;
		background: color-mix(in oklab, var(--color-ink-950) 55%, transparent);
	}
	.tone-ok {
		color: var(--color-jade-300);
	}
	.tone-warn {
		color: var(--color-gold-300);
	}
	.tone-err {
		color: var(--color-cinnabar-300);
	}
	.toast :global(span) {
		color: var(--color-mist-100);
	}
</style>
