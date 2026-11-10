<!--
	Trang phục & trang trí — "Gương Thủy Nguyệt" in Nhân Vật (GET /cosmetics, POST /cosmetics/buy, POST /cosmetics/equip):
	the look (dáng người, trang phục vẽ sẵn, tư thế — appearances.yaml, decisions 2026-11-09), titles, portrait
	frames, chat name effects and Động Phủ decorations. Looks only — none of it changes tu vi, odds, rank or điểm duyên. Ownership,
	prices, season/era windows and Tiên Duyên grants come from the server; the preview is a local draft until saved.
-->
<script lang="ts">
	import { api, ApiError, newIdempotencyKey } from '$lib/api/client';
	import { cosmetics, kindLabel, lookFx, slugOf } from '$lib/game/cosmetics.svelte';
	import { game } from '$lib/game/store.svelte';
	import { toast } from '$lib/game/toasts.svelte';
	import { sameLook } from '$lib/game/looks';
	import type { Appearance, CosmeticDef, CosmeticKind, CosmeticsView } from '$lib/game/types';
	import LookPicker from './LookPicker.svelte';
	import ChatSample from './ChatSample.svelte';
	import CosmeticArt from './CosmeticArt.svelte';
	import TitlePlaque from './TitlePlaque.svelte';
	import GameWindow from './GameWindow.svelte';
	import Icon from './Icon.svelte';
	import NameFx from './NameFx.svelte';
	import Portrait from './Portrait.svelte';

	let view = $state<CosmeticsView | null>(null);
	let failed = $state(false);
	let kind = $state<CosmeticKind | 'look'>('look');
	let draft = $state<Appearance>({ caveDecorationIds: [] });
	let pick = $state<CosmeticDef | null>(null);
	let busy = $state(false);
	let buyKey: string | null = null; // kept across network errors so a retry replays the same purchase
	let saveKey: string | null = null;

	const kinds: CosmeticKind[] = ['avatar_frame', 'title', 'chat_aura', 'cave_decoration'];
	const tabLabel = (k: CosmeticKind | 'look') => (k === 'look' ? 'Dáng người' : kindLabel[k]);

	function adopt(v: CosmeticsView) {
		view = v;
		draft = { ...v.equipped, caveDecorationIds: [...v.equipped.caveDecorationIds], look: v.equipped.look ? { ...v.equipped.look } : undefined };
		if (!v.equipped.look && kind === 'look') kind = 'avatar_frame'; // an older server serves no look
	}

	$effect(() => {
		void cosmetics.load();
		api
			.cosmetics()
			.then(adopt)
			.catch(() => (failed = true));
	});

	const cat = $derived(cosmetics.catalog);
	const slots = $derived(cat?.settings.caveSlots ?? 0);
	const owned = $derived(new Set(view?.owned ?? []));
	const onSale = $derived(new Set(view?.available ?? []));
	const series = $derived((cat?.series ?? []).filter((s) => s.kind === kind));
	const aura = $derived(game.catalogue?.realmOf(game.character?.stageId ?? '')?.visual.aura ?? 'none');
	const fxTier = $derived(game.catalogue?.realmOf(game.character?.stageId ?? '')?.visual.fxTier ?? 1);
	const lookDirty = $derived(!!view && !!draft.look && !sameLook(draft.look, view.equipped.look));
	const dirty = $derived(
		!!view &&
			(lookDirty ||
				(draft.titleId ?? '') + (draft.avatarFrameId ?? '') + (draft.chatAuraId ?? '') + draft.caveDecorationIds.join() !==
					(view.equipped.titleId ?? '') +
						(view.equipped.avatarFrameId ?? '') +
						(view.equipped.chatAuraId ?? '') +
						view.equipped.caveDecorationIds.join())
	);
	// Look editing: the character's path (server stage) and the 24 h body cooldown the server reported.
	const path = $derived(game.catalogue?.realmOf(game.character?.stageId ?? '')?.path ?? 'tien');
	const bodyLocked = $derived(!!view?.bodyRetryAt && Date.parse(view.bodyRetryAt) > Date.now());
	const lockNote = $derived(
		view?.bodyRetryAt
			? `Vừa đổi dáng người — đổi tiếp được từ ${new Date(view.bodyRetryAt).toLocaleString('vi-VN', { hour: '2-digit', minute: '2-digit', day: '2-digit', month: '2-digit' })}. Phụ kiện, màu áo, tư thế đổi tự do.`
			: ''
	);

	const slotKey = { title: 'titleId', avatar_frame: 'avatarFrameId', chat_aura: 'chatAuraId' } as const;
	const worn = (c: CosmeticDef) =>
		c.kind === 'cave_decoration' ? draft.caveDecorationIds.includes(c.id) : draft[slotKey[c.kind]] === c.id;

	/** Why an unowned cosmetic can't be bought now ('' = buyable). */
	function lockOf(c: CosmeticDef, v: CosmeticsView): string {
		if (c.source === 'tien_duyen') return `Tiên Duyên ${c.perkTier}`;
		if (!onSale.has(c.id)) return c.season ? `Mùa ${cosmetics.seasonName(c.season)}` : 'Ngoài thời bán';
		if ((c.minTier ?? 0) > v.tier) return `Cần Tiên Duyên ${c.minTier}`;
		return '';
	}

	function toggle(c: CosmeticDef) {
		if (c.kind === 'cave_decoration') {
			const ids = draft.caveDecorationIds;
			if (ids.includes(c.id)) draft.caveDecorationIds = ids.filter((x) => x !== c.id);
			else if (ids.length < slots) draft.caveDecorationIds = [...ids, c.id];
			else toast('err', `Động phủ chỉ bày được ${slots} món — cất bớt một món trước.`);
			return;
		}
		const k = slotKey[c.kind];
		draft[k] = draft[k] === c.id ? undefined : c.id;
	}

	async function save() {
		busy = true;
		saveKey ??= newIdempotencyKey();
		try {
			// the look rides along only when it changed (absent = keep, so a cosmetics-only save never touches it)
			const changedLook = lookDirty || (draft.avatarFrameId ?? '') !== (view?.equipped.avatarFrameId ?? '');
			const r = await api.cosmeticEquip(lookDirty ? draft : { ...draft, look: undefined }, saveKey);
			saveKey = null;
			if (changedLook) lookFx.seq++; // after the server confirmed it
			adopt(r.state);
			toast('ok', 'Đã thay trang phục.');
			await game.reload();
		} catch (e) {
			if (e instanceof ApiError && e.status < 500) saveKey = null;
			toast('err', e instanceof ApiError ? e.message : String(e));
		} finally {
			busy = false;
		}
	}

	async function buy() {
		if (!pick) return;
		busy = true;
		buyKey ??= newIdempotencyKey();
		try {
			const r = await api.cosmeticBuy(pick.id, buyKey);
			buyKey = null;
			const keep = draft;
			adopt(r.state);
			draft = keep; // keep unsaved choices
			toast('ok', `Đã mua ${pick.name} (−${r.price.toLocaleString('vi-VN')} linh thạch).`);
			pick = null;
			await game.reload();
		} catch (e) {
			if (e instanceof ApiError && e.status < 500) buyKey = null;
			toast('err', e instanceof ApiError ? e.message : String(e));
			api.cosmetics().then((v) => (view = v)).catch(() => {});
		} finally {
			busy = false;
		}
	}
