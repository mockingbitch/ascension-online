<!-- Thế Lực (window): my faction hall (members, ranking by merit, switch with cooldown, split choice) — or the picker when I
     have none. Everything is server data (GET /factions, /factions/me, /factions/{id}/members). -->
<script lang="ts">
	import { api } from '$lib/api/client';
	import FactionEmblem from '$lib/components/FactionEmblem.svelte';
	import FactionPicker from '$lib/components/FactionPicker.svelte';
	import FactionWorks from '$lib/components/FactionWorks.svelte';
	import Icon from '$lib/components/Icon.svelte';
	import RankMedal from '$lib/components/RankMedal.svelte';
	import { changeFaction, factionColor, factionError, factionOf, factions, loadFactionMe, loadFactions } from '$lib/game/faction.svelte';
	import { loreOf } from '$lib/game/factionLore';
	import { formatPoints } from '$lib/game/format';
	import { game } from '$lib/game/store.svelte';
	import { toast } from '$lib/game/toasts.svelte';
	import type { FactionChangeReason, FactionMemberEntry, FactionView, PathId } from '$lib/game/types';
	import { openWindow } from '$lib/game/windows';

	// Fresh roster/ranking and membership each time the window opens (shared with the HUD card; server numbers).
	$effect(() => void Promise.all([loadFactions(), loadFactionMe()]));

	const cat = $derived(game.catalogue!);
	const me = $derived(factions.me);
	const list = $derived(factions.list);
	const f = $derived(factionOf(me?.factionId));
	const lore = $derived(f ? loreOf(f.id) : null);
	const path = $derived<PathId>(me?.path ?? cat.stages.get(game.character?.stageId ?? '')?.path ?? 'tien');
	const pathName = (p: PathId) => cat.paths.get(p)?.name ?? p;
	const name = (id: string | null | undefined) => (id ? (factionOf(id)?.name ?? id) : '—');
	const when = (s: string | null | undefined) => (s ? new Date(s).toLocaleString('vi-VN', { dateStyle: 'short', timeStyle: 'short' }) : '');

	// Clock for countdowns only (display); refreshed every minute.
	let now = $state(Date.now());
	$effect(() => {
		const t = setInterval(() => (now = Date.now()), 60_000);
		return () => clearInterval(t);
	});
	function left(s: string | null | undefined) {
		if (!s) return '';
		const ms = Math.max(0, Date.parse(s) - now);
		const d = Math.floor(ms / 86_400_000);
		const h = Math.floor((ms % 86_400_000) / 3_600_000);
		const m = Math.floor((ms % 3_600_000) / 60_000);
		return d > 0 ? `${d} ngày ${h} giờ` : h > 0 ? `${h} giờ ${m} phút` : `${Math.max(1, m)} phút`;
	}
	const canSwitch = $derived(!!me?.factionId && !me.locked && (!me.canSwitchAt || Date.parse(me.canSwitchAt) <= now));
	const cooldownDays = $derived(me ? Math.round(me.switchCooldownSeconds / 86_400) : 0);
	const lockLabels: Record<string, string> = { fengshen: 'Phong Thần lượng kiếp', admin: 'quản trị', event: 'sự kiện' };
	const reasons: Record<FactionChangeReason, string> = {
		join: 'Gia nhập',
		switch: 'Đổi phe',
		split_choice: 'Chọn nhánh',
		split_default: 'Xếp nhánh (mặc định)',
		auto_migrate: 'Thế lực đổi tên',
		reincarnation: 'Luân hồi',
		admin: 'Quản trị'
	};

	// Members (paged by the server cursor; later pages are appended for display).
	let members = $state<FactionMemberEntry[]>([]);
	let total = $state(0);
	let cursor = $state<string | null>(null);
	let membersErr = $state<string | null>(null);
	let loadingMembers = $state(false);
	async function loadMembers(id: string, next = '') {
		loadingMembers = true;
		try {
			const p = await api.factionMembers(id, next);
			members = next ? [...members, ...p.entries] : p.entries;
			total = p.total;
			cursor = p.nextCursor;
			membersErr = null;
		} catch (e) {
			membersErr = factionError(e);
		} finally {
			loadingMembers = false;
		}
	}
	$effect(() => {
		if (f?.id) void loadMembers(f.id);
	});

	const ranked = $derived((list?.factions ?? []).filter((x) => x.meritRank > 0).sort((a, b) => a.meritRank - b.meritRank));
	const switchTargets = $derived((list?.factions ?? []).filter((x) => x.open && x.id !== me?.factionId));
	const branches = $derived((me?.mustChoose ?? []).map((id) => factionOf(id)).filter((x): x is FactionView => !!x));
	let tab = $state<'hall' | 'works' | 'switch'>('hall');

	async function act(kind: 'join' | 'switch' | 'choose', x: FactionView) {
		const err = await changeFaction(kind, x.id);
		if (err) return toast('err', err);
		tab = 'hall';
		const verbs = { join: 'Đã gia nhập', switch: 'Đã chuyển sang', choose: 'Đã chọn' };
		toast('ok', `${verbs[kind]} ${x.name}.`);
		await game.reload(); // the path (and stage id) may have been converted
	}
	const profile = (e: FactionMemberEntry) =>
		openWindow({ kind: 'profile', characterId: e.characterId, name: e.name, stageId: e.stageId, avatarFrameId: e.avatarFrameId, titleId: e.titleId });
