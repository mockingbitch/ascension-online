<!--
	Cửa hàng (GET /shop, POST /shop/buy): prices, today's remaining limits, buy confirmation. Prices, limits and
	availability come from the server; the confirmation total is lots × the server price, display only.
-->
<script lang="ts">
	import { fxBus } from '$lib/engine/fx/bus';
	import { api, ApiError, newIdempotencyKey } from '$lib/api/client';
	import { art } from '$lib/game/art';
	import { game } from '$lib/game/store.svelte';
	import { toast } from '$lib/game/toasts.svelte';
	import type { ShopListing, ShopLockedReason, ShopView } from '$lib/game/types';
	import GameWindow from './GameWindow.svelte';
	import Icon, { type IconName } from './Icon.svelte';

	let shop = $state<ShopView | null>(null);
	let failed = $state(false);
	let pick = $state<ShopListing | null>(null);
	let lots = $state(1);
	let buying = $state(false);
	let buyKey: string | null = null; // kept across network errors so a retry replays the same purchase

	$effect(() => {
		api
			.shop()
			.then((v) => (shop = v))
			.catch(() => (failed = true));
	});

	const locked: Record<ShopLockedReason, string> = {
		tier: 'Cần Tiên Duyên cao hơn',
		owned: 'Đã sở hữu',
		sold_out: 'Hết lượt hôm nay',
		stones: 'Chưa đủ linh thạch'
	};
	const iconOf = (l: ShopListing): IconName =>
		l.category === 'pill' ? 'pill' : l.category === 'herb' ? 'lotus' : l.itemId.endsWith('_phu') ? 'talisman' : 'spark';
	const desc = (id: string) => game.catalogue?.items.get(id)?.description ?? '';
	const maxLots = (l: ShopListing, s: ShopView) => {
		let n = s.maxLotsPerOrder;
		if (l.remainingTodayLots >= 0) n = Math.min(n, l.remainingTodayLots);
		if (l.price > 0) n = Math.min(n, Math.floor(s.balance / l.price));
		return Math.max(1, n);
	};
	const resetTime = (iso: string) => new Date(iso).toLocaleTimeString('vi-VN', { hour: '2-digit', minute: '2-digit' });

	function open(l: ShopListing) {
		pick = l;
		lots = 1;
		buyKey = null;
	}

	async function confirm() {
		if (!pick) return;
		buying = true;
		buyKey ??= newIdempotencyKey();
		try {
			const r = await api.shopBuy(pick.id, lots, buyKey);
			buyKey = null;
			shop = r.shop;
			if (!r.replayed) {
				fxBus.emit('purchase', { itemId: r.item.itemId });
				fxBus.emit('reward', { items: [{ itemId: r.item.itemId, quantity: r.item.quantity }], currencies: [], source: 'shop' });
			}
			toast('ok', `Đã mua ${pick.name} ×${r.lots * pick.lotSize} (−${r.totalPrice.toLocaleString('vi-VN')} linh thạch).`);
			pick = null;
			await game.reload();
		} catch (e) {
			if (e instanceof ApiError && e.status < 500) buyKey = null; // refused: nothing written
			toast('err', e instanceof ApiError ? e.message : String(e));
			api.shop().then((v) => (shop = v)).catch(() => {});
		} finally {
			buying = false;
		}
	}
</script>

