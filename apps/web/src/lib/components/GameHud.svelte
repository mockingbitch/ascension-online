<!-- Top HUD of the game shell: who I am, world progress, resources, Tiên Duyên, connection, menu. -->
<script lang="ts">
	import { goto } from '$app/navigation';
	import { feed } from '$lib/api/realtime.svelte';
	import { logout, session } from '$lib/api/session.svelte';
	import { factionColor, factionOf, factions } from '$lib/game/faction.svelte';
	import { formatPoints, progressRatio } from '$lib/game/format';
	import { game } from '$lib/game/store.svelte';
	import { openWindow } from '$lib/game/windows';
	import FactionEmblem from './FactionEmblem.svelte';
	import Icon from './Icon.svelte';
	import Portrait from './Portrait.svelte';
	import WorldProgress from './WorldProgress.svelte';

	const ch = $derived(game.character);
	const realm = $derived(ch && game.catalogue ? game.catalogue.realmOf(ch.stageId) : undefined);
	const faction = $derived(factionOf(factions.me?.factionId));
	let menu = $state<HTMLDetailsElement>();
	const pending = $derived(game.claimable + (game.preview?.ready ? 1 : 0) + (game.vip && game.vip.benefits.dailyGift > 0 && !game.vip.giftClaimedToday ? 1 : 0));
	const ratio = $derived(ch ? progressRatio(ch.cultivationMilli, ch.requiredMilli) : 0);
	// What the bar leads to: the next tầng, or the next realm when this is the bình cảnh stage.
	const target = $derived.by(() => {
		if (!ch || !game.catalogue) return '';
		const next = game.catalogue.nextStage(ch.stageId);
		if (!next) return '';
		const label = game.catalogue.stageLabel(next.id);
		return next.realmId !== ch.realmId ? `đột phá ${label}` : label;
	});
	const phu = $derived(game.catalogue?.items.get('tu_linh_phu'));
	const phuName = $derived(phu?.name ?? 'Tụ Linh Phù');
	const phuTitle = $derived(`${phuName}: ${game.phuCount}${phu?.description ? ` — ${phu.description}` : ''}`);

	async function signOut() {
		await logout();
		await goto('/login');
	}
</script>

