<!--
	Đạo Lữ (window, Phase 5.1, live): proposals inbox, the bond card with the partner's presence and the song tu panel.
	Everything is server state (GET /dao-lu, replaced after every action). Song tu: while this screen is open and a session
	is started, a heartbeat goes out every heartbeatSeconds; the SERVER measures the overlap (both partners active, server
	clock) and pays it only on "Nhận tu vi" (community bucket, inside the 150 ceiling). No client-side rewards.
	Entry from a player card: window state `propose` (deep link /play/dao-lu?cau-than=<id>&ten=<name>) opens the proposal.
-->
<script lang="ts">
	import { ApiError, api, newIdempotencyKey } from '$lib/api/client';
	import GameWindow from '$lib/components/GameWindow.svelte';
	import Icon from '$lib/components/Icon.svelte';
	import Portrait from '$lib/components/Portrait.svelte';
	import { chat } from '$lib/game/chat.svelte';
	import { formatPoints } from '$lib/game/format';
	import { game } from '$lib/game/store.svelte';
	import { toast } from '$lib/game/toasts.svelte';
	import type { DaoLuBlocker, DaoLuView, SongTuView } from '$lib/game/types';
	import { untrack } from 'svelte';
	import type { WindowProps } from './registry';

	let { win }: WindowProps = $props();
	const target = $derived(win.kind === 'companion' ? win.propose : undefined);

	const ch = $derived(game.character!);
	const cat = $derived(game.catalogue!);
	const aura = (stageId: string) => cat.realmOf(stageId)?.visual.aura ?? 'none';

	let view = $state<DaoLuView | null>(null);
	let failed = $state(false);
	let busy = $state(false);
	const st = $derived<SongTuView | null>(view?.songTu ?? null);
	const rules = $derived(view?.rules);
	const minRealm = $derived(
		rules ? [...cat.realms.values()].find((r) => r.tier === rules.minTier && r.path === cat.stages.get(ch.stageId)?.path) : undefined
	);

	const blockerText: Record<DaoLuBlocker, (v: DaoLuView) => string> = {
		account_age: (v) => `Tài khoản cần ít nhất ${v.rules.minAccountAgeDays} ngày tuổi.`,
		tier: () => `Cần đạt ${minRealm?.name ?? 'cảnh giới tối thiểu'} trở lên.`,
		bonded: () => 'Đạo hữu đã có đạo lữ.',
		cooldown: (v) => `Vừa chấm dứt một mối đạo lữ — chờ đến ${v.cooldownUntil ? dateText(v.cooldownUntil) : 'hết thời gian chờ'}.`
	};

	function dateText(iso: string) {
		return new Date(iso).toLocaleString('vi-VN', { day: '2-digit', month: '2-digit', hour: '2-digit', minute: '2-digit' });
	}
	function dur(secs: number) {
		const h = Math.floor(secs / 3600);
		const m = Math.floor((secs % 3600) / 60);
		return h > 0 ? `${h} giờ ${m.toString().padStart(2, '0')} phút` : `${m} phút`;
	}

	async function load() {
		try {
			view = await api.daoLu();
			failed = false;
		} catch {
			failed = true;
		}
	}

	// Full refresh every 30 s while open (proposals arrive, the partner comes and goes).
	$effect(() => {
		untrack(() => void load());
		const t = setInterval(() => {
			if (document.visibilityState === 'visible') void load();
		}, 30_000);
		return () => clearInterval(t);
	});

	// Song tu heartbeat: only while a session is started today and the tab is visible. The server measures time itself.
	const beating = $derived(!!st?.available && !!st.sessionStarted && !!view?.bond);
	const beatEvery = $derived((st?.heartbeatSeconds ?? 30) * 1000);
	$effect(() => {
		if (!beating) return;
		const every = beatEvery;
		const t = setInterval(async () => {
			if (document.visibilityState !== 'visible' || !view) return;
			try {
				const s = await api.songTuHeartbeat();
				if (view) view = { ...view, songTu: s };
			} catch (e) {
				if (e instanceof ApiError && e.code === 'no_session') void load(); // game-day reset: a new session is needed
			}
		}, every);
		return () => clearInterval(t);
	});

	function errMsg(e: unknown) {
		if (e instanceof ApiError) {
			const d = e.details as { side?: string; blockers?: DaoLuBlocker[] } | undefined;
			if (e.code === 'not_eligible' && d?.blockers?.length && view) {
				return `${e.message}: ${d.blockers.map((b) => blockerText[b](view!)).join(' ')}`;
			}
			return e.message;
		}
		return 'Không gửi được, thử lại sau.';
	}
	const definitive = (e: unknown) => e instanceof ApiError && e.status >= 400 && e.status < 500;

	// ───── proposal / accept: double confirmation ─────
	type Confirm = { mode: 'propose' | 'accept'; id: string; name: string; stageId: string; key: string };
	let confirm = $state<Confirm | null>(null);
	let typed = $state('');

	function openConfirm(mode: Confirm['mode'], id: string, name: string, stageId: string) {
		typed = '';
		confirm = { mode, id, name, stageId, key: newIdempotencyKey() };
	}

	async function submitConfirm() {
		if (!confirm) return;
		const c = confirm;
		busy = true;
		try {
			view = c.mode === 'propose' ? await api.daoLuPropose(c.id, c.key) : await api.daoLuAccept(c.id, c.key);
			confirm = null;
			toast('ok', c.mode === 'propose' ? `Đã gửi lời cầu thân tới ${c.name}.` : `Lời thề đã lập — ${c.name} là đạo lữ của đạo hữu.`);
		} catch (e) {
			if (definitive(e)) confirm = null;
			toast('err', errMsg(e));
			void load();
		} finally {
			busy = false;
		}
	}

	async function act(fn: () => Promise<DaoLuView>, ok: string) {
		busy = true;
		try {
			view = await fn();
			toast('ok', ok);
		} catch (e) {
			toast('err', errMsg(e));
			void load();
		} finally {
			busy = false;
		}
	}

	// Entry from a player card (ProfileWindow "Cầu thân").
	let handledTarget = '';
	$effect(() => {
		const id = target?.characterId;
		if (!id || id === handledTarget || !view) return;
		handledTarget = id;
		untrack(() => {
			if (view?.bond) toast('warn', 'Đạo hữu đã có đạo lữ.');
			else if (id !== ch.id) openConfirm('propose', id, target?.name ?? 'đạo hữu', target?.stageId ?? '');
		});
	});

	// ───── song tu ─────
	let claimKey: string | null = null;
	async function startSongTu() {
		busy = true;
		try {
			const s = await api.songTuStart();
			if (view) view = { ...view, songTu: s };
			toast('ok', 'Song tu khai mở — giữ màn hình này mở, cả hai cùng tu luyện.');
		} catch (e) {
			toast('err', errMsg(e));
		} finally {
			busy = false;
		}
	}
	async function claim() {
		busy = true;
		claimKey ??= newIdempotencyKey();
		try {
			const r = await api.songTuClaim(claimKey);
			claimKey = null;
			if (view) view = { ...view, songTu: r.songTu };
			const clipped = r.earnedMilli < r.rawMilli ? ' (phần vượt hạn mức hôm nay không được tính)' : '';
			toast('ok', `Song tu ${dur(r.seconds)}: +${formatPoints(r.earnedMilli)} tu vi${r.vipPoints ? `, +${r.vipPoints} điểm duyên` : ''}${clipped}.`);
			await game.reload();
		} catch (e) {
			if (definitive(e)) claimKey = null;
			toast('err', errMsg(e));
		} finally {
			busy = false;
		}
	}

	// Candidates: people seen in world chat (the server checks every gate on propose).
	const candidates = $derived(
		[...new Map(chat.messages.filter((m) => m.characterId !== ch.id).map((m) => [m.characterId, m])).values()].slice(-8).reverse()
	);
	const pending = $derived(view?.outgoing[0] ?? null);
	const partner = $derived(view?.bond?.partner ?? null);
	// Present = online (presence ping) or active (a game action within the activity window, as song tu reads it).
	const partnerHere = $derived(!!partner?.online || !!st?.partnerActive);
	// red thread: song tu running today (server session); heartbeat: both present (I am here, partner online/active)
	const songTuOn = $derived(!!partner && !!st?.available && !!st.sessionStarted);
	const pct = $derived(st ? Math.min(100, Math.round((st.accruedSeconds / Math.max(1, st.windowSeconds)) * 100)) : 0);
	const paidPct = $derived(st ? Math.min(100, Math.round((st.paidSeconds / Math.max(1, st.windowSeconds)) * 100)) : 0);
