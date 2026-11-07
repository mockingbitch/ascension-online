<!-- Nạp Tiên Duyên: package picker → order → provider checkout (new tab) → poll the order → success with the new level.
     Display only: the server prices every package and only a verified provider webhook (or reconciliation) credits.
     Returning from the checkout page never credits anything; we just keep asking the server. -->
<script lang="ts">
	import { api, ApiError, newIdempotencyKey } from '$lib/api/client';
	import type { PaymentOrder, PaymentPackagesView, PaymentStatus, VipStatus } from '$lib/game/types';
	import Icon from './Icon.svelte';

	let {
		signedIn,
		initialOrderId = null,
		hardMaxOfflineHours = null,
		onvip
	}: {
		signedIn: boolean;
		/** ?order= on the return URL: resume watching that order */
		initialOrderId?: string | null;
		/** highest tier's offline cap, for the "what it gives" copy */
		hardMaxOfflineHours?: number | null;
		onvip?: (v: VipStatus) => void;
	} = $props();

	let pkgs = $state<PaymentPackagesView | null>(null);
	let loadFailed = $state(false);
	let selected = $state<string | null>(null);
	let order = $state<PaymentOrder | null>(null);
	let after = $state<VipStatus | null>(null);
	let history = $state<PaymentOrder[]>([]);
	let busy = $state(false);
	let error = $state<string | null>(null);
	// One key per purchase attempt: a retry after a network error re-sends it (same order, never two).
	let attemptKey = $state<string | null>(null);

	const vnd = (n: number) => `${n.toLocaleString('vi-VN')}đ`;
	const open = (s: PaymentStatus) => s === 'created' || s === 'pending';
	const label: Record<PaymentStatus, string> = {
		created: 'Đang tạo đơn',
		pending: 'Chờ thanh toán',
		paid: 'Đã nhận',
		failed: 'Thất bại',
		expired: 'Hết hạn',
		refunded: 'Đã hoàn tiền'
	};
	const when = (iso: string) => new Date(iso).toLocaleString('vi-VN', { dateStyle: 'short', timeStyle: 'short' });
	const closed = $derived(!!pkgs && pkgs.providers.length === 0);
	const pick = $derived(pkgs?.packages.find((p) => p.id === selected) ?? null);

	$effect(() => {
		api.paymentPackages().then(
			(p) => {
				pkgs = p;
				selected ??= p.packages[1]?.id ?? p.packages[0]?.id ?? null;
			},
			() => (loadFailed = true)
		);
	});

	async function loadHistory() {
		if (!signedIn) return;
		history = (await api.paymentOrders().catch(() => ({ orders: [] as PaymentOrder[] }))).orders;
	}
	$effect(() => {
		if (!signedIn) return;
		loadHistory();
		if (initialOrderId) refresh(initialOrderId);
	});

	async function refresh(id: string) {
		try {
			const res = await api.paymentOrder(id);
			order = res.order; // server state replaces local state
			if (res.order.status === 'paid' && res.vip) {
				after = res.vip;
				onvip?.(res.vip);
			}
			if (!open(res.order.status)) loadHistory();
		} catch (e) {
			if (e instanceof ApiError && e.status === 404) order = null;
		}
	}

	// Poll while the order is open and the tab is visible (no timer otherwise).
	$effect(() => {
		const o = order;
		if (!o || !open(o.status)) return;
		let stopped = false;
		let timer: ReturnType<typeof setTimeout>;
		const tick = async () => {
			if (stopped) return;
			if (document.visibilityState === 'visible') await refresh(o.id);
			if (!stopped && Date.now() < Date.parse(o.expiresAt) + 60_000) timer = setTimeout(tick, 2500);
		};
		timer = setTimeout(tick, 2500);
		const onVis = () => document.visibilityState === 'visible' && refresh(o.id);
		document.addEventListener('visibilitychange', onVis);
		return () => {
			stopped = true;
			clearTimeout(timer);
			document.removeEventListener('visibilitychange', onVis);
		};
	});

	async function buy() {
		if (!pick) return;
		busy = true;
		error = null;
		after = null;
		attemptKey ??= newIdempotencyKey();
		try {
			const res = await api.createPaymentOrder(pick.id, attemptKey);
			order = res.order;
			attemptKey = null;
			loadHistory();
		} catch (e) {
			if (e instanceof ApiError && e.status >= 400 && e.status < 500) attemptKey = null;
			error = e instanceof ApiError ? e.message : 'Không tạo được đơn, thử lại.';
		} finally {
			busy = false;
		}
	}

	function reset() {
		order = null;
		after = null;
		error = null;
	}