<header class="hud flex h-14 shrink-0 items-center gap-2 px-2 sm:gap-3 sm:px-4">
	{#if ch}
		<div class="flex min-w-0 flex-1 items-center gap-2.5">
			<button
				type="button"
				class="shrink-0"
				aria-label="Hồ sơ {ch.name}"
				onclick={() => openWindow({ kind: 'profile', characterId: ch.id, name: ch.name, stageId: ch.stageId })}
			>
				<Portrait aura={realm?.visual.aura ?? 'none'} size={40} frameId={ch.appearance?.avatarFrameId} />
			</button>
			<span class="min-w-0">
				<button
					type="button"
					class="block max-w-full truncate text-left font-display text-lg leading-tight font-bold text-mist-100 hover:text-gold-300"
					onclick={() => openWindow({ kind: 'profile', characterId: ch.id, name: ch.name, stageId: ch.stageId })}
				>
					{ch.name}
				</button>
				<span class="flex min-w-0 items-center gap-1.5 text-xs text-mist-300">
					<span class="truncate">{game.catalogue?.stageLabel(ch.stageId)}{#if game.myRank} · <span class="text-gold-300">Hạng {game.myRank}</span>{/if}</span>
					{#if faction}
						<a href="/play/the-luc" class="faction hidden shrink-0 items-center gap-1 sm:inline-flex" style:color={factionColor(faction)} title="Thế lực: {faction.name}">
							<FactionEmblem id={faction.id} size={14} />{faction.name}
						</a>
					{/if}
					<button type="button" class="vip-badge" onclick={() => openWindow({ kind: 'vip' })} title="Tiên Duyên — xem đặc quyền">
						<Icon name="seal" size={11} />
						{game.vip?.benefits.name ?? 'Tiên Duyên'}
						{#if game.vip && game.vip.benefits.dailyGift > 0 && !game.vip.giftClaimedToday}<span class="dot" aria-label="Có quà chưa nhận"></span>{/if}
					</button>
				</span>
			</span>
		</div>

		<!-- tu vi: the one progress the whole game is about, centred -->
		<div class="qi hidden w-[min(22rem,30vw)] shrink-0 md:block" title="Tu vi tầng hiện tại (số liệu máy chủ)">
			<div class="flex items-baseline justify-between gap-2 text-xs">
				<span class="text-mist-300">Tu vi <strong class="text-sm font-semibold text-mist-100 tabular-nums">{formatPoints(ch.cultivationMilli)}/{formatPoints(ch.requiredMilli)}</strong></span>
				{#if target}<span class="truncate text-mist-300">→ {target}</span>{/if}
			</div>
			<div class="qi-track mt-1" role="progressbar" aria-label="Tu vi tầng hiện tại" aria-valuemin={0} aria-valuemax={100} aria-valuenow={Math.round(ratio * 100)}>
				<div class="qi-fill" style:width="{ratio * 100}%"></div>
			</div>
		</div>
	{/if}

	<div class="flex flex-1 items-center justify-end gap-1.5 sm:gap-3">
		<WorldProgress />
		<span class="res" title="Linh thạch" aria-label="Linh thạch: {game.stones.toLocaleString('vi-VN')}" role="img">
			<Icon name="stone" size={16} class="text-spirit-cyan" />
			<span class="tabular-nums">{game.stones.toLocaleString('vi-VN')}</span>
		</span>
		<span class="res hidden sm:inline-flex" title={phuTitle} aria-label="{phuName}: {game.phuCount}" role="img">
			<Icon name="talisman" size={16} class="text-gold-500" />
			<span class="tabular-nums" aria-hidden="true">{game.phuCount}</span>
		</span>
		<button type="button" class="relative inline-flex size-10 items-center justify-center text-mist-300 hover:text-mist-100" onclick={() => openWindow({ kind: 'notifications' })}>
			<Icon name="bell" label="Thông báo" />
			{#if pending}<span class="count tabular-nums">{pending}</span>{/if}
		</button>
		<span
			class="conn"
			class:on={feed.connected}
			role="status"
			title={feed.connected ? 'Trực tuyến — đã kết nối truyền âm' : 'Mất kết nối — đang kết nối lại…'}
			aria-label={feed.connected ? 'Trực tuyến' : 'Đang kết nối lại'}
		></span>

		<details bind:this={menu} class="relative">
			<summary class="inline-flex size-10 cursor-pointer list-none items-center justify-center text-mist-300 hover:text-mist-100">
				<Icon name="gear" label="Tùy chọn" />
			</summary>
			<div class="panel absolute top-11 right-0 z-40 w-56 p-1.5 text-sm" role="menu">
				<a role="menuitem" class="menu-item" href="/chinh-sach-du-lieu" onclick={() => menu?.removeAttribute('open')}>Chính sách dữ liệu</a>
				{#if session.user?.role === 'admin'}
					<a role="menuitem" class="menu-item" href="/admin/reports" onclick={() => menu?.removeAttribute('open')}>Quản trị báo cáo</a>
				{/if}
				<button role="menuitem" type="button" class="menu-item w-full text-left text-cinnabar-300" onclick={signOut}>
					<Icon name="exit" size={16} /> Đăng xuất
				</button>
			</div>
		</details>
	</div>
</header>

<style>
	.hud {
		background: linear-gradient(180deg, color-mix(in oklab, var(--color-ink-950) 94%, transparent), color-mix(in oklab, var(--color-ink-950) 72%, transparent));
		border-bottom: 1px solid color-mix(in oklab, var(--color-gold-500) 22%, transparent);
		backdrop-filter: blur(6px);
	}
	.res {
		display: inline-flex;
		height: 2.25rem;
		align-items: center;
		gap: 0.375rem;
		padding: 0 0.625rem;
		font-size: 0.875rem;
		font-weight: 600;
		color: var(--color-mist-100);
		border: 1px solid color-mix(in oklab, var(--color-gold-500) 22%, transparent);
		background: color-mix(in oklab, var(--color-ink-950) 55%, transparent);
		border-radius: 2px;
	}
	.vip-badge {
		position: relative;
		display: inline-flex;
		flex-shrink: 0;
		align-items: center;
		gap: 0.2rem;
		padding: 0 0.4rem;
		font-size: 0.68rem;
		font-weight: 600;
		line-height: 1.15rem;
		color: var(--color-gold-300);
		border: 1px solid color-mix(in oklab, var(--color-gold-500) 45%, transparent);
		border-radius: 9999px;
		background: color-mix(in oklab, var(--color-gold-500) 10%, transparent);
	}
	.vip-badge:hover {
		background: color-mix(in oklab, var(--color-gold-500) 20%, transparent);
	}
	.qi-track {
		height: 8px;
		overflow: hidden;
		border-radius: 9999px;
		background: var(--color-ink-950);
		box-shadow: inset 0 0 0 1px color-mix(in oklab, var(--color-gold-500) 25%, transparent);
	}
	.qi-fill {
		height: 100%;
		border-radius: inherit;
		background: linear-gradient(90deg, var(--color-jade-700), var(--color-jade-500) 45%, var(--color-spirit-cyan));
		box-shadow: 0 0 8px color-mix(in oklab, var(--color-spirit-cyan) 55%, transparent);
		transition: width 600ms var(--ease-qi);
	}
	.dot {
		position: absolute;
		top: -3px;
		right: -3px;
		width: 8px;
		height: 8px;
		border-radius: 9999px;
		background: var(--color-cinnabar-500);
	}
	.count {
		position: absolute;
		top: 2px;
		right: 0;
		min-width: 17px;
		height: 17px;
		padding: 0 4px;
		border-radius: 9999px;
		font-size: 0.65rem;
		font-weight: 700;
		line-height: 17px;
		color: var(--color-mist-100);
		background: var(--color-cinnabar-500);
	}
	.conn {
		width: 8px;
		height: 8px;
		border-radius: 9999px;
		background: var(--color-mist-500);
	}
	.conn.on {
		background: var(--color-jade-300);
		box-shadow: 0 0 8px var(--color-jade-300);
	}
	.menu-item {
		display: flex;
		align-items: center;
		gap: 0.5rem;
		min-height: 2.5rem;
		padding: 0 0.75rem;
		color: var(--color-mist-100);
	}
	.menu-item:hover {
		background: color-mix(in oklab, var(--color-gold-500) 10%, transparent);
	}
</style>
