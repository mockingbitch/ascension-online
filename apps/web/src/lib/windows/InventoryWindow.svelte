<!--
	Túi Đồ (window, tabs Hành trang / Cửa hàng / Trao đổi; Trang phục lives in Nhân Vật — the bag keeps a shortcut): server inventory (GET /inventory) + item
	catalogue (names, rarity, descriptions), the shop and Trao đổi (Phase 5.2). A player card opens the trade tab with
	the composer addressed to that player (window state `to`; deep link /play/tui-do?tab=trao-doi&den=…&ten=…).
-->
<script lang="ts">
	import { goto } from '$app/navigation';
	import EmptyState from '$lib/components/EmptyState.svelte';
	import GameWindow from '$lib/components/GameWindow.svelte';
	import Icon, { type IconName } from '$lib/components/Icon.svelte';
	import ShopPanel from '$lib/components/ShopPanel.svelte';
	import TradePanel from '$lib/components/TradePanel.svelte';
	import { ApiError, api } from '$lib/api/client';
	import { art } from '$lib/game/art';
	import { formatBp, formatPoints } from '$lib/game/format';
	import { game } from '$lib/game/store.svelte';
	import { toast } from '$lib/game/toasts.svelte';
	import { openWindow } from '$lib/game/windows';
	import type { EquippedArtifact, ItemDef } from '$lib/game/types';
	import { hudFx } from '$lib/engine/fx/hud.svelte';
	import type { WindowProps } from './registry';

	let { win, tab: mode }: WindowProps = $props();
	// opening the bag clears the "new items" badge of the action bar
	$effect(() => hudFx.clearUnseen());

	type Tab = 'all' | 'stone' | 'treasure' | 'pill' | 'herb' | 'misc';
	const tabs: { id: Tab; label: string }[] = [
		{ id: 'all', label: 'Tất cả' },
		{ id: 'stone', label: 'Linh thạch' },
		{ id: 'treasure', label: 'Pháp bảo' },
		{ id: 'pill', label: 'Đan dược' },
		{ id: 'herb', label: 'Linh dược' },
		{ id: 'misc', label: 'Vật phẩm' }
	];
	const rarityName: Record<string, string> = { pham: 'Phàm', linh: 'Linh', huyen: 'Huyền', dia: 'Địa', thien: 'Thiên', tien: 'Tiên' };
	const tabOf = (d: ItemDef): Tab =>
		d.category === 'pill' ? 'pill' : d.category === 'herb' ? 'herb' : d.category === 'artifact' || d.category === 'treasure' ? 'treasure' : 'misc';
	const iconOf = (d: ItemDef): IconName => (d.category === 'pill' ? 'pill' : d.category === 'herb' ? 'lotus' : d.id.endsWith('_phu') ? 'talisman' : 'spark');

	// Raster icons: src/lib/assets/art/items/<item_id>.webp (linh_thach for the currency); SVG icon otherwise.
	const stoneArt = art('items/linh_thach');
	const itemArt = (id: string) => art(`items/${id}`);
	// The window tab (Hành trang / Cửa hàng / Trao đổi) comes from the frame; a player card sets `to`.
	const tradeTarget = $derived(win.kind === 'inventory' && win.to && win.to.characterId !== game.character?.id ? win.to : null);
	let tab = $state<Tab>('all');
	let selected = $state<ItemDef | null>(null);
	const selArt = $derived(selected ? itemArt(selected.id) : undefined);

	const cat = $derived(game.catalogue!);
	const owned = $derived(
		(game.inventory?.items ?? [])
			.filter((i) => i.quantity > 0)
			.map((i) => ({ def: cat.items.get(i.itemId), qty: i.quantity, art: itemArt(i.itemId) }))
			.filter((x): x is { def: ItemDef; qty: number; art: ReturnType<typeof itemArt> } => !!x.def)
	);
	const shown = $derived(tab === 'all' ? owned : owned.filter((x) => tabOf(x.def) === tab));
	// Linh thạch is in the HUD: the "all" grid shows its tile only when there are some (an empty bag reads as empty)
	const showStones = $derived(tab === 'stone' || (tab === 'all' && game.stones > 0));
	const qtyOf = (id: string) => owned.find((x) => x.def.id === id)?.qty ?? 0;
	const usable = (d: ItemDef) => d.id === 'tu_linh_phu' && game.character && game.character.offlineBonusSeconds <= 0 && !game.character.bonusUsedToday;
	// Speed items: the server decides (daily uses, speed cap, 150 ceiling; all or nothing). This only greys out a pill
	// that can no longer fit today's speed-item room.
	const speed = $derived(game.character?.today.speedItems);
	const speedFull = (d: ItemDef) => !!speed && speed.capMilli - speed.milli < (d.instantPointsMilli ?? 0);

	// Pháp bảo slot (GET /artifacts): one equipped artifact counts toward the tribulation odds. The server's answer replaces it.
	let equipped = $state<EquippedArtifact | null>(null);
	let equipBusy = $state(false);
	$effect(() => {
		api.artifacts().then((v) => (equipped = v)).catch(() => {});
	});
	async function equip(itemId: string) {
		equipBusy = true;
		try {
			equipped = await api.equipArtifact(itemId);
			toast('ok', itemId ? 'Đã luyện hóa pháp bảo làm hộ thân khi độ kiếp.' : 'Đã thu pháp bảo vào túi.');
		} catch (e) {
			toast('err', e instanceof ApiError ? e.message : 'Không đổi được pháp bảo, thử lại sau.');
		} finally {
			equipBusy = false;
		}
	}