</script>

<section class="panel mx-auto mb-8 max-w-5xl p-4" aria-labelledby="topup-title">
	<div class="flex flex-wrap items-baseline justify-between gap-2">
		<h2 id="topup-title" class="panel-title">Nạp Tiên Duyên</h2>
		{#if pkgs}
			<p class="text-sm text-mist-300">{vnd(1000)} = <span class="text-mist-100 tabular-nums">{pkgs.pointsPer1000Vnd}</span> {pkgs.pointName}</p>
		{/if}
	</div>

	<div class="mt-3 grid gap-3 text-sm sm:grid-cols-2">
		<div class="fact is-yes">
			<p class="mb-1 flex items-center gap-1.5 font-semibold text-jade-300"><Icon name="check" size={16} /> Tiên Duyên mang lại</p>
			<ul class="list-inside list-disc space-y-0.5 text-mist-300">
				<li>Bế quan lâu hơn khi vắng mặt{hardMaxOfflineHours ? ` (tối đa ${hardMaxOfflineHours} canh giờ)` : ''}</li>
				<li>Quà linh thạch mỗi ngày từ cấp 2</li>
				<li>Danh hiệu, khung chân dung, hào quang, trang trí động phủ</li>
			</ul>
		</div>
		<div class="fact is-no">
			<p class="mb-1 flex items-center gap-1.5 font-semibold text-cinnabar-300"><Icon name="cross" size={16} /> Không bao giờ</p>
			<ul class="list-inside list-disc space-y-0.5 text-mist-300">
				<li>Tăng tốc độ tu luyện hay vượt trần tu vi mỗi ngày</li>
				<li>Tăng tỉ lệ đột phá, lôi kiếp hay điểm Thiên Bảng</li>
				<li>Bán vật phẩm tăng tỉ lệ bằng tiền</li>
			</ul>
		</div>
	</div>

	{#if loadFailed}
		<p class="mt-4 text-mist-500">Không tải được bảng giá.</p>
	{:else if !pkgs}
		<p class="mt-4 text-mist-500">Đang tải…</p>
	{:else if closed}
		<p class="mt-4 border border-gold-500/20 bg-ink-950/40 px-4 py-3 text-sm text-mist-300">
			<Icon name="lock" size={16} class="mr-1 inline align-[-3px] text-gold-500" /> Kênh nạp chưa mở. Bạn vẫn tích {pkgs.pointName} qua tu luyện, nghe giảng và nhiệm vụ ngày.
		</p>
	{:else if order}
		{@const o = order}
		<div class="mt-4 border border-gold-500/25 bg-ink-950/40 p-4" aria-live="polite">
			<div class="flex flex-wrap items-center justify-between gap-2">
				<p class="font-display text-lg text-mist-100">{o.packageName} · <span class="tabular-nums">{vnd(o.amountVnd)}</span></p>
				<span class="status" data-s={o.status}>{label[o.status]}</span>
			</div>
			<p class="mt-1 text-sm text-mist-300">
				+<span class="tabular-nums text-mist-100">{o.points.toLocaleString('vi-VN')}</span> {pkgs.pointName} · Mã đơn <span class="font-mono text-xs">{o.id.slice(0, 8)}</span>
			</p>

			{#if open(o.status)}
				{#if o.checkout}
					<p class="mt-3 text-sm text-mist-300">Thanh toán ở trang của cổng thanh toán, rồi quay lại đây. Đơn hết hạn lúc {when(o.expiresAt)}.</p>
					<div class="mt-3 flex flex-wrap items-center gap-3">
						<a class="btn btn-gold h-11 px-5" href={o.checkout.url} target="_blank" rel="noopener">
							Mở trang thanh toán <Icon name="arrow" size={16} />
						</a>
						<span class="flex items-center gap-2 text-sm text-mist-500"><span class="pulse" aria-hidden="true"></span> Đang chờ xác nhận từ cổng…</span>
					</div>
				{:else}
					<p class="mt-3 text-sm text-mist-500">Đang tạo trang thanh toán…</p>
				{/if}
			{:else if o.status === 'paid'}
				<div class="mt-3 flex items-center gap-4">
					<div class="seal grid size-16 shrink-0 place-items-center rounded-full text-gold-300"><Icon name="seal" size={26} /></div>
					<div>
						<p class="font-display text-xl text-gold-300">Duyên đã kết</p>
						{#if after}
							<p class="text-sm text-mist-300">
								Nay là <strong class="text-mist-100">{after.benefits.name}</strong> · {after.points.toLocaleString('vi-VN')} {after.pointName} · bế quan tối đa {after.benefits.offlineCapHours} canh giờ
							</p>
						{/if}
					</div>
				</div>
			{:else if o.status === 'failed'}
				<p class="mt-3 text-sm text-cinnabar-300">Thanh toán không thành công. Không có khoản nào bị trừ điểm; nếu tiền đã bị trừ, đơn sẽ được đối soát tự động.</p>
			{:else if o.status === 'expired'}
				<p class="mt-3 text-sm text-mist-300">Đơn đã hết hạn. Nếu bạn đã thanh toán, hệ thống vẫn đối soát với cổng trong 24 giờ và cộng {pkgs.pointName} khi xác nhận.</p>
			{:else if o.status === 'refunded'}
				<p class="mt-3 text-sm text-mist-300">Đơn đã được hoàn tiền; {pkgs.pointName} của đơn đã được trừ lại.</p>
			{/if}
			{#if !open(o.status)}
				<button class="btn btn-ghost mt-4 h-10 px-4 text-sm" onclick={reset}>Nạp gói khác</button>
			{/if}
		</div>
	{:else}
		<div class="mt-4 grid grid-cols-2 gap-2 sm:grid-cols-3" role="radiogroup" aria-label="Chọn gói nạp">
			{#each pkgs.packages as p (p.id)}
				<button class="pkg" class:is-on={selected === p.id} role="radio" aria-checked={selected === p.id} onclick={() => (selected = p.id)}>
					<span class="font-display text-base text-mist-100">{p.name}</span>
					<span class="text-lg font-semibold text-gold-300 tabular-nums">{vnd(p.amountVnd)}</span>
					<span class="text-xs text-mist-300 tabular-nums">+{p.points.toLocaleString('vi-VN')} {pkgs.pointName}</span>
				</button>
			{/each}
		</div>
		<div class="mt-4 flex flex-wrap items-center gap-3">
			{#if signedIn}
				<button class="btn btn-gold h-11 px-6" disabled={busy || !pick} onclick={buy}>
					{busy ? 'Đang tạo đơn…' : pick ? `Nạp ${vnd(pick.amountVnd)}` : 'Chọn gói'}
				</button>
			{:else}
				<a class="btn btn-gold h-11 px-6" href="/login">Đăng nhập để nạp</a>
			{/if}
			<span class="text-xs text-mist-500">Qua {pkgs.providers.map((p) => p.name).join(', ')}</span>
		</div>
	{/if}
	{#if error}<p class="mt-3 text-sm text-cinnabar-300" role="alert">{error}</p>{/if}

	<p class="mt-4 text-xs text-mist-500">
		<strong class="text-mist-300">Chính sách hoàn tiền (bản nháp, chờ ý kiến pháp lý):</strong> khi một giao dịch được hoàn tiền, {pkgs?.pointName ?? 'điểm duyên'}
		của giao dịch đó bị trừ đúng số đã cộng; cấp Tiên Duyên có thể giảm và các đặc quyền ngoại hình của cấp cao hơn bị gỡ. Quà ngày đã nhận
		không bị thu hồi. Liên hệ hỗ trợ kèm mã đơn.
	</p>

	{#if signedIn && history.length}
		<details class="mt-4">
			<summary class="cursor-pointer text-sm text-mist-300">Lịch sử nạp ({history.length})</summary>
			<ul class="mt-2 divide-y divide-gold-500/10 text-sm">
				{#each history as h (h.id)}
					<li class="flex flex-wrap items-center justify-between gap-2 py-2">
						<span class="text-mist-100">{h.packageName} · <span class="tabular-nums">{vnd(h.amountVnd)}</span></span>
						<span class="flex items-center gap-3 text-mist-500">
							<span class="tabular-nums">{when(h.createdAt)}</span>
							<span class="status" data-s={h.status}>{label[h.status]}</span>
						</span>
					</li>
				{/each}
			</ul>
		</details>
	{/if}
</section>

<style>
	.fact {
		border: 1px solid color-mix(in oklab, var(--color-gold-500) 14%, transparent);
		background: color-mix(in oklab, var(--color-ink-950) 55%, transparent);
		padding: 0.75rem 0.9rem;
	}
	.fact.is-yes {
		border-left: 2px solid var(--color-jade-300);
	}
	.fact.is-no {
		border-left: 2px solid var(--color-cinnabar-500);
	}
	.pkg {
		display: flex;
		flex-direction: column;
		align-items: flex-start;
		gap: 0.15rem;
		min-height: 5.5rem;
		padding: 0.75rem 0.9rem;
		text-align: left;
		border: 1px solid color-mix(in oklab, var(--color-gold-500) 20%, transparent);
		background: color-mix(in oklab, var(--color-ink-950) 50%, transparent);
		transition: border-color 0.15s, background 0.15s;
	}
	.pkg:hover {
		border-color: color-mix(in oklab, var(--color-gold-300) 45%, transparent);
	}
	.pkg.is-on {
		border-color: var(--color-gold-300);
		background: linear-gradient(180deg, color-mix(in oklab, var(--color-gold-500) 16%, transparent), transparent);
		box-shadow: inset 0 0 0 1px color-mix(in oklab, var(--color-gold-300) 40%, transparent);
	}
	.pkg:focus-visible {
		outline: 2px solid var(--color-gold-300);
		outline-offset: 2px;
	}
	.status {
		font-size: 0.75rem;
		letter-spacing: 0.04em;
		padding: 0.1rem 0.5rem;
		border: 1px solid var(--color-ink-600);
		color: var(--color-mist-300);
	}
	.status[data-s='pending'],
	.status[data-s='created'] {
		color: var(--color-gold-300);
		border-color: color-mix(in oklab, var(--color-gold-500) 45%, transparent);
	}
	.status[data-s='paid'] {
		color: var(--color-jade-300);
		border-color: color-mix(in oklab, var(--color-jade-300) 45%, transparent);
	}
	.status[data-s='failed'] {
		color: var(--color-cinnabar-300);
		border-color: color-mix(in oklab, var(--color-cinnabar-500) 55%, transparent);
	}
	.pulse {
		width: 0.5rem;
		height: 0.5rem;
		border-radius: 999px;
		background: var(--color-gold-300);
		animation: pulse 1.4s ease-in-out infinite;
	}
	@keyframes pulse {
		50% {
			opacity: 0.25;
		}
	}
	@media (prefers-reduced-motion: reduce) {
		.pulse {
			animation: none;
		}
	}
	.seal {
		background: radial-gradient(circle, color-mix(in oklab, var(--color-gold-500) 22%, var(--color-ink-800)), var(--color-ink-950));
		box-shadow:
			0 0 0 1px var(--color-gold-300),
			0 0 24px -4px color-mix(in oklab, var(--color-gold-300) 45%, transparent);
	}
</style>