{#if failed}
	<p class="py-10 text-center text-mist-500">Cửa hàng chưa mở được — thử lại sau.</p>
{:else if !shop}
	<p class="text-mist-500">Đang mở cửa hàng…</p>
{:else}
	{@const s = shop}
	<p class="mb-3 flex flex-wrap items-center gap-x-4 gap-y-1 text-sm text-mist-300">
		<span class="flex items-center gap-1.5"><Icon name="stone" size={16} /> <strong class="text-mist-100 tabular-nums">{s.balance.toLocaleString('vi-VN')}</strong> linh thạch</span>
		<span class="text-xs text-mist-500">Giới hạn ngày làm mới lúc {resetTime(s.resetsAt)}</span>
	</p>
	<ul class="stagger grid gap-2 sm:grid-cols-2">
		{#each s.listings as l (l.id)}
			{@const a = art(`items/${l.itemId}`)}
			<li class="row" style:--r="var(--color-rarity-{l.rarity})">
				<div class="thumb">
					{#if a}<img src={a.src} srcset={a.srcset} alt="" class="size-full object-contain" />{:else}<Icon name={iconOf(l)} size={28} />{/if}
				</div>
				<div class="min-w-0 flex-1">
					<p class="truncate font-medium text-mist-100">{l.name}{#if l.lotSize > 1}<span class="text-mist-500"> ×{l.lotSize}</span>{/if}</p>
					<p class="flex items-center gap-1 text-sm text-gold-300 tabular-nums"><Icon name="stone" size={13} />{l.price.toLocaleString('vi-VN')}</p>
					<p class="text-xs text-mist-500">
						{#if l.remainingTodayLots >= 0}Còn {l.remainingTodayLots}/{l.dailyLimitLots} lượt hôm nay{:else if l.owned && l.category === 'cosmetic'}Đã có{:else}Không giới hạn{/if}
					</p>
				</div>
				{#if l.available}
					<button class="btn btn-jade h-10 px-3 text-sm" disabled={buying || game.busy} onclick={() => open(l)}>Mua</button>
				{:else}
					<span class="max-w-24 text-right text-xs text-mist-500">{l.lockedReason ? locked[l.lockedReason] : ''}</span>
				{/if}
			</li>
		{/each}
	</ul>
	<p class="mt-3 text-xs leading-relaxed text-mist-500">
		Cửa hàng không bán vật phẩm tăng tỉ lệ đột phá hay vượt kiếp. Đan tăng tốc vẫn tính trong hạn mức tu vi mỗi ngày.
	</p>

	<GameWindow open={!!pick} title={pick ? `Mua ${pick.name}` : ''} onclose={() => (pick = null)}>
		{#if pick}
			{@const l = pick}
			{@const most = maxLots(l, s)}
			<p class="leading-relaxed text-mist-100">{desc(l.itemId)}</p>
			<div class="mt-4 flex items-center justify-between gap-3">
				<span class="text-sm text-mist-300">Số lượng{#if l.lotSize > 1} (mỗi phần {l.lotSize}){/if}</span>
				<div class="flex items-center gap-2">
					<button class="btn btn-ghost size-10" aria-label="Bớt" disabled={lots <= 1} onclick={() => (lots = Math.max(1, lots - 1))}>−</button>
					<span class="w-8 text-center text-lg font-semibold tabular-nums" aria-live="polite">{lots}</span>
					<button class="btn btn-ghost size-10" aria-label="Thêm" disabled={lots >= most} onclick={() => (lots = Math.min(most, lots + 1))}>+</button>
				</div>
			</div>
			<dl class="mt-4 space-y-1 text-sm">
				<div class="flex justify-between"><dt class="text-mist-300">Tổng</dt><dd class="font-semibold text-gold-300 tabular-nums">{(l.price * lots).toLocaleString('vi-VN')} linh thạch</dd></div>
				<div class="flex justify-between"><dt class="text-mist-300">Đang có</dt><dd class="tabular-nums">{s.balance.toLocaleString('vi-VN')} linh thạch</dd></div>
			</dl>
			<button class="btn btn-jade mt-5 w-full" disabled={buying || l.price * lots > s.balance} onclick={confirm}>
				<Icon name="check" size={16} /> Xác nhận mua
			</button>
		{/if}
	</GameWindow>
{/if}

<style>
	.row {
		display: flex;
		align-items: center;
		gap: 0.75rem;
		padding: 0.6rem 0.75rem;
		border: 1px solid color-mix(in oklab, var(--r) 35%, transparent);
		background: color-mix(in oklab, var(--color-ink-950) 60%, transparent);
	}
	.thumb {
		display: grid;
		place-items: center;
		width: 3rem;
		height: 3rem;
		flex-shrink: 0;
		color: var(--r);
		border: 1px solid color-mix(in oklab, var(--r) 55%, transparent);
		background: radial-gradient(circle at 50% 35%, color-mix(in oklab, var(--r) 18%, transparent), transparent 70%);
	}
</style>