</script>

{#if failed}
	<p class="py-10 text-center text-mist-500">Chưa mở được tủ trang phục — thử lại sau.</p>
{:else if !view || !cat}
	<p class="text-mist-500">Đang mở tủ trang phục…</p>
{:else}
	{@const v = view}
	<!-- preview: what others will see -->
	<section class="preview mb-4 grid items-center gap-4 p-4 sm:grid-cols-[auto_minmax(0,1fr)]">
		<div class="flex items-center gap-4">
			<Portrait {aura} size={88} frameId={draft.avatarFrameId} look={draft.look} fxTier={fxTier} mine />
			<div class="min-w-0">
				<p class="eyebrow">Người khác thấy</p>
				<p class="truncate font-display text-2xl font-bold text-mist-100">
					<NameFx fx={draft.chatAuraId}>{game.character?.name ?? ''}</NameFx>
				</p>
				<p class="min-h-6">
					{#if draft.titleId}<TitlePlaque name={cosmetics.name(draft.titleId)} rarity={cosmetics.get(draft.titleId)?.rarity} size="sm" />{/if}
				</p>
				<div class="mt-1.5 max-w-[17rem]"><ChatSample name={game.character?.name ?? ''} fx={draft.chatAuraId} /></div>
			</div>
		</div>
		<div>
			<p class="eyebrow mb-1.5">Động phủ · {draft.caveDecorationIds.length}/{slots} món</p>
			<ul class="flex flex-wrap gap-2">
				{#each Array.from({ length: slots }, (_, i) => draft.caveDecorationIds[i]) as id, i (i)}
					<li class="slot" title={id ? cosmetics.name(id) : 'Trống'}>
						{#if id}<CosmeticArt kind="cave_decoration" slug={slugOf(id)} rarity={cosmetics.get(id)?.rarity} size={44} />{:else}<span class="text-xs text-mist-500">Trống</span>{/if}
					</li>
				{/each}
			</ul>
		</div>
		<div class="flex flex-wrap items-center gap-x-4 gap-y-2 text-sm text-mist-300 sm:col-span-2">
			<span class="flex items-center gap-1.5"><Icon name="stone" size={16} /> <strong class="text-mist-100 tabular-nums">{v.balance.toLocaleString('vi-VN')}</strong> linh thạch</span>
			<span class="text-xs text-mist-500">Mùa {cosmetics.seasonName(v.season) || '—'} · trang trí theo mùa đổi mỗi quý</span>
			<button class="btn btn-jade ml-auto h-10 px-4 text-sm" disabled={!dirty || busy} onclick={save}>
				<Icon name="check" size={16} /> {dirty ? 'Lưu trang phục' : 'Đang mặc'}
			</button>
		</div>
	</section>

	<div class="-mx-1 mb-3 flex gap-1 overflow-x-auto pb-1" role="tablist" aria-label="Loại trang phục">
		{#each [...(draft.look && cosmetics.looks ? (['look'] as const) : []), ...kinds] as k (k)}
			<button role="tab" aria-selected={kind === k} class="tab" class:is-on={kind === k} onclick={() => (kind = k)}>{tabLabel(k)}</button>
		{/each}
	</div>

	{#if kind === 'look' && draft.look && cosmetics.looks}
		<div class="mb-4" data-testid="look-editor">
			<LookPicker catalog={cosmetics.looks} {path} bind:look={draft.look} {aura} frameId={draft.avatarFrameId}
				currentBody={view.equipped.look?.bodyId} lockedBody={bodyLocked} {lockNote} />
		</div>
	{/if}

	{#each series as s (s.id)}
		<section class="mb-4">
			<h3 class="mb-2 flex items-center gap-2 text-sm font-semibold text-mist-300">
				{s.name}
				{#if s.source === 'tien_duyen'}<span class="chip">Quà Tiên Duyên</span>{/if}
			</h3>
			<ul class="grid grid-cols-2 gap-2 sm:grid-cols-3 xl:grid-cols-4">
				{#each s.items as id (id)}
					{@const c = cosmetics.get(id)}
					{#if c}
						{@const has = owned.has(c.id)}
						{@const lock = has ? '' : lockOf(c, v)}
						<li class="card" class:on={worn(c)} style:--r="var(--color-rarity-{c.rarity})">
							<button type="button" class="flex w-full flex-col items-center gap-1 px-2 pt-3 pb-2 text-center" onclick={() => (has ? toggle(c) : (pick = c))}>
								<!-- each skin previewed as it will look: frames on the player's own portrait, titles as their plaque,
								     chat effects on a chat line -->
								{#if c.kind === 'avatar_frame'}
									<span class="my-1"><Portrait {aura} size={64} frameId={c.id} look={draft.look} fxTier={fxTier} /></span>
								{:else if c.kind === 'title'}
									<span class="flex min-h-16 w-full items-center justify-center px-1"><TitlePlaque name={c.name} rarity={c.rarity} wrap /></span>
								{:else if c.kind === 'chat_aura'}
									<span class="flex min-h-16 w-full items-center"><ChatSample name={game.character?.name || c.name} fx={c.id} text="an hảo" /></span>
								{:else}
									<CosmeticArt kind={c.kind} slug={c.slug} rarity={c.rarity} label={c.name} size={56} />
								{/if}
								{#if c.kind !== 'title'}
									<span class="line-clamp-2 min-h-10 text-sm font-medium text-mist-100">
										{#if c.kind === 'chat_aura'}<NameFx fx={c.id}>{c.name}</NameFx>{:else}{c.name}{/if}
									</span>
								{/if}
								{#if has}
									<span class="state" class:gold={worn(c)}>{worn(c) ? 'Đang dùng' : 'Đã có · chạm để dùng'}</span>
								{:else if lock}
									<span class="state"><Icon name="lock" size={11} /> {lock}</span>
								{:else}
									<span class="state price"><Icon name="stone" size={12} />{(c.price ?? 0).toLocaleString('vi-VN')}</span>
								{/if}
							</button>
						</li>
					{/if}
				{/each}
			</ul>
		</section>
	{/each}
	<p class="text-xs leading-relaxed text-mist-500">
		Trang phục chỉ để người khác nhìn thấy — không tăng tu vi, tỉ lệ đột phá, thứ hạng hay điểm duyên. Đồ theo mùa và theo thời thế chỉ bán
		trong thời của nó; đã mua thì giữ mãi.
	</p>

	<GameWindow open={!!pick} title={pick?.name ?? ''} onclose={() => (pick = null)}>
		{#if pick}
			{@const c = pick}
			{@const lock = lockOf(c, v)}
			<div class="flex items-center gap-4">
				{#if c.kind === 'avatar_frame'}
					<Portrait {aura} size={88} frameId={c.id} look={draft.look} />
				{:else}
					<CosmeticArt kind={c.kind} slug={c.slug} rarity={c.rarity} label={c.name} size={88} />
				{/if}
				<div class="min-w-0">
					<p class="eyebrow">{kindLabel[c.kind]}</p>
					{#if c.kind === 'chat_aura'}
						<p class="font-display text-2xl font-bold"><NameFx fx={c.id}>{game.character?.name ?? c.name}</NameFx></p>
						<div class="mt-1.5"><ChatSample name={game.character?.name ?? c.name} fx={c.id} /></div>
					{:else if c.kind === 'title'}
						<TitlePlaque name={c.name} rarity={c.rarity} />
					{/if}
				</div>
			</div>
			{#if c.description}<p class="mt-3 leading-relaxed text-mist-100">{c.description}</p>{/if}
			{#if c.source === 'tien_duyen'}
				<p class="mt-4 text-sm text-mist-300">Quà của Tiên Duyên {c.perkTier} — tự có khi đạt cấp, không bán.</p>
			{:else}
				<dl class="mt-4 space-y-1 text-sm">
					<div class="flex justify-between"><dt class="text-mist-300">Giá</dt><dd class="font-semibold text-gold-300 tabular-nums">{(c.price ?? 0).toLocaleString('vi-VN')} linh thạch</dd></div>
					<div class="flex justify-between"><dt class="text-mist-300">Đang có</dt><dd class="tabular-nums">{v.balance.toLocaleString('vi-VN')} linh thạch</dd></div>
				</dl>
				{#if lock}<p class="mt-3 text-sm text-mist-300"><Icon name="lock" size={12} /> {lock}</p>{/if}
				<button class="btn btn-jade mt-5 w-full" disabled={busy || !!lock || (c.price ?? 0) > v.balance} onclick={buy}>
					<Icon name="check" size={16} /> {(c.price ?? 0) > v.balance ? 'Chưa đủ linh thạch' : 'Xác nhận mua'}
				</button>
			{/if}
		{/if}
	</GameWindow>
{/if}

<style>
	.tab {
		display: inline-flex;
		align-items: center;
		flex-shrink: 0;
		height: 2.5rem;
		padding: 0 0.9rem;
		font-size: 0.875rem;
		color: var(--color-mist-300);
		border-bottom: 2px solid transparent;
	}
	.tab.is-on {
		color: var(--color-gold-300);
		font-weight: 600;
		border-bottom-color: var(--color-gold-500);
	}
	.preview {
		border: 1px solid color-mix(in oklab, var(--color-gold-500) 25%, transparent);
		background:
			radial-gradient(ellipse at 15% 50%, color-mix(in oklab, var(--color-jade-500) 12%, transparent), transparent 60%),
			color-mix(in oklab, var(--color-ink-950) 55%, transparent);
	}
	.slot {
		display: grid;
		place-items: center;
		width: 3.25rem;
		height: 3.25rem;
		border: 1px dashed color-mix(in oklab, var(--color-gold-500) 30%, transparent);
	}
	.card {
		border: 1px solid color-mix(in oklab, var(--r) 30%, transparent);
		background: color-mix(in oklab, var(--color-ink-950) 60%, transparent);
		transition: border-color 200ms var(--ease-qi);
	}
	@media (hover: hover) {
		.card:hover {
			border-color: color-mix(in oklab, var(--r) 70%, var(--color-gold-300));
			transform: translateY(-2px);
			box-shadow: 0 8px 18px -10px color-mix(in oklab, var(--r) 70%, transparent);
		}
	}
	.card {
		transition:
			border-color var(--dur-fast) var(--ease-qi),
			transform var(--dur-fast) var(--ease-qi),
			box-shadow var(--dur-fast) var(--ease-qi);
	}
	.card.on {
		border-color: var(--color-gold-300);
		background: color-mix(in oklab, var(--color-gold-500) 10%, var(--color-ink-950));
	}
	.state {
		display: inline-flex;
		align-items: center;
		gap: 0.25rem;
		font-size: 0.75rem;
		color: var(--color-mist-500);
	}
	.state.gold {
		color: var(--color-gold-300);
	}
	.state.price {
		color: var(--color-gold-300);
		font-variant-numeric: tabular-nums;
	}
	.chip {
		font-size: 0.7rem;
		padding: 0.05rem 0.4rem;
		border: 1px solid color-mix(in oklab, var(--color-gold-500) 40%, transparent);
		color: var(--color-gold-300);
	}
</style>