</script>


	<section class="bond relative overflow-hidden px-4 py-7">
		<div class="petals" aria-hidden="true">
			{#each Array.from({ length: 14 }, (_, i) => i) as i (i)}<span style:left="{(i * 37) % 100}%" style:animation-delay="{(i * 0.7) % 8}s"></span>{/each}
		</div>
		<div class="relative mx-auto flex max-w-2xl items-center justify-between gap-2">
			<div class="flex min-w-0 flex-col items-center text-center">
				<div class="ring" class:is-beat={!!partner && partnerHere} data-beat={!!partner && partnerHere}><Portrait aura={aura(ch.stageId)} size={96} /></div>
				<p class="mt-2 max-w-[9rem] truncate font-display text-xl font-bold text-mist-100">{ch.name}</p>
				<p class="text-xs text-mist-300">{cat.stageLabel(ch.stageId)}</p>
			</div>
			<div class="link flex-1" class:is-on={!!partner} class:is-thread={songTuOn} class:is-pending={!partner && !!pending} aria-hidden="true" data-thread={songTuOn}>
				{#if songTuOn}<span class="spark"></span>{/if}
			</div>
			<div class="flex min-w-0 flex-col items-center text-center">
				{#if partner}
					<div class="ring relative" class:is-beat={partnerHere}>
						<Portrait aura={aura(partner.stageId)} size={96} />
						<span class="dot" class:on={partnerHere} title={partnerHere ? 'Đang trực tuyến' : 'Ngoại tuyến'}></span>
					</div>
					<p class="mt-2 max-w-[9rem] truncate font-display text-xl font-bold text-mist-100">{partner.name}</p>
					<p class="text-xs" class:text-jade-300={partnerHere} class:text-mist-500={!partnerHere}>
						{partnerHere ? 'Đang trực tuyến' : 'Ngoại tuyến'}
					</p>
				{:else if pending}
					<div class="ring"><Portrait aura={aura(pending.stageId)} size={96} /></div>
					<p class="mt-2 max-w-[9rem] truncate font-display text-xl font-bold text-mist-100">{pending.name}</p>
					<p class="text-xs text-gold-300">Chờ hồi đáp đến {dateText(pending.expiresAt)}</p>
				{:else}
					<div class="ring empty grid size-[104px] place-items-center text-mist-500"><Icon name="heart" size={34} /></div>
					<p class="mt-2 font-display text-xl text-mist-500">Chưa có đạo lữ</p>
				{/if}
			</div>
		</div>
		{#if view?.bond}
			<p class="relative mt-4 text-center text-xs text-mist-300">Kết đạo lữ từ {dateText(view.bond.formedAt)} · trọn một đời tu</p>
		{/if}
	</section>

	{#if failed && !view}
		<p class="mt-4 text-sm text-cinnabar-300">Không tải được Đào Hoa Đình. <button class="underline" onclick={load}>Thử lại</button></p>
	{/if}

	{#if view?.bond && st}
		<!-- Song tu -->
		<section class="mt-5">
			<h2 class="panel-title">Song tu hôm nay</h2>
			<div class="mt-3 grid gap-3 sm:grid-cols-2">
				<div class="flex items-center gap-2 text-sm">
					<span class="dot static" class:on={st.meActive}></span>
					<span>Bản thân: {st.meActive ? 'đang tu luyện' : 'chưa có thao tác gần đây'}</span>
				</div>
				<div class="flex items-center gap-2 text-sm">
					<span class="dot static" class:on={st.partnerActive}></span>
					<span>
						{partner?.name}: {st.partnerActive ? 'đang tu luyện' : 'chưa có mặt'}{#if st.partnerBeatAt} · nhịp cuối {dateText(st.partnerBeatAt)}{/if}
					</span>
				</div>
			</div>
			<div class="mt-4">
				<div class="flex justify-between text-xs text-mist-300">
					<span>Thời gian song tu chung</span>
					<span class="tabular-nums">{dur(st.accruedSeconds)} / {dur(st.windowSeconds)}</span>
				</div>
				<div class="meter mt-1" role="progressbar" aria-valuemin={0} aria-valuemax={st.windowSeconds} aria-valuenow={st.accruedSeconds} aria-label="Thời gian song tu chung">
					<div class="fill" style:width="{pct}%"></div>
					<div class="paid" style:width="{paidPct}%"></div>
				</div>
				<p class="mt-1 text-xs text-mist-500">Đã nhận {dur(st.paidSeconds)} · chờ nhận {dur(st.claimableSeconds)}{st.claimableMilli > 0 ? ` (≈ ${formatPoints(st.claimableMilli)} tu vi)` : ''}</p>
			</div>
			{#if !st.available}
				<p class="mt-3 text-sm text-mist-500">Song tu tạm đóng trên máy chủ này.</p>
			{:else if !st.sessionStarted}
				<button class="btn btn-jade mt-4 w-full" disabled={busy} onclick={startSongTu}><Icon name="lotus" size={16} /> Khai mở song tu</button>
				<p class="mt-2 text-xs text-mist-500">Cần cả hai cùng đang tu luyện (có thao tác trong game gần đây). Một người khai mở là đủ cho cả hai.</p>
			{:else}
				<div class="mt-4 grid gap-2 sm:grid-cols-2">
					<p class="flex items-center gap-2 text-sm text-jade-300"><Icon name="spark" size={16} /> Đang song tu — giữ màn hình này mở.</p>
					<button class="btn btn-gold" disabled={busy || st.claimableSeconds <= 0} onclick={claim}>
						<Icon name="gift" size={16} /> Nhận tu vi song tu
					</button>
				</div>
				<p class="mt-2 text-xs text-mist-500">
					Chỉ tính lúc cả hai cùng có mặt, tối đa {dur(st.windowSeconds)} mỗi ngày, vẫn trong hạn mức tu vi hằng ngày. Phần chưa nhận mất khi sang ngày mới
					({dateText(st.resetAt)}).
				</p>
			{/if}
		</section>
	{/if}

	{#if view && !view.bond}
		{#if view.blockers.length}
			<p class="mt-5 flex items-start gap-2 border-l-2 border-gold-500 bg-ink-950/40 p-3 text-sm text-mist-300">
				<Icon name="lock" size={16} class="mt-0.5 shrink-0" />
				<span>{view.blockers.map((b) => blockerText[b](view!)).join(' ')}</span>
			</p>
		{/if}

		<div class="mt-5 grid gap-5 lg:grid-cols-2">
			<section>
				<h2 class="panel-title">Lời cầu thân gửi tới</h2>
				{#if view.incoming.length}
					<ul class="mt-2 divide-y divide-gold-500/10">
						{#each view.incoming as p (p.proposalId)}
							<li class="flex flex-wrap items-center gap-2 py-2">
								<span class="min-w-0 flex-1 truncate">
									<span class="font-semibold" style:color="var(--color-aura-{aura(p.stageId)})">{p.name}</span>
									<span class="text-xs text-mist-500">{cat.stageLabel(p.stageId)} · đến {dateText(p.expiresAt)}</span>
								</span>
								<button class="btn btn-gold h-9 px-3 text-sm" disabled={busy || view.blockers.length > 0} onclick={() => openConfirm('accept', p.proposalId, p.name, p.stageId)}>
									Nhận lời
								</button>
								<button class="btn btn-ghost h-9 px-3 text-sm" disabled={busy} onclick={() => act(() => api.daoLuDecline(p.proposalId), `Đã từ chối ${p.name}.`)}>
									Từ chối
								</button>
							</li>
						{/each}
					</ul>
				{:else}
					<p class="mt-2 text-sm text-mist-500">Chưa có ai cầu thân.</p>
				{/if}

				{#if pending}
					<h2 class="panel-title mt-5">Lời cầu thân đã gửi</h2>
					<div class="mt-2 flex items-center gap-2">
						<span class="min-w-0 flex-1 truncate text-sm">Gửi <strong class="text-mist-100">{pending.name}</strong>, chờ đến {dateText(pending.expiresAt)}</span>
						<button class="btn btn-ghost h-9 px-3 text-sm" disabled={busy} onclick={() => act(() => api.daoLuCancel(pending.proposalId), 'Đã rút lại lời cầu thân.')}>
							Rút lại
						</button>
					</div>
				{/if}
			</section>

			<section>
				<h2 class="panel-title">Đạo hữu vừa truyền âm</h2>
				{#if candidates.length}
					<ul class="mt-2 divide-y divide-gold-500/10">
						{#each candidates as c (c.characterId)}
							<li class="flex items-center gap-3 py-2">
								<span class="min-w-0 flex-1 truncate">
									<span class="font-semibold" style:color="var(--color-aura-{aura(c.stageId)})">{c.name}</span>
									<span class="text-xs text-mist-500">{cat.stageLabel(c.stageId)}</span>
								</span>
								<button
									class="btn btn-ghost h-9 px-3 text-sm"
									disabled={busy || !!pending || view.blockers.length > 0 || view.proposalsLeft <= 0}
									onclick={() => openConfirm('propose', c.characterId, c.name, c.stageId)}
								>
									Cầu thân
								</button>
							</li>
						{/each}
					</ul>
				{:else}
					<p class="mt-2 text-sm text-mist-500">Chưa thấy ai trong kênh Thế Giới. Có thể cầu thân từ thẻ nhân vật của một đạo hữu.</p>
				{/if}
				<p class="mt-2 text-xs text-mist-500">Còn {view.proposalsLeft}/{view.rules.dailyProposals} lời cầu thân hôm nay · mỗi lời có hạn {view.rules.proposalExpiryHours} giờ.</p>
			</section>
		</div>
	{/if}

	{#if rules}
		<dl class="mt-6 grid grid-cols-2 gap-2 text-center text-sm sm:grid-cols-4">
			<div class="border border-gold-500/15 p-2"><dt class="eyebrow">Song tu</dt><dd>{dur(rules.windowSeconds)}/ngày</dd></div>
			<div class="border border-gold-500/15 p-2"><dt class="eyebrow">Tối đa</dt><dd class="text-jade-300">+{formatPoints(rules.maxSongTuMilli)} tu vi/ngày</dd></div>
			<div class="border border-gold-500/15 p-2"><dt class="eyebrow">Điều kiện</dt><dd>{minRealm?.name ?? '—'} · {rules.minAccountAgeDays} ngày</dd></div>
			<div class="border border-gold-500/15 p-2"><dt class="eyebrow">Chấm dứt</dt><dd>lượng kiếp · vắng {rules.absenceDays} ngày</dd></div>
		</dl>
	{/if}

<GameWindow local icon="heart" open={!!confirm} title={confirm?.mode === 'accept' ? 'Nhận lời — xác nhận lần hai' : 'Lời thề đạo lữ'} onclose={() => (confirm = null)}>
	{#if confirm}
		<p class="flex items-start gap-2 border-l-2 border-cinnabar-500 bg-ink-950/50 p-3 text-sm text-cinnabar-300">
			<Icon name="bolt" size={18} class="mt-0.5 shrink-0" />
			<span>
				Kết đạo lữ với <strong class="text-mist-100">{confirm.name}</strong> là ràng buộc <strong>vĩnh viễn</strong>. Không thể tự hủy: chỉ chấm dứt
				khi một bên vào lượng kiếp (luân hồi, Phong Thần), khi ban quản trị xử lý báo cáo quấy rối, hoặc khi một bên vắng mặt quá {rules?.absenceDays ?? 'nhiều'}
				ngày. Mỗi người chỉ có một đạo lữ.
			</span>
		</p>
		{#if confirm.mode === 'propose'}
			<p class="mt-3 text-sm text-mist-300">Đối phương sẽ thấy lời cầu thân trong {rules?.proposalExpiryHours ?? 'vài'} giờ và cũng phải xác nhận lời thề này.</p>
		{:else}
			<p class="mt-3 text-sm text-mist-300">{confirm.name} đã lập lời thề khi cầu thân. Nhận lời là lời xác nhận cuối cùng.</p>
		{/if}
		<p class="mt-2 text-xs text-mist-500">Một người dùng nhiều tài khoản để tự song tu là vi phạm; các cặp đáng ngờ được ghi nhận để ban quản trị xem xét.</p>
		<label class="mt-4 block text-sm text-mist-300">
			Gõ đúng đạo hiệu <strong class="text-mist-100">{confirm.name}</strong> để xác nhận
			<input bind:value={typed} autocomplete="off" class="mt-1 h-11 w-full rounded-[2px] border border-ink-600 bg-ink-950/60 px-3 text-mist-100 outline-none focus:border-gold-500" />
		</label>
		<div class="mt-4 grid grid-cols-2 gap-2">
			<button class="btn btn-ghost h-11" onclick={() => (confirm = null)}>Suy nghĩ thêm</button>
			<button class="btn btn-gold" disabled={busy || typed.trim() !== confirm.name} onclick={submitConfirm}>
				{confirm.mode === 'accept' ? 'Lập lời thề' : 'Gửi lời cầu thân'}
			</button>
		</div>
	{/if}
</GameWindow>

<style>
	.bond {
		border: 1px solid color-mix(in oklab, var(--color-cinnabar-300) 30%, transparent);
		background:
			radial-gradient(ellipse at 50% 40%, color-mix(in oklab, var(--color-cinnabar-300) 16%, transparent), transparent 60%),
			radial-gradient(ellipse at 50% 100%, color-mix(in oklab, var(--color-gold-300) 14%, transparent), transparent 60%),
			color-mix(in oklab, var(--color-ink-950) 45%, transparent);
	}
	.ring {
		border-radius: 9999px;
		box-shadow:
			0 0 0 2px color-mix(in oklab, var(--color-gold-300) 60%, transparent),
			0 0 30px -4px color-mix(in oklab, var(--color-cinnabar-300) 60%, transparent);
	}
	.ring.empty {
		border: 1px dashed var(--color-mist-500);
		box-shadow: none;
	}
	.dot {
		position: absolute;
		right: 6px;
		bottom: 6px;
		width: 14px;
		height: 14px;
		border-radius: 9999px;
		border: 2px solid var(--color-ink-950);
		background: var(--color-mist-500);
	}
	.dot.static {
		position: static;
		display: inline-block;
		width: 10px;
		height: 10px;
		border-width: 0;
	}
	.dot.on {
		background: var(--color-jade-300);
		box-shadow: 0 0 8px var(--color-jade-300);
	}
	.link {
		height: 2px;
		margin: 0 0.5rem 3rem;
		background: repeating-linear-gradient(90deg, var(--color-mist-500) 0 6px, transparent 6px 12px);
		opacity: 0.5;
	}
	.link.is-pending {
		background: repeating-linear-gradient(90deg, var(--color-gold-300) 0 6px, transparent 6px 12px);
		opacity: 0.8;
	}
	.link.is-on {
		background: linear-gradient(90deg, var(--color-gold-300), var(--color-cinnabar-300), var(--color-gold-300));
		box-shadow: 0 0 12px var(--color-cinnabar-300);
		opacity: 1;
		animation: var(--animate-qi-breathe);
	}
	/* song tu: the link becomes a red thread with a glint running along it */
	.link.is-thread {
		position: relative;
		overflow: hidden;
		height: 3px;
		background: linear-gradient(90deg, var(--color-cinnabar-500), var(--color-cinnabar-300), var(--color-cinnabar-500));
		box-shadow: 0 0 10px var(--color-cinnabar-300);
	}
	.spark {
		position: absolute;
		inset: 0;
		width: 30%;
		background: linear-gradient(90deg, transparent, var(--color-gold-300), transparent);
		transform: translateX(-100%);
	}
	:global(:root[data-fx='full']) .spark {
		animation: thread 2.2s var(--ease-in-out-sine) infinite;
	}
	:global(:root[data-fx='lite']) .spark {
		animation: thread 4.4s var(--ease-in-out-sine) infinite;
	}
	@keyframes thread {
		to {
			transform: translateX(340%);
		}
	}
	/* both online: a light heartbeat on the two portraits (lub-dub, then rest) */
	:global(:root[data-fx='full']) .ring.is-beat {
		animation: beat 1.6s var(--ease-qi) infinite;
	}
	@keyframes beat {
		0%,
		40%,
		100% {
			transform: scale(1);
		}
		10% {
			transform: scale(1.045);
		}
		20% {
			transform: scale(1);
		}
		30% {
			transform: scale(1.03);
		}
	}
	.meter {
		position: relative;
		height: 10px;
		border: 1px solid color-mix(in oklab, var(--color-gold-300) 30%, transparent);
		background: color-mix(in oklab, var(--color-ink-950) 70%, transparent);
	}
	.meter .fill {
		position: absolute;
		inset: 0 auto 0 0;
		background: linear-gradient(90deg, var(--color-cinnabar-300), var(--color-gold-300));
		opacity: 0.55;
		transition: width 0.6s ease;
	}
	.meter .paid {
		position: absolute;
		inset: 0 auto 0 0;
		background: var(--color-jade-300);
		opacity: 0.85;
		transition: width 0.6s ease;
	}
	.petals span {
		position: absolute;
		top: -10px;
		width: 8px;
		height: 6px;
		border-radius: 60% 0;
		background: var(--color-cinnabar-300);
		opacity: 0.55;
		animation: fall 9s linear infinite;
	}
	@keyframes fall {
		to {
			transform: translate(40px, 420px) rotate(320deg);
			opacity: 0;
		}
	}
	@media (prefers-reduced-motion: reduce) {
		.petals,
		.link.is-on {
			animation: none;
		}
		.petals span {
			animation: none;
			display: none;
		}
	}
</style>
