<!-- Nhân Vật: who I am. Server data (name, realm, tu vi, rank, resources, Tiên Duyên, faction); equipment slots
     are previews until that system exists (no invented combat stats — the game has no combat yet). -->
<script lang="ts">
	import Icon from '$lib/components/Icon.svelte';
	import NameFx from '$lib/components/NameFx.svelte';
	import Portrait from '$lib/components/Portrait.svelte';
	import { cosmetics } from '$lib/game/cosmetics.svelte';
	import RealmBadge from '$lib/components/RealmBadge.svelte';
	import ScreenFrame from '$lib/components/ScreenFrame.svelte';
	import { artFirst } from '$lib/game/art';
	import { formatPoints, ratePerDay } from '$lib/game/format';
	import { useIdleClock } from '$lib/game/idle.svelte';
	import { game } from '$lib/game/store.svelte';
	import { openWindow } from '$lib/game/windows';
	import FactionEmblem from '$lib/components/FactionEmblem.svelte';
	import { factionColor, factionOf, factions } from '$lib/game/faction.svelte';

	const clock = useIdleClock();
	const ch = $derived(game.character!);
	const cat = $derived(game.catalogue!);
	const realm = $derived(cat.realmOf(ch.stageId));
	const stage = $derived(cat.stages.get(ch.stageId));
	const aura = $derived(realm?.visual.aura ?? 'none');
	const faction = $derived(factionOf(factions.me?.factionId));
	// Full-body art per realm (art-prompts §4): this realm's, else the closest lower realm's, else the portrait.
	const body = $derived(
		artFirst(
			...[...cat.realms.values()]
				.filter((r) => r.path === realm?.path && r.tier <= (realm?.tier ?? 0))
				.sort((a, b) => b.tier - a.tier)
				.map((r) => `characters/body-${r.id}`)
		)
	);
	const days = $derived(Math.max(1, Math.ceil((Date.now() - Date.parse(ch.createdAt)) / 86_400_000)));

	const slots = [
		{ name: 'Pháp bảo', icon: 'seal' },
		{ name: 'Linh khí', icon: 'spark' },
		{ name: 'Đan dược', icon: 'pill' },
		{ name: 'Hộ thân', icon: 'talisman' }
	] as const;
</script>

<svelte:head><title>Nhân Vật · Tiên Đạo</title></svelte:head>

<ScreenFrame title="Nhân Vật" subtitle="Đạo hiệu, cảnh giới và hành trang" icon="person">
	<div class="grid gap-5 lg:grid-cols-[minmax(0,1fr)_minmax(0,1.2fr)]">
		<!-- portrait plate -->
		<section class="hero relative flex flex-col items-center overflow-hidden px-4 py-8 text-center" style:--aura="var(--color-aura-{aura})" style:--fc={factionColor(faction)}>
			<div class="halo absolute inset-0" aria-hidden="true"></div>
			{#if body}
				<img src={body.src} srcset={body.srcset} alt="" class="body relative h-[340px] object-contain" draggable="false" />
				{#if ch.appearance?.avatarFrameId}<div class="absolute top-4 left-4"><Portrait {aura} size={60} frameId={ch.appearance.avatarFrameId} /></div>{/if}
			{:else}
				<div class="relative"><Portrait {aura} size={168} frameId={ch.appearance?.avatarFrameId} /></div>
			{/if}
			<p class="relative mt-4 eyebrow">{realm?.title ?? 'Tu sĩ'}</p>
			<h2 class="relative font-display text-4xl font-bold text-mist-100"><NameFx fx={ch.appearance?.chatAuraId}>{ch.name}</NameFx></h2>
			{#if cosmetics.name(ch.appearance?.titleId)}<p class="relative text-sm text-gold-300">「{cosmetics.name(ch.appearance?.titleId)}」</p>{/if}
			{#if realm}<div class="relative mt-2"><RealmBadge name={realm.name} aura={realm.visual.aura} /></div>{/if}
			<p class="relative mt-1 text-mist-300">{stage?.name}</p>
			<a href="/play/the-luc" class="relative mt-4 inline-flex items-center gap-2 text-sm" style:color="var(--fc)">
				{#if faction}<FactionEmblem id={faction.id} size={20} />{:else}<Icon name="sect" size={16} />{/if}
				{faction ? faction.name : 'Chưa gia nhập thế lực'}
			</a>
			<a href="/play/tui-do?tab=trang-phuc" class="btn btn-ghost relative mt-3 h-10 px-3 text-sm"><Icon name="spark" size={14} /> Trang phục & trang trí</a>
		</section>

		<div class="space-y-5">
			<section>
				<h3 class="panel-title">Đạo cơ</h3>
				<dl class="stats mt-3 grid grid-cols-2 gap-2 sm:grid-cols-3">
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
				<p class="mt-2 text-xs text-mist-500">Ô trang bị khai mở khi bí cảnh thử luyện ra mắt. Trang bị chỉ tăng tỉ lệ vượt kiếp (có trần), không tăng tốc độ tu luyện.</p>
			</section>
		</div>
	</div>
</ScreenFrame>

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
