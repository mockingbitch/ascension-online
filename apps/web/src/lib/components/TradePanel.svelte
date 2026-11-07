<!--
	Trao đổi (GET/POST /trades, Phase 5.2): incoming offers inbox with a confirm screen ("không hoàn tác"), my pending
	offers, composing a gift/swap to a player picked from a player card (chat name, Thiên Bảng, faction member list),
	and history. Tradable assets, values, fee, caps and gates come from the server (trade.yaml); the fee and value shown
	while composing are an estimate — the server prices and re-checks everything at accept.
-->
<script lang="ts">
	import { api, ApiError, newIdempotencyKey } from '$lib/api/client';
	import { game } from '$lib/game/store.svelte';
	import { toast } from '$lib/game/toasts.svelte';
	import type { TradeBlocker, TradeLine, TradeLineInput, TradeOffer, TradePeer, TradeRelation, TradeView } from '$lib/game/types';
	import GameWindow from './GameWindow.svelte';
	import Icon from './Icon.svelte';

	/** A recipient picked from a player card (/play/tui-do?tab=trao-doi&den=…&ten=…&canh-gioi=…). */
	let { target = null }: { target?: { characterId: string; name: string; stageId: string } | null } = $props();

	type Sub = 'inbox' | 'compose' | 'history';
	let sub = $state<Sub>('inbox');
	let view = $state<TradeView | null>(null);
	let failed = $state(false);
	let history = $state<TradeOffer[] | null>(null);
	let busy = $state(false);

	const load = () =>
		api
			.trades()
			.then((v) => (view = v))
			.catch(() => (failed = true));
	$effect(() => {
		void load();
	});

	const relName: Record<TradeRelation, string> = { stranger: 'Người lạ', faction: 'Đồng môn', dao_lu: 'Đạo lữ' };
	const statusName: Record<string, string> = {
		pending: 'Đang chờ',
		accepted: 'Đã hoàn tất',
		declined: 'Bị từ chối',
		cancelled: 'Đã thu hồi',
		expired: 'Hết hạn'
	};
	const n = (v: number) => v.toLocaleString('vi-VN');
	const assetName = (id: string) => (id === 'spirit_stone' ? 'Linh thạch' : (game.catalogue?.itemName(id) ?? id));
	const lineText = (ls: TradeLine[] | TradeLineInput[]) => ls.map((l) => `${assetName(l.assetId)} ×${n(l.quantity)}`).join(', ');
	const tierName = (tier: number) => [...(game.catalogue?.realms.values() ?? [])].find((r) => r.tier === tier)?.name ?? `bậc ${tier}`;
	const blockerText = (b: TradeBlocker, gate: TradeView['rules']['receive']) =>
		b === 'account_age'
			? `tài khoản chưa đủ ${gate.minAccountAgeDays} ngày`
			: b === 'character_age'
				? `nhân vật chưa đủ ${gate.minCharacterAgeDays} ngày`
				: `chưa đạt ${tierName(gate.minTier)}`;
	const capFor = (r: TradeRelation) =>
		!view ? 0 : r === 'dao_lu' ? view.rules.receivedValuePerDay.daoLu : r === 'faction' ? view.rules.receivedValuePerDay.faction : view.rules.receivedValuePerDay.stranger;
	const timeLeft = (iso: string) => {
		const ms = new Date(iso).getTime() - Date.now();
		if (ms <= 0) return 'đã hết hạn';
		const h = Math.floor(ms / 3_600_000);
		return h >= 1 ? `còn ${h} canh giờ` : `còn ${Math.max(1, Math.round(ms / 60_000))} phút`;
	};
	const errMsg = (e: unknown) => (e instanceof ApiError ? e.message : 'Không kết nối được, thử lại sau.');

	// ───── inbox: confirm / decline / cancel ─────
	let confirming = $state<TradeOffer | null>(null);
	let understood = $state(false);
	let acceptKey: string | null = null; // kept across network errors so a retry replays the same accept

	function openConfirm(o: TradeOffer) {
		confirming = o;
		understood = false;
		acceptKey = null;
	}

	async function accept() {
		if (!confirming || !understood) return;
		busy = true;
		acceptKey ??= newIdempotencyKey();
		try {
			view = await api.tradeAccept(confirming.id, acceptKey);
			acceptKey = null;
			toast('ok', `Đã nhận ${lineText(confirming.give)} từ ${confirming.counterpart.name}.`);
			confirming = null;
			history = null;
			await game.reload();
		} catch (e) {
			if (e instanceof ApiError && e.status < 500) acceptKey = null; // refused: nothing moved
			toast('err', errMsg(e));
			void load();
		} finally {
			busy = false;
		}
	}

	async function answer(o: TradeOffer, action: 'decline' | 'cancel') {
		busy = true;
		try {
			view = action === 'decline' ? await api.tradeDecline(o.id) : await api.tradeCancel(o.id);
			history = null;
			toast('ok', action === 'decline' ? 'Đã từ chối lời trao đổi.' : 'Đã thu hồi lời trao đổi.');
		} catch (e) {
			toast('err', errMsg(e));
			void load();
		} finally {
			busy = false;
		}
	}

	// ───── compose ─────
	type Row = { assetId: string; quantity: number };
	let peer = $state<TradePeer | null>(null);
	let peerErr = $state<string | null>(null);
	let mode = $state<'gift' | 'swap'>('gift');
	let give = $state<Row[]>([{ assetId: 'spirit_stone', quantity: 10 }]);
	let take = $state<Row[]>([]);
	let createKey: string | null = null;

	let handled = '';
	$effect(() => {
		if (!target || target.characterId === handled) return;
		handled = target.characterId;
		sub = 'compose';
		peer = null;
		peerErr = null;
		api
			.tradePeer(target.characterId)
			.then((p) => (peer = p))
			.catch((e) => (peerErr = errMsg(e)));
	});

	const owned = (id: string) =>
		id === 'spirit_stone' ? game.stones : (game.inventory?.items.find((i) => i.itemId === id)?.quantity ?? 0);
	const assets = $derived(view?.rules.assets ?? []);
	const valueOf = (id: string) => assets.find((a) => a.id === id)?.value ?? 0;
	const sumValue = (rows: Row[]) => rows.reduce((s, r) => s + valueOf(r.assetId) * Math.max(0, r.quantity || 0), 0);
	const stonesIn = (rows: Row[]) => rows.filter((r) => r.assetId === 'spirit_stone').reduce((s, r) => s + Math.max(0, r.quantity || 0), 0);
	/** Display estimate of the burned fee (server-authoritative at accept). */
	const feeOf = (stones: number) => {
		if (!view || stones <= 0) return 0;
		return Math.max(Math.ceil((stones * view.rules.stoneFeeBp) / 10_000), view.rules.minStoneFee);
	};
	const giveValue = $derived(sumValue(give));
	const takeValue = $derived(mode === 'swap' ? sumValue(take) : 0);
	const myFee = $derived(feeOf(stonesIn(give)));
	const cap = $derived(peer?.receiveCap ?? view?.rules.receivedValuePerDay.stranger ?? 0);
	const composeProblem = $derived.by(() => {
		if (!view) return 'Đang tải…';
		if (!target) return 'Chọn người nhận từ thẻ đạo hữu.';
		if (view.limits.sendBlockers.length) return `Chưa thể tặng: ${view.limits.sendBlockers.map((b) => blockerText(b, view!.rules.send)).join(', ')}.`;
		if (peer?.blockers.length) return `${peer.name} chưa thể nhận: ${peer.blockers.map((b) => blockerText(b, view!.rules.receive)).join(', ')}.`;
		if (view.limits.offersLeft <= 0) return 'Hôm nay đã gửi đủ số lời trao đổi.';
		const rows = mode === 'swap' ? [...give, ...take] : give;
		if (rows.some((r) => !r.assetId || !(r.quantity >= 1) || !Number.isInteger(r.quantity))) return 'Số lượng phải là số nguyên ≥ 1.';
		const ids = give.map((r) => r.assetId);
		if (new Set(ids).size !== ids.length) return 'Mỗi vật phẩm chỉ ghi một dòng.';
		for (const r of give) {
			const need = r.quantity + (r.assetId === 'spirit_stone' ? myFee : 0);
			if (owned(r.assetId) < need) return `Không đủ ${assetName(r.assetId)} (cần ${n(need)}${r.assetId === 'spirit_stone' ? ' kể cả phí' : ''}).`;
		}
		if (mode === 'swap') {
			if (!take.length) return 'Thêm vật phẩm muốn đổi lấy.';
			const tids = take.map((r) => r.assetId);
			if (new Set(tids).size !== tids.length || tids.some((id) => ids.includes(id))) return 'Hai bên không được trùng vật phẩm.';
			if (takeValue > cap) return `Bạn chỉ nhận được tối đa ${n(cap)} giá trị mỗi ngày từ ${relName[peer?.relation ?? 'stranger'].toLowerCase()}.`;
		}
		if (giveValue > cap) return `Vượt hạn mức nhận của đối phương (${n(cap)} giá trị/ngày).`;
		return null;
	});

	/** Adds a line with the first asset not used on either side (the server refuses the same asset on both sides). */
	function addRow(rows: Row[]) {
		const used = new Set([...give, ...(mode === 'swap' ? take : [])].map((r) => r.assetId));
		const next = assets.find((a) => !used.has(a.id));
		if (next) rows.push({ assetId: next.id, quantity: 1 });
	}

	async function send() {
		if (!target || composeProblem) return;
		busy = true;
		createKey ??= newIdempotencyKey();
		try {
			const t = mode === 'swap' ? take.map((r) => ({ ...r })) : [];
			view = await api.tradeCreate(target.characterId, give.map((r) => ({ ...r })), t, createKey);
			createKey = null;
			toast('ok', `Đã gửi lời ${mode === 'swap' ? 'trao đổi' : 'tặng'} đến ${target.name}. Chờ đối phương xác nhận.`);
			sub = 'inbox';
		} catch (e) {
			if (e instanceof ApiError && e.status < 500) createKey = null;
			toast('err', errMsg(e));
		} finally {
			busy = false;
		}
	}

	// ───── history + report ─────
	$effect(() => {
		if (sub === 'history' && history === null) {
			api
				.tradeHistory()
				.then((h) => (history = h.offers))
				.catch(() => (history = []));
		}
	});
	let reporting = $state<TradeOffer | null>(null);
	let reportNote = $state('');
	let reportKey = '';
	function openReport(o: TradeOffer) {
		reporting = o;
		reportNote = '';
		reportKey = newIdempotencyKey();
	}
	async function report() {
		if (!reporting) return;
		busy = true;
		try {
			await api.reportTrade(reporting.id, reportNote.trim(), reportKey);
			toast('ok', 'Đã ghi nhận báo cáo. Ban quản trị sẽ xem xét giao dịch này.');
			reporting = null;
		} catch (e) {
			toast('err', errMsg(e));
		} finally {
			busy = false;
		}
	}
