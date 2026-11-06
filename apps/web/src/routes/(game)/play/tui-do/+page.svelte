<!-- Túi Đồ: server inventory (GET /inventory) + item catalogue (names, rarity, descriptions). Tabs by category. -->
<script lang="ts">
	import GameWindow from '$lib/components/GameWindow.svelte';
	import Icon, { type IconName } from '$lib/components/Icon.svelte';
	import ScreenFrame from '$lib/components/ScreenFrame.svelte';
	import { art } from '$lib/game/art';
	import { game } from '$lib/game/store.svelte';
	import type { ItemDef } from '$lib/game/types';

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
	const tabOf = (d: ItemDef): Tab => (d.category === 'pill' ? 'pill' : d.category === 'herb' ? 'herb' : d.category === 'treasure' ? 'treasure' : 'misc');
	const iconOf = (d: ItemDef): IconName => (d.category === 'pill' ? 'pill' : d.category === 'herb' ? 'lotus' : d.id.endsWith('_phu') ? 'talisman' : 'spark');

	// Raster icons: src/lib/assets/art/items/<item_id>.webp (linh_thach for the currency); SVG icon otherwise.
	const stoneArt = art('items/linh_thach');
	const itemArt = (id: string) => art(`items/${id}`);
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
	const showStones = $derived(tab === 'all' || tab === 'stone');
	const qtyOf = (id: string) => owned.find((x) => x.def.id === id)?.qty ?? 0;
	const usable = (d: ItemDef) => d.id === 'tu_linh_phu' && game.character && game.character.offlineBonusSeconds <= 0 && !game.character.bonusUsedToday;
</script>

<svelte:head><title>Túi Đồ · Tiên Đạo</title></svelte:head>

<ScreenFrame title="Túi Đồ" subtitle="Càn khôn đại — vật phẩm và linh thạch" icon="bag">
	<div class="tabs -mx-1 mb-4 flex gap-1 overflow-x-auto pb-1" role="tablist" aria-label="Loại vật phẩm">
		{#each tabs as t (t.id)}
			<button role="tab" aria-selected={tab === t.id} class="tab" class:is-on={tab === t.id} onclick={() => (tab = t.id)}>{t.label}</button>
		{/each}
	</div>

	{#if !game.inventory}
		<p class="text-mist-500">Đang mở càn khôn đại…</p>
	{:else}
		<ul class="grid grid-cols-3 gap-3 sm:grid-cols-4 xl:grid-cols-6">
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
						<span class="name">{x.def.name}</span>
					</button>
				</li>
			{/each}
		</ul>
		{#if !shown.length && !showStones}
			<p class="py-10 text-center text-mist-500">
				{tab === 'treasure' ? 'Chưa có pháp bảo — pháp bảo rơi ra từ bí cảnh thử luyện (sắp khai mở).' : 'Chưa có vật phẩm loại này.'}
			</p>
		{/if}
	{/if}
</ScreenFrame>

<GameWindow open={!!selected} title={selected?.name ?? ''} onclose={() => (selected = null)}>
	{#if selected}
		{@const d = selected}
		<div class="flex items-start gap-4">
			<div class="cell size-24 shrink-0" style:--r="var(--color-rarity-{d.rarity})">
				{#if selArt}<img src={selArt.src} srcset={selArt.srcset} alt="" class="icon-art" />{:else}<Icon name={iconOf(d)} size={40} />{/if}
			</div>
			<div class="min-w-0">
				<p class="font-semibold" style:color="var(--color-rarity-{d.rarity})">Phẩm {rarityName[d.rarity] ?? d.rarity}</p>
				<p class="mt-1 text-sm text-mist-300">Đang có: <strong class="text-mist-100 tabular-nums">{qtyOf(d.id)}</strong></p>
				<p class="mt-1 text-xs text-mist-500">{d.tradable ? 'Có thể trao đổi' : 'Không thể trao đổi'}</p>
			</div>
		</div>
		<p class="mt-4 leading-relaxed text-mist-100">{d.description}</p>
		{#if d.offlineCapBonusHours}<p class="mt-2 text-sm text-jade-300">Công dụng: lần bế quan kế tiếp được thêm {d.offlineCapBonusHours} canh giờ (1 lần mỗi ngày).</p>{/if}
		{#if usable(d)}
			<button class="btn btn-jade mt-5 w-full" disabled={game.busy} onclick={async () => { await game.usePhu(); selected = null; }}>
				<Icon name="talisman" size={16} /> Dùng ngay
			</button>
		{/if}
	{/if}
</GameWindow>

<style>
	.tab {
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
