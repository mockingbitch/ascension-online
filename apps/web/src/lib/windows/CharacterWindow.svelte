<!-- Nhân Vật (window): who I am. Server data (name, realm, tu vi, rank, resources, Tiên Duyên, faction); equipment slots
     are previews until that system exists (no invented combat stats — the game has no combat yet).
     Tab "Trang phục" (Gương Thủy Nguyệt, decisions 2026-11-09 #9): dáng người / trang phục (bộ vẽ sẵn) / tư thế and the
     cosmetics (CosmeticsPanel). Looks only. -->
<script lang="ts">
	import Icon from '$lib/components/Icon.svelte';
	import NameFx from '$lib/components/NameFx.svelte';
	import Portrait from '$lib/components/Portrait.svelte';
	import { cosmetics } from '$lib/game/cosmetics.svelte';
	import RealmBadge from '$lib/components/RealmBadge.svelte';
	import { artFirst } from '$lib/game/art';
	import { formatPoints, ratePerDay } from '$lib/game/format';
	import { useIdleClock } from '$lib/game/idle.svelte';
	import { game } from '$lib/game/store.svelte';
	import FactionEmblem from '$lib/components/FactionEmblem.svelte';
	import CosmeticsPanel from '$lib/components/CosmeticsPanel.svelte';
	import CultivatorFigure from '$lib/components/CultivatorFigure.svelte';
	import { charArtFirst } from '$lib/game/charArt.svelte';
	import { factionColor, myFaction } from '$lib/game/faction.svelte';
	import { bodySlots, figureOf } from '$lib/game/looks';
	import { openWindow, updateWindow } from '$lib/game/windows';
	import type { WindowProps } from './registry';

	let { tab }: WindowProps = $props();
	const toCosmetics = () => updateWindow({ kind: 'character', tab: 'cosmetics' });

	const clock = useIdleClock();
	const ch = $derived(game.character!);
	const cat = $derived(game.catalogue!);
	const realm = $derived(cat.realmOf(ch.stageId));
	const stage = $derived(cat.stages.get(ch.stageId));
	const aura = $derived(realm?.visual.aura ?? 'none');
	// Membership comes from /factions/me; the roster only supplies the name/colour. Never show "no faction" while either
	// is still loading (the old check read a missing roster as "not a member").
	const mine = $derived(myFaction());
	const faction = $derived(mine.state === 'member' ? mine.view : undefined);
	// Full-body art per realm (art-prompts §4): this realm's, else the closest lower realm's. With a chosen look the
	// body's own folder (characters/<body_id>/body-<realm_id>, lazy) and then the SVG figure of the look — never the
	// shared art of another figure; without a look the shared characters/body-<realm_id> chain, else the portrait.
	const chain = $derived(
		[...cat.realms.values()]
			.filter((r) => r.path === realm?.path && r.tier <= (realm?.tier ?? 0))
			.sort((a, b) => b.tier - a.tier)
			.map((r) => r.id)
	);
	const look = $derived(ch.appearance?.look);
	const figure = $derived(figureOf(cosmetics.looks, look));
	const body = $derived(look ? charArtFirst(bodySlots(look, chain, realm?.visual.fxTier ?? 1)) : artFirst(...chain.map((r) => `characters/body-${r}`)));
	const days = $derived(Math.max(1, Math.ceil((Date.now() - Date.parse(ch.createdAt)) / 86_400_000)));

	const slots = [
		{ name: 'Pháp bảo', icon: 'seal' },
		{ name: 'Linh khí', icon: 'spark' },
		{ name: 'Đan dược', icon: 'pill' },
		{ name: 'Hộ thân', icon: 'talisman' }
	] as const;
</script>

{#if tab === 'cosmetics'}
	<CosmeticsPanel />
{:else}
	<div class="grid gap-5 lg:grid-cols-[minmax(0,1fr)_minmax(0,1.2fr)]">
		<!-- portrait plate -->
		<section class="hero relative flex flex-col items-center overflow-hidden px-4 py-8 text-center" style:--aura="var(--color-aura-{aura})" style:--fc={factionColor(faction)}>
			<div class="halo absolute inset-0" aria-hidden="true"></div>
			{#if body || look}
				{#if body}
					<img src={body.src} srcset={body.srcset} alt="" class="body relative h-[340px] object-contain" draggable="false" />
				{:else}
					<div class="body relative h-[300px] w-full max-w-[340px]" data-testid="look-figure"><CultivatorFigure {aura} {figure} class="size-full" /></div>
				{/if}
				<div class="absolute top-4 left-4"><Portrait {aura} size={60} frameId={ch.appearance?.avatarFrameId} {look} fxTier={realm?.visual.fxTier} /></div>
			{:else}
				<div class="relative"><Portrait {aura} size={168} frameId={ch.appearance?.avatarFrameId} {look} fxTier={realm?.visual.fxTier} /></div>
			{/if}
			<p class="relative mt-4 eyebrow">{realm?.title ?? 'Tu sĩ'}</p>
			<h2 class="relative font-display text-4xl font-bold text-mist-100"><NameFx fx={ch.appearance?.chatAuraId}>{ch.name}</NameFx></h2>
			{#if cosmetics.name(ch.appearance?.titleId)}<p class="relative text-sm text-gold-300">「{cosmetics.name(ch.appearance?.titleId)}」</p>{/if}
			{#if realm}<div class="relative mt-2"><RealmBadge name={realm.name} aura={realm.visual.aura} /></div>{/if}
			<p class="relative mt-1 text-mist-300">{stage?.name}</p>
			<a href="/play/the-luc" class="relative mt-4 inline-flex items-center gap-2 text-sm" style:color="var(--fc)" data-testid="my-faction" data-state={mine.state}>
				{#if mine.state === 'member'}<FactionEmblem id={mine.id} size={20} />{:else}<Icon name="sect" size={16} />{/if}
				{#if mine.state === 'member'}{faction?.name ?? 'Thế lực của đạo hữu'}{:else if mine.state === 'none'}Chưa gia nhập thế lực{:else}Đang tải thế lực…{/if}
			</a>
			<button type="button" class="btn btn-ghost relative mt-3 h-10 px-3 text-sm" onclick={toCosmetics}>
				<Icon name="spark" size={14} /> Trang phục & trang trí
			</button>
		</section>

		<div class="space-y-5">
			<section>
				<h3 class="panel-title">Đạo cơ</h3>
				<dl class="stats stagger mt-3 grid grid-cols-2 gap-2 sm:grid-cols-3">
					<div><dt>Tổng tu vi</dt><dd>{formatPoints((stage?.cumulativeBeforeMilli ?? 0) + clock.shown)}</dd></div>
					<div><dt>Thiên Bảng</dt><dd class="text-gold-300">{game.myRank ? `Hạng ${game.myRank}` : '—'}</dd></div>
					<div><dt>Tốc độ treo máy</dt><dd class="text-jade-300">{ratePerDay(ch.idleRateMilliPerHour)}</dd></div>
					<div><dt>Giới hạn bế quan</dt><dd>{Math.round(ch.offlineCapSeconds / 3600)} canh giờ</dd></div>
					<div><dt>Tâm cảnh</dt><dd>{ch.failStreak ? `${ch.failStreak} lần vững thêm` : 'Bình ổn'}</dd></div>
					<div><dt>Ngày tu đạo</dt><dd>{days} ngày</dd></div>
				</dl>
			</section>

			<section>
				<h3 class="panel-title">Tài nguyên</h3>
				<ul class="mt-3 grid grid-cols-2 gap-2 text-sm">
					<li class="res"><Icon name="stone" size={18} class="text-spirit-cyan" /> Linh thạch <strong class="ml-auto tabular-nums">{game.stones.toLocaleString('vi-VN')}</strong></li>
					<li class="res"><Icon name="talisman" size={18} class="text-gold-500" /> Tụ Linh Phù <strong class="ml-auto tabular-nums">{game.phuCount}</strong></li>
				</ul>
				<button class="mt-2 inline-flex items-center gap-1 text-sm text-gold-300 hover:underline" onclick={() => openWindow({ kind: 'vip' })}>
					<Icon name="seal" size={15} /> {game.vip?.benefits.name ?? 'Tiên Duyên'} — xem đặc quyền
				</button>
			</section>

			<section>
				<h3 class="panel-title">Hành trang</h3>
				<ul class="mt-3 grid grid-cols-4 gap-2">
					{#each slots as s (s.name)}
						<li class="slot" title="{s.name} — khai mở cùng hệ thống pháp bảo (bí cảnh)">
							<Icon name={s.icon} size={26} />
							<span class="text-xs">{s.name}</span>
							<Icon name="lock" size={12} class="absolute top-1.5 right-1.5 text-mist-500" />
						</li>
					{/each}
				</ul>
			</section>
		</div>
	</div>
{/if}

<style>
	.hero {
		border: 1px solid color-mix(in oklab, var(--fc) 40%, transparent);
		background: linear-gradient(180deg, color-mix(in oklab, var(--color-ink-800) 80%, transparent), color-mix(in oklab, var(--color-ink-950) 90%, transparent));
	}
	.halo {
		background:
			radial-gradient(circle at 50% 30%, color-mix(in oklab, var(--aura) 30%, transparent), transparent 55%),
			radial-gradient(circle at 50% 120%, color-mix(in oklab, var(--fc) 25%, transparent), transparent 60%);
		animation: var(--animate-qi-breathe);
	}
	.body {
		filter: drop-shadow(0 0 24px color-mix(in oklab, var(--aura) 45%, transparent));
	}
	.stats > div {
		padding: 0.6rem 0.75rem;
		border: 1px solid color-mix(in oklab, var(--color-gold-500) 15%, transparent);
		background: color-mix(in oklab, var(--color-ink-950) 40%, transparent);
	}
	.stats dt {
		font-size: 0.7rem;
		letter-spacing: 0.1em;
		text-transform: uppercase;
		color: var(--color-mist-500);
	}
	.stats dd {
		margin-top: 2px;
		font-weight: 600;
		font-variant-numeric: tabular-nums;
	}
	.res {
		display: flex;
		align-items: center;
		gap: 0.5rem;
		padding: 0.6rem 0.75rem;
		border: 1px solid color-mix(in oklab, var(--color-gold-500) 15%, transparent);
	}
	.slot {
		position: relative;
		display: flex;
		aspect-ratio: 1;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		gap: 4px;
		color: var(--color-mist-500);
		border: 1px dashed color-mix(in oklab, var(--color-gold-500) 30%, transparent);
		background: color-mix(in oklab, var(--color-ink-950) 50%, transparent);
	}
</style>