</script>


{#if f}
	<div class="mb-3 flex items-center gap-2.5">
		<span style:color={factionColor(f)}><FactionEmblem id={f.id} size={30} /></span>
		<div class="min-w-0">
			<p class="font-display text-2xl leading-tight font-bold whitespace-nowrap" style:color={factionColor(f)}>{f.name}</p>
			{#if f.motto}<p class="truncate text-sm text-mist-300">“{f.motto}”</p>{/if}
		</div>
	</div>
{/if}
	{#if !list || !me}
		<p class="text-mist-500">Đang mở sơn môn…</p>
	{:else if !f}
		<p class="mb-4 text-sm text-mist-300">Đạo hữu chưa gia nhập thế lực nào. Chọn một nơi để kết giao và cùng đồng môn đi qua các đại kiếp.</p>
		<FactionPicker factions={list.factions.filter((x) => x.open)} {path} {pathName} busy={factions.busy} onjoin={(x) => act('join', x)}
			note={`Gia nhập xong phải chờ ${Math.round(list.switchCooldownSeconds / 86_400)} ngày mới đổi phe được.`} />
		<div class="mt-6"><FactionWorks compact /></div>
	{:else}
		{#if branches.length}
			<section class="choose mb-5 px-4 py-4" aria-labelledby="split-title">
				<h2 id="split-title" class="panel-title">{name(me.upcoming?.from ?? me.factionId)} đã phân nhánh — chọn nhánh của bạn</h2>
				<p class="mt-1 text-sm text-mist-300">
					Hạn chọn: <strong class="text-gold-300">{when(me.chooseDeadline)}</strong> (còn {left(me.chooseDeadline)}). Ai chưa chọn sẽ
					được chia đều vào các nhánh, nhánh nào ít người từ lần phân chia này thì nhận trước. Tu vi giữ nguyên, không tính là đổi phe.
				</p>
				{#if me.locked}
					<p class="mt-2 text-sm text-cinnabar-300">
						Thế lực của đạo hữu đang bị khóa nên không tự chọn được nhánh; hết hạn sẽ được xếp như người chưa chọn.
					</p>
				{/if}
				<ul class="mt-3 grid gap-2 sm:grid-cols-3">
					{#each branches as b (b.id)}
						<li style:--fc={factionColor(b)}>
							<button class="branch flex w-full items-center gap-3 px-3 py-3 text-left" disabled={factions.busy || me.locked}
								onclick={() => act('choose', b)}>
								<span style:color="var(--fc)"><FactionEmblem id={b.id} size={44} /></span>
								<span class="min-w-0">
									<span class="block font-display text-lg font-bold" style:color="var(--fc)">{b.name}</span>
									<span class="block text-xs text-mist-300">{b.pitch ?? ''} · {b.members.toLocaleString('vi-VN')} đệ tử</span>
								</span>
							</button>
						</li>
					{/each}
				</ul>
			</section>
		{:else if me.upcoming}
			<p class="notice mb-5 flex gap-2 px-3 py-2 text-sm text-gold-300" role="status">
				<Icon name="bell" size={16} class="mt-0.5 shrink-0" />
				{#if me.upcoming.kind === 'split_faction'}
					{name(me.upcoming.from)} sẽ chia thành {(me.upcoming.into ?? []).map(name).join(', ')} vào {when(me.upcoming.dueAt)} (còn
					{left(me.upcoming.dueAt)}). Sau đó đạo hữu có tới {when(me.upcoming.deadlineAt)} để chọn nhánh.
				{:else}
					{name(me.upcoming.from)} sẽ đổi thành {name(me.upcoming.to)} vào {when(me.upcoming.dueAt)} (còn {left(me.upcoming.dueAt)}).
					Thành viên được chuyển tự động, tu vi giữ nguyên.
				{/if}
			</p>
		{/if}

		<div class="grid gap-5 lg:grid-cols-[320px_minmax(0,1fr)]" style:--fc={factionColor(f)}>
			<section class="hall flex flex-col items-center px-4 py-6 text-center">
				<span class="flag" style:color="var(--fc)" data-flag><FactionEmblem id={f.id} size={120} /></span>
				{#if lore?.leaderTitle}<p class="mt-3 text-sm text-mist-300">Chủ sự: <span class="text-mist-100">{lore.leaderTitle}</span></p>{/if}
				<dl class="mt-4 grid w-full grid-cols-3 gap-2 text-center text-sm">
					<div><dt class="eyebrow">Đệ tử</dt><dd class="font-semibold tabular-nums">{f.members.toLocaleString('vi-VN')}</dd></div>
					<div><dt class="eyebrow">Công</dt><dd class="font-semibold tabular-nums">{formatPoints(f.meritMilli)}</dd></div>
					<div><dt class="eyebrow">Hạng</dt><dd class="font-semibold text-gold-300">{f.meritRank ? `#${f.meritRank}` : '—'}</dd></div>
				</dl>
				<p class="mt-3 text-xs text-mist-500">Gia nhập {when(me.joinedAt)} · Đạo đồ {pathName(me.path)}</p>
				{#if !f.open}<p class="mt-2 text-xs text-gold-300">Thế lực đã đóng cửa thu nhận đệ tử mới.</p>{/if}
				<div class="mt-5 flex gap-1" role="tablist">
					<button role="tab" aria-selected={tab === 'hall'} class="tab" class:is-on={tab === 'hall'} onclick={() => (tab = 'hall')}>Sơn môn</button>
					<button role="tab" aria-selected={tab === 'works'} class="tab" class:is-on={tab === 'works'} onclick={() => (tab = 'works')}>Công trình</button>
					<button role="tab" aria-selected={tab === 'switch'} class="tab" class:is-on={tab === 'switch'} onclick={() => (tab = 'switch')}>Đổi phe</button>
				</div>
			</section>

			<div class="min-w-0 space-y-5">
				{#if tab === 'works'}
					<FactionWorks />
				{:else if tab === 'switch'}
					<section>
						<h2 class="panel-title">Đổi thế lực</h2>
						{#if me.locked}
							<p class="mt-2 flex gap-2 text-sm text-gold-300"><Icon name="lock" size={16} class="mt-0.5 shrink-0" />
								Đang khóa đổi phe ({lockLabels[me.lockReason ?? ''] ?? me.lockReason}){me.lockedUntil ? ` tới ${when(me.lockedUntil)}` : ''}.</p>
						{:else if !canSwitch}
							<p class="mt-2 flex gap-2 text-sm text-mist-300"><Icon name="clock" size={16} class="mt-0.5 shrink-0" />
								Có thể đổi phe từ {when(me.canSwitchAt)} (còn {left(me.canSwitchAt)}).</p>
						{:else}
							<p class="mt-2 text-sm text-mist-300">Sau khi đổi, phải chờ {cooldownDays} ngày mới đổi được lần nữa.</p>
						{/if}
						{#if canSwitch}
							<div class="mt-3">
								<FactionPicker factions={switchTargets} current={me.factionId} {path} {pathName} busy={factions.busy} verb="Chuyển sang"
									note="Đổi phe sẽ bắt đầu thời gian chờ {cooldownDays} ngày." onjoin={(x) => act('switch', x)} />
							</div>
						{/if}
					</section>
					{#if me.history.length}
						<section>
							<h2 class="panel-title">Lịch sử</h2>
							<ul class="mt-2 space-y-1.5 text-sm text-mist-300">
								{#each me.history as h (h.createdAt + h.to)}
									<li class="flex flex-wrap gap-x-2"><span class="text-mist-500 tabular-nums">{when(h.createdAt)}</span>
										<span>{reasons[h.reason]}: {h.from ? `${name(h.from)} → ` : ''}<strong class="text-mist-100">{name(h.to)}</strong>
											{#if h.fromPath && h.toPath}(đạo đồ {pathName(h.fromPath)} → {pathName(h.toPath)}){/if}</span></li>
								{/each}
							</ul>
						</section>
					{/if}
				{:else}
					<section>
						<h2 class="panel-title">Thành viên <span class="text-sm text-mist-500 tabular-nums">· {total.toLocaleString('vi-VN')}</span></h2>
						{#if membersErr}
							<p role="alert" class="mt-2 text-sm text-cinnabar-300">{membersErr}</p>
						{:else}
							<ol class="mt-2 divide-y divide-gold-500/10">
								{#each members as e (e.characterId)}
									{@const r = cat.realmOf(e.stageId)}
									<li>
										<button type="button" class="row flex w-full items-center gap-3 px-2 py-2 text-left" class:me={e.characterId === game.character?.id}
											onclick={() => profile(e)}>
											<RankMedal rank={e.rank} size={30} />
											<span class="min-w-0 flex-1 truncate font-medium text-mist-100">{e.name}</span>
											<span class="truncate text-xs text-mist-500">{r ? cat.stageLabel(e.stageId) : e.stageId}</span>
										</button>
									</li>
								{/each}
							</ol>
							{#if cursor}
								<button class="btn btn-ghost mt-2" disabled={loadingMembers} onclick={() => loadMembers(f.id, cursor ?? '')}>Xem thêm</button>
							{/if}
						{/if}
					</section>
					<section>
						<h2 class="panel-title">Bảng công thế lực</h2>
						<p class="mt-1 text-xs text-mist-500">
							Tổng tu vi đồng môn tích được trong thời kỳ hiện tại{list.meritSince ? ` (từ ${new Date(list.meritSince).toLocaleDateString('vi-VN')})` : ''}.
							Thế lực không cộng sức mạnh hay tốc độ tu luyện.
						</p>
						{#if !list.rankingAvailable}
							<p class="mt-2 text-sm text-mist-500">Bảng công tạm thời chưa cập nhật.</p>
						{/if}
						<ol class="mt-2 space-y-1.5">
							{#each ranked as x (x.id)}
								<li class="frow flex items-center gap-3 px-3 py-2" class:mine={x.id === f.id} style:--fc={factionColor(x)}>
									<RankMedal rank={x.meritRank} size={30} />
									<span class="min-w-0 flex-1">
										<span class="block truncate font-display text-lg font-bold" style:color="var(--fc)">{x.name}</span>
										<span class="block text-xs text-mist-500">{x.members.toLocaleString('vi-VN')} đệ tử</span>
									</span>
									<span class="text-right text-sm text-mist-100 tabular-nums">{formatPoints(x.meritMilli)}<span class="block text-xs text-mist-500">điểm công</span></span>
								</li>
							{/each}
						</ol>
					</section>
					{#if lore?.description}
						<section>
							<h2 class="panel-title">Đặc điểm</h2>
							<p class="mt-2 text-mist-100">{lore.description}</p>
							<ul class="mt-2 flex flex-wrap gap-2">
								{#each lore.traits as t (t)}<li class="trait">{t}</li>{/each}
							</ul>
						</section>
					{/if}
				{/if}
			</div>
		</div>
	{/if}

<style>
	.hall {
		border: 1px solid color-mix(in oklab, var(--fc) 40%, transparent);
		background:
			radial-gradient(circle at 50% 20%, color-mix(in oklab, var(--fc) 22%, transparent), transparent 60%),
			color-mix(in oklab, var(--color-ink-950) 45%, transparent);
	}
	.choose {
		border: 1px solid color-mix(in oklab, var(--color-gold-300) 50%, transparent);
		background: color-mix(in oklab, var(--color-gold-500) 8%, transparent);
	}
	.notice {
		border: 1px solid color-mix(in oklab, var(--color-gold-500) 35%, transparent);
	}
	.branch {
		min-height: 3.5rem;
		border: 1px solid color-mix(in oklab, var(--fc) 40%, transparent);
		background: color-mix(in oklab, var(--color-ink-950) 45%, transparent);
		transition: box-shadow 200ms, transform 200ms var(--ease-qi);
	}
	.branch:hover:not(:disabled) {
		transform: translateY(-2px);
		box-shadow: 0 0 20px -6px var(--fc);
	}
	.tab {
		height: 2.5rem;
		padding: 0 0.8rem;
		white-space: nowrap;
		font-size: 0.875rem;
		color: var(--color-mist-300);
		border: 1px solid color-mix(in oklab, var(--color-gold-500) 20%, transparent);
	}
	.tab.is-on {
		color: var(--color-ink-950);
		font-weight: 600;
		background: linear-gradient(180deg, var(--color-gold-300), var(--color-gold-500));
	}
	.row:hover {
		background: color-mix(in oklab, var(--color-gold-500) 8%, transparent);
	}
	.me,
	.mine {
		background: linear-gradient(90deg, color-mix(in oklab, var(--color-gold-500) 14%, transparent), transparent);
	}
	.frow {
		border: 1px solid color-mix(in oklab, var(--fc) 35%, transparent);
	}
	.trait {
		padding: 2px 10px;
		font-size: 0.8rem;
		color: var(--fc);
		border: 1px solid color-mix(in oklab, var(--fc) 45%, transparent);
		border-radius: 9999px;
	}
	/* the faction badge flutters like a banner (skew/rotate around its top edge) */
	.flag {
		display: inline-block;
		transform-origin: 50% 0;
	}
	:global(:root[data-fx='full']) .flag {
		animation: flutter 3.6s var(--ease-in-out-sine) infinite;
	}
	:global(:root[data-fx='lite']) .flag {
		animation: flutter 7s var(--ease-in-out-sine) infinite;
	}
	@keyframes flutter {
		25% {
			transform: skewY(2.5deg) rotate(-1deg);
		}
		50% {
			transform: skewY(-1.5deg) rotate(0.6deg);
		}
		75% {
			transform: skewY(1.5deg) rotate(-0.4deg);
		}
	}
</style>