</script>


{#if mode === 'shop'}
		<ShopPanel />
	{:else if mode === 'trade'}
		<TradePanel target={tradeTarget} />
	{:else}
		<div class="-mx-1 mb-4 flex gap-1 overflow-x-auto pb-1" role="group" aria-label="Loại vật phẩm">
			{#each tabs as t (t.id)}
				<button type="button" aria-pressed={tab === t.id} class="tab press" class:is-on={tab === t.id} onclick={() => (tab = t.id)}>{t.label}</button>
			{/each}
			<button type="button" class="tab press ml-auto gap-1.5 text-gold-300" data-testid="bag-to-cosmetics"
				onclick={() => openWindow({ kind: 'character', tab: 'cosmetics' })}>
				<Icon name="spark" size={14} /> Trang phục
			</button>
		</div>

		{#if !game.inventory}
			<p class="text-mist-500">Đang mở càn khôn đại…</p>
		{:else}
			<ul class="stagger grid grid-cols-3 gap-3 sm:grid-cols-4 lg:grid-cols-6">
				{#if showStones}
					<li>
						<div class="cell" style:--r="var(--color-spirit-cyan)">
							{#if stoneArt}<img src={stoneArt.src} srcset={stoneArt.srcset} alt="" class="icon-art" />{:else}<Icon name="stone" size={34} />{/if}
							<span class="qty tabular-nums">{game.stones.toLocaleString('vi-VN')}</span>
							<span class="name">Linh thạch</span>
						</div>
					</li>
				{/if}
				{#each shown as x (x.def.id)}
					<li>
						<button type="button" class="cell w-full" style:--r="var(--color-rarity-{x.def.rarity})" onclick={() => (selected = x.def)}>
							{#if x.art}<img src={x.art.src} srcset={x.art.srcset} alt="" class="icon-art" />{:else}<Icon name={iconOf(x.def)} size={34} />{/if}
							<span class="qty tabular-nums">×{x.qty}</span>
							{#if equipped?.itemId === x.def.id}<span class="worn">Hộ thân</span>{/if}
							{#if !x.def.tradable}<span class="locked" title="Không thể trao đổi"><Icon name="lock" size={11} label="Không thể trao đổi" /></span>{/if}
							<span class="name">{x.def.name}</span>
						</button>
					</li>
				{/each}
			</ul>
			{#if !shown.length && tab !== 'stone'}
				<EmptyState
					icon={tab === 'pill' ? 'pill' : tab === 'herb' ? 'lotus' : 'bag'}
					title={tab === 'all' ? 'Càn khôn đại còn trống' : 'Chưa có vật phẩm loại này'}
					hint={tab === 'treasure' || tab === 'all' ? 'Vào bí cảnh để kiếm pháp bảo.' : 'Hoàn thành nhiệm vụ ngày hoặc ghé Cửa hàng.'}
					action={tab === 'treasure' || tab === 'all'
						? { label: 'Vào Bí Cảnh', icon: 'portal', onclick: () => void goto('/play/bi-canh') }
						: { label: 'Nhiệm vụ ngày', icon: 'check', onclick: () => openWindow({ kind: 'tasks' }) }}
				/>
			{/if}
		{/if}
	{/if}

<GameWindow local open={!!selected} title={selected?.name ?? ''} icon={selected ? iconOf(selected) : undefined} onclose={() => (selected = null)}>
	{#if selected}
		{@const d = selected}
		<div class="flex items-start gap-4">
			<div class="cell size-24 shrink-0" style:--r="var(--color-rarity-{d.rarity})">
				{#if selArt}<img src={selArt.src} srcset={selArt.srcset} alt="" class="icon-art" />{:else}<Icon name={iconOf(d)} size={40} />{/if}
			</div>
			<div class="min-w-0">
				<p class="font-semibold" style:color="var(--color-rarity-{d.rarity})">Phẩm {rarityName[d.rarity] ?? d.rarity}</p>
				<p class="mt-1 text-sm text-mist-300">Đang có: <strong class="text-mist-100 tabular-nums">{qtyOf(d.id)}</strong></p>
				<p class="mt-1 flex items-center gap-1 text-xs text-mist-500">
					{#if d.tradable}<Icon name="gift" size={12} /> Có thể tặng / trao đổi{:else}<Icon name="lock" size={12} /> Không thể trao đổi{/if}
				</p>
			</div>
		</div>
		<p class="mt-4 leading-relaxed text-mist-100">{d.description}</p>
		{#if d.offlineCapBonusHours}<p class="mt-2 text-sm text-jade-300">Công dụng: lần bế quan kế tiếp được thêm {d.offlineCapBonusHours} canh giờ (1 lần mỗi ngày).</p>{/if}
		{#if d.instantPointsMilli}
			<p class="mt-2 text-sm text-jade-300">
				Công dụng: +{formatPoints(d.instantPointsMilli)} tu vi mỗi viên, tối đa {d.maxUsePerGameDay} viên mỗi ngày (tính trong hạn mức hằng ngày).
			</p>
			{#if speed}<p class="mt-1 text-xs text-mist-500">Hôm nay đã nhận từ đan tăng tốc: {formatPoints(speed.milli)}/{formatPoints(speed.capMilli)} điểm</p>{/if}
			{#if qtyOf(d.id) > 0}
				<button class="btn btn-jade mt-5 w-full" disabled={game.busy || speedFull(d)} onclick={async () => { await game.useSpeed(d.id); selected = null; }}>
					<Icon name="pill" size={16} /> {speedFull(d) ? 'Đã đủ hạn mức hôm nay' : 'Dùng'}
				</button>
			{/if}
		{/if}
		{#if d.category === 'artifact'}
			<p class="mt-2 text-sm text-jade-300">
				Công dụng: khi độ lôi kiếp, tỉ lệ vượt kiếp +{formatBp(d.tribulationBonusBp ?? 0)}. Chỉ một pháp bảo hộ thân được tính; tổng cộng thêm có trần.
			</p>
			<p class="mt-1 text-xs text-mist-500">Chỉ rơi từ bí cảnh. Không bán, không trao đổi.</p>
			{#if qtyOf(d.id) > 0}
				{#if equipped?.itemId === d.id}
					<p class="mt-4 flex items-center gap-2 text-sm text-gold-300"><Icon name="check" size={16} /> Đang làm hộ thân</p>
					<button class="btn btn-ghost mt-2 w-full" disabled={equipBusy} onclick={() => void equip('')}>Thu về túi</button>
				{:else}
					<button class="btn btn-jade mt-5 w-full" disabled={equipBusy} onclick={() => void equip(d.id)}>
						<Icon name="bolt" size={16} /> {equipped?.itemId ? 'Đổi làm hộ thân' : 'Dùng làm hộ thân'}
					</button>
				{/if}
			{/if}
		{/if}
		{#if usable(d)}
			<button class="btn btn-jade mt-5 w-full" disabled={game.busy} onclick={async () => { await game.usePhu(); selected = null; }}>
				<Icon name="talisman" size={16} /> Dùng ngay
			</button>
		{/if}
	{/if}
</GameWindow>

<style>
	.worn {
		position: absolute;
		z-index: var(--z-local-1);
		left: 4px;
		top: 4px;
		padding: 0 4px;
		font-size: 0.65rem;
		color: var(--color-ink-950);
		background: var(--color-gold-300);
		border-radius: 2px;
	}
	.locked {
		position: absolute;
		left: 4px;
		top: 4px;
		display: grid;
		place-items: center;
		width: 18px;
		height: 18px;
		color: var(--color-mist-300);
		background: color-mix(in oklab, var(--color-ink-950) 80%, transparent);
		border: 1px solid color-mix(in oklab, var(--color-mist-500) 40%, transparent);
	}
	.tab {
		display: inline-flex;
		align-items: center;
		gap: 0.35rem;
		flex-shrink: 0;
		height: 2.25rem;
		padding: 0 0.9rem;
		font-size: 0.875rem;
		color: var(--color-mist-300);
		border: 1px solid color-mix(in oklab, var(--color-gold-500) 20%, transparent);
	}
	.tab.is-on {
		color: var(--color-ink-950);
		font-weight: 600;
		background: linear-gradient(180deg, var(--color-gold-300), var(--color-gold-500));
	}
	.cell {
		position: relative;
		display: flex;
		aspect-ratio: 1;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		gap: 4px;
		padding: 8px;
		color: var(--r);
		border: 1px solid color-mix(in oklab, var(--r) 55%, transparent);
		background:
			radial-gradient(circle at 50% 35%, color-mix(in oklab, var(--r) 18%, transparent), transparent 70%),
			color-mix(in oklab, var(--color-ink-950) 60%, transparent);
		box-shadow: inset 0 0 0 3px color-mix(in oklab, var(--color-ink-950) 70%, transparent);
		transition: transform 160ms var(--ease-qi);
	}
	button.cell:hover {
		transform: translateY(-2px);
		box-shadow:
			inset 0 0 0 3px color-mix(in oklab, var(--color-ink-950) 70%, transparent),
			0 0 18px -4px var(--r);
	}
	.icon-art {
		width: 62%;
		height: 62%;
		object-fit: contain;
		filter: drop-shadow(0 0 8px color-mix(in oklab, var(--r) 50%, transparent));
	}
	.qty {
		position: absolute;
		top: 4px;
		right: 6px;
		font-size: 0.75rem;
		font-weight: 700;
		color: var(--color-mist-100);
	}
	.name {
		width: 100%;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		text-align: center;
		font-size: 0.75rem;
		color: var(--color-mist-100);
	}
</style>