</script>

{#snippet lines(ls: TradeLine[], empty: string)}
	{#if ls.length}
		<ul class="space-y-0.5">
			{#each ls as l (l.assetId)}
				<li class="flex items-center gap-1.5">
					<Icon name={l.assetId === 'spirit_stone' ? 'stone' : 'lotus'} size={14} />
					<span class="text-mist-100">{assetName(l.assetId)}</span>
					<span class="tabular-nums text-gold-300">×{n(l.quantity)}</span>
				</li>
			{/each}
		</ul>
	{:else}
		<p class="text-mist-500">{empty}</p>
	{/if}
{/snippet}

{#snippet offerCard(o: TradeOffer)}
	{@const inc = o.direction === 'incoming'}
	<li class="offer">
		<div class="flex items-start justify-between gap-3">
			<div class="min-w-0">
				<p class="truncate font-medium text-mist-100">
					<Icon name={o.kind === 'gift' ? 'gift' : 'arrow'} size={15} class="mr-1 inline text-gold-300" />
					{inc ? 'Từ' : 'Đến'} {o.counterpart.name}
				</p>
				<p class="text-xs text-mist-500">
					{o.kind === 'gift' ? 'Tặng' : 'Đổi'} · {relName[o.relation]} · {o.status === 'pending' ? timeLeft(o.expiresAt) : statusName[o.status]}
				</p>
			</div>
			{#if o.status !== 'pending'}<span class="chip" data-s={o.status}>{statusName[o.status]}</span>{/if}
		</div>
		<div class="mt-2 grid gap-2 text-sm sm:grid-cols-2">
			<div>
				<p class="eyebrow mb-1">{inc ? 'Bạn nhận' : 'Bạn tặng'}</p>
				{@render lines(o.give, '—')}
			</div>
			{#if o.kind === 'swap'}
				<div>
					<p class="eyebrow mb-1">{inc ? 'Bạn đưa' : 'Bạn nhận lại'}</p>
					{@render lines(o.take, '—')}
				</div>
			{/if}
		</div>
		{#if o.status === 'pending'}
			<div class="mt-3 flex gap-2">
				{#if inc}
					<button class="btn btn-jade h-10 flex-1 text-sm" disabled={busy} onclick={() => openConfirm(o)}><Icon name="check" size={15} /> Xem & nhận</button>
					<button class="btn btn-ghost h-10 px-4 text-sm" disabled={busy} onclick={() => answer(o, 'decline')}>Từ chối</button>
				{:else}
					<p class="flex-1 self-center text-xs text-mist-500">Phí dự kiến khi hoàn tất: {n(o.senderFee)} linh thạch</p>
					<button class="btn btn-ghost h-10 px-4 text-sm" disabled={busy} onclick={() => answer(o, 'cancel')}>Thu hồi</button>
				{/if}
			</div>
		{:else if o.status === 'accepted' || o.status === 'declined'}
			<div class="mt-2 flex items-center justify-between gap-2 text-xs text-mist-500">
				<span>
					{#if o.status === 'accepted'}Phí đã hủy: {n(inc ? o.receiverFee : o.senderFee)} linh thạch{/if}
				</span>
				<button class="inline-flex h-9 items-center gap-1 px-2 text-mist-300 hover:text-mist-100" onclick={() => openReport(o)}>
					<Icon name="flag" size={13} /> Báo cáo
				</button>
			</div>
		{/if}
	</li>
{/snippet}

{#if failed}
	<p class="py-10 text-center text-mist-500">Chưa mở được Trao Đổi — thử lại sau.</p>
{:else if !view}
	<p class="text-mist-500">Đang mở Trao Đổi…</p>
{:else}
	{@const v = view}
	<div class="limits mb-3 text-sm">
		<span>Lời gửi còn <strong>{v.limits.offersLeft}/{v.rules.offersPerDay}</strong></span>
		<span>Giao dịch còn <strong>{v.limits.completedLeft}/{v.rules.completedPerDay}</strong></span>
		<span>Đã nhận <strong>{n(v.limits.valueReceived)}</strong> giá trị · đã tặng <strong>{n(v.limits.valueSent)}/{n(v.rules.sentValuePerDay)}</strong></span>
	</div>
	<p class="mb-3 text-xs leading-relaxed text-mist-500">
		Chỉ trao đổi linh thạch và nguyên liệu. Hạn mức nhận mỗi ngày: người lạ {n(v.rules.receivedValuePerDay.stranger)} · đồng môn
		{n(v.rules.receivedValuePerDay.faction)} · đạo lữ {n(v.rules.receivedValuePerDay.daoLu)} giá trị. Phí {(v.rules.stoneFeeBp / 100).toLocaleString('vi-VN')}% linh
		thạch người đưa trả thêm và bị hủy. Lời trao đổi hết hạn sau {v.rules.expiryHours} canh giờ.
	</p>
	{#if v.limits.receiveBlockers.length}
		<p class="warn mb-3 text-sm">
			<Icon name="lock" size={15} class="mr-1 inline" /> Chưa thể nhận đồ: {v.limits.receiveBlockers.map((b) => blockerText(b, v.rules.receive)).join(', ')}.
		</p>
	{/if}

	<div class="mb-3 flex gap-1" role="tablist" aria-label="Trao đổi">
		<button role="tab" aria-selected={sub === 'inbox'} class="sub" class:is-on={sub === 'inbox'} onclick={() => (sub = 'inbox')}>
			Hộp thư{#if v.incoming.length}<span class="badge">{v.incoming.length}</span>{/if}
		</button>
		<button role="tab" aria-selected={sub === 'compose'} class="sub" class:is-on={sub === 'compose'} onclick={() => (sub = 'compose')}>Gửi tặng / đổi</button>
		<button role="tab" aria-selected={sub === 'history'} class="sub" class:is-on={sub === 'history'} onclick={() => (sub = 'history')}>Lịch sử</button>
	</div>

	{#if sub === 'inbox'}
		<h3 class="eyebrow mb-2">Gửi đến bạn</h3>
		{#if v.incoming.length}
			<ul class="space-y-2">{#each v.incoming as o (o.id)}{@render offerCard(o)}{/each}</ul>
		{:else}
			<p class="mb-2 text-sm text-mist-500">Chưa có lời trao đổi nào.</p>
		{/if}
		<h3 class="eyebrow mt-5 mb-2">Bạn đã gửi ({v.limits.openOutgoing}/{v.rules.maxOpenOutgoing})</h3>
		{#if v.outgoing.length}
			<ul class="space-y-2">{#each v.outgoing as o (o.id)}{@render offerCard(o)}{/each}</ul>
		{:else}
			<p class="text-sm text-mist-500">Không có lời nào đang chờ.</p>
		{/if}
	{:else if sub === 'compose'}
		{#if !target}
			<p class="py-6 text-center text-sm leading-relaxed text-mist-500">
				Chọn người nhận: bấm vào tên đạo hữu trong kênh chat, Thiên Bảng hoặc danh sách thành viên thế lực, rồi chọn
				<strong class="text-mist-300">Trao đổi</strong>.
			</p>
		{:else}
			<div class="offer">
				<p class="font-medium text-mist-100">Đến <span class="text-gold-300">{target.name}</span></p>
				<p class="text-xs text-mist-500">
					{game.catalogue?.stageLabel(peer?.stageId ?? target.stageId)}
					{#if peer} · {relName[peer.relation]} · nhận tối đa {n(peer.receiveCap)} giá trị/ngày{/if}
				</p>
				{#if peerErr}<p class="mt-1 text-sm text-cinnabar-300">{peerErr}</p>{/if}

				<div class="mt-3 flex gap-1" role="radiogroup" aria-label="Kiểu">
					<button role="radio" aria-checked={mode === 'gift'} class="sub" class:is-on={mode === 'gift'} onclick={() => (mode = 'gift')}>
						<Icon name="gift" size={14} /> Tặng
					</button>
					<button role="radio" aria-checked={mode === 'swap'} class="sub" class:is-on={mode === 'swap'} onclick={() => (mode = 'swap')}>
						<Icon name="arrow" size={14} /> Đổi
					</button>
				</div>

				{#snippet editor(rows: Row[], label: string, mine: boolean)}
					<p class="eyebrow mt-4 mb-1">{label}</p>
					{#each rows as r, i (i)}
						<div class="mb-2 flex items-center gap-2">
							<select class="field min-w-0 flex-1" bind:value={r.assetId} aria-label="Vật phẩm">
								{#each assets as a (a.id)}
									<option value={a.id}>{assetName(a.id)}{mine ? ` (có ${n(owned(a.id))})` : ''}</option>
								{/each}
							</select>
							<input class="field w-24 text-right tabular-nums" type="number" min="1" max={view?.rules.maxQuantityPerLine} step="1"
								inputmode="numeric" bind:value={r.quantity} aria-label="Số lượng" />
							<button class="btn btn-ghost size-10 shrink-0" aria-label="Bỏ dòng" disabled={mine && rows.length <= 1}
								onclick={() => rows.splice(i, 1)}><Icon name="cross" size={14} /></button>
						</div>
					{/each}
					{#if rows.length < (view?.rules.maxLinesPerSide ?? 0) && give.length + (mode === 'swap' ? take.length : 0) < assets.length}
						<button class="text-sm text-jade-300 hover:text-mist-100" onclick={() => addRow(rows)}>+ Thêm dòng</button>
					{/if}
				{/snippet}

				{@render editor(give, mode === 'swap' ? 'Bạn đưa' : 'Bạn tặng', true)}
				{#if mode === 'swap'}
					{@render editor(take, `Đổi lấy từ ${target.name}`, false)}
				{/if}

				<dl class="mt-4 space-y-1 text-sm">
					<div class="flex justify-between"><dt class="text-mist-300">Giá trị đối phương nhận</dt><dd class="tabular-nums">{n(giveValue)} / {n(cap)}</dd></div>
					{#if mode === 'swap'}
						<div class="flex justify-between"><dt class="text-mist-300">Giá trị bạn nhận</dt><dd class="tabular-nums">{n(takeValue)}</dd></div>
					{/if}
					<div class="flex justify-between"><dt class="text-mist-300">Phí bạn trả (ước tính, bị hủy)</dt><dd class="tabular-nums text-gold-300">{n(myFee)} linh thạch</dd></div>
				</dl>
				{#if composeProblem}<p class="mt-3 text-sm text-mist-500">{composeProblem}</p>{/if}
				<button class="btn btn-gold mt-4 w-full" disabled={busy || !!composeProblem} onclick={send}>
					<Icon name={mode === 'swap' ? 'arrow' : 'gift'} size={16} /> Gửi lời {mode === 'swap' ? 'trao đổi' : 'tặng'}
				</button>
				<p class="mt-2 text-xs text-mist-500">
					Vật phẩm chỉ chuyển khi đối phương xác nhận; khi đó máy chủ kiểm tra lại số dư, hạn mức và phí. Lời gửi tính vào giới hạn ngày ngay khi gửi.
				</p>
			</div>
		{/if}
	{:else}
		{#if history === null}
			<p class="text-mist-500">Đang tải lịch sử…</p>
		{:else if history.length}
			<ul class="space-y-2">{#each history as o (o.id)}{@render offerCard(o)}{/each}</ul>
		{:else}
			<p class="text-sm text-mist-500">Chưa có giao dịch nào.</p>
		{/if}
	{/if}
{/if}

<GameWindow open={!!confirming} title="Xác nhận trao đổi" onclose={() => (confirming = null)}>
	{#if confirming}
		{@const o = confirming}
		<p class="text-sm text-mist-300">
			{o.kind === 'gift' ? 'Quà tặng' : 'Lời trao đổi'} từ <strong class="text-mist-100">{o.counterpart.name}</strong> · {relName[o.relation]}
		</p>
		<div class="mt-4 grid grid-cols-2 gap-3 text-sm">
			<div class="side">
				<p class="eyebrow mb-1.5">Bạn nhận</p>
				{@render lines(o.give, '—')}
			</div>
			<div class="side">
				<p class="eyebrow mb-1.5">Bạn đưa</p>
				{@render lines(o.take, 'Không có')}
			</div>
		</div>
		<dl class="mt-4 space-y-1 text-sm">
			<div class="flex justify-between gap-3"><dt class="text-mist-300">Phí bạn trả (bị hủy)</dt><dd class="tabular-nums text-gold-300">{n(o.receiverFee)} linh thạch</dd></div>
			<div class="flex justify-between gap-3"><dt class="text-mist-300">Phí người gửi trả</dt><dd class="tabular-nums">{n(o.senderFee)} linh thạch</dd></div>
			<div class="flex justify-between gap-3">
				<dt class="text-mist-300">Đã nhận hôm nay (sau giao dịch)</dt>
				<dd class="tabular-nums">{n((view?.limits.valueReceived ?? 0) + o.valueToReceiver)} / {n(capFor(o.relation))} giá trị</dd>
			</div>
		</dl>
		<div class="warn mt-4 text-sm">
			<p class="font-semibold text-cinnabar-300">Không hoàn tác</p>
			<p class="mt-1 leading-relaxed">Xác nhận là vật phẩm chuyển ngay giữa hai bên. Không thể đòi lại. Đừng trao đổi để mua bán bằng tiền thật ngoài game.</p>
			<label class="mt-3 flex cursor-pointer items-start gap-2">
				<input type="checkbox" class="mt-1 accent-gold-500" bind:checked={understood} />
				<span>Tôi đã xem kỹ hai bên và hiểu giao dịch không hoàn tác.</span>
			</label>
		</div>
		<button class="btn btn-jade mt-4 w-full" disabled={busy || !understood} onclick={accept}><Icon name="check" size={16} /> Xác nhận nhận</button>
		<button class="mt-2 inline-flex h-10 w-full items-center justify-center gap-1 text-sm text-mist-300 hover:text-mist-100"
			onclick={() => { const x = confirming; confirming = null; if (x) openReport(x); }}>
			<Icon name="flag" size={14} /> Báo cáo lời trao đổi này
		</button>
	{/if}
</GameWindow>

<GameWindow open={!!reporting} title="Báo cáo giao dịch" onclose={() => (reporting = null)}>
	{#if reporting}
		<p class="text-sm text-mist-300">Báo cáo <strong class="text-mist-100">{reporting.counterpart.name}</strong> về giao dịch này (ví dụ: mua bán tiền thật, lừa đảo).</p>
		<textarea class="field mt-3 h-24 w-full" maxlength="500" placeholder="Ghi chú (không bắt buộc)" bind:value={reportNote}></textarea>
		<button class="btn btn-gold mt-3 w-full" disabled={busy} onclick={report}><Icon name="flag" size={16} /> Gửi báo cáo</button>
	{/if}
</GameWindow>

<style>
	.limits {
		display: flex;
		flex-wrap: wrap;
		gap: 0.25rem 1rem;
		color: var(--color-mist-300);
	}
	.limits strong {
		color: var(--color-mist-100);
		font-variant-numeric: tabular-nums;
	}
	.offer,
	.side {
		padding: 0.75rem;
		border: 1px solid color-mix(in oklab, var(--color-gold-500) 20%, transparent);
		background: color-mix(in oklab, var(--color-ink-950) 60%, transparent);
	}
	.warn {
		padding: 0.75rem;
		color: var(--color-mist-100);
		border: 1px solid color-mix(in oklab, var(--color-cinnabar-500) 45%, transparent);
		background: color-mix(in oklab, var(--color-cinnabar-500) 10%, transparent);
	}
	.sub {
		display: inline-flex;
		align-items: center;
		gap: 0.35rem;
		height: 2.25rem;
		padding: 0 0.8rem;
		font-size: 0.8125rem;
		color: var(--color-mist-300);
		border: 1px solid color-mix(in oklab, var(--color-gold-500) 20%, transparent);
	}
	.sub.is-on {
		color: var(--color-gold-300);
		border-color: var(--color-gold-500);
	}
	.badge {
		min-width: 1.1rem;
		padding: 0 0.3rem;
		font-size: 0.7rem;
		font-weight: 700;
		color: var(--color-ink-950);
		background: var(--color-gold-300);
		border-radius: 999px;
	}
	.chip {
		flex-shrink: 0;
		padding: 0.1rem 0.45rem;
		font-size: 0.7rem;
		color: var(--color-mist-300);
		border: 1px solid color-mix(in oklab, var(--color-mist-500) 40%, transparent);
	}
	.chip[data-s='accepted'] {
		color: var(--color-jade-300);
		border-color: color-mix(in oklab, var(--color-jade-300) 50%, transparent);
	}
	.field {
		height: 2.5rem;
		padding: 0 0.6rem;
		color: var(--color-mist-100);
		border: 1px solid var(--color-ink-600);
		background: var(--color-ink-950);
	}
	textarea.field {
		padding: 0.5rem 0.6rem;
	}
</style>
