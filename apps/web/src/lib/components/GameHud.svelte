<!-- Top HUD of the game shell: who I am, resources, Tiên Duyên, connection, menu. -->
<script lang="ts">
	import { goto } from '$app/navigation';
	import { feed } from '$lib/api/realtime.svelte';
	import { logout, session } from '$lib/api/session.svelte';
	import { formatPoints, progressRatio } from '$lib/game/format';
	import { game } from '$lib/game/store.svelte';
	import { openWindow } from '$lib/game/windows';
	import Icon from './Icon.svelte';
	import Portrait from './Portrait.svelte';
	import RealmBadge from './RealmBadge.svelte';

	const ch = $derived(game.character);
	const realm = $derived(ch && game.catalogue ? game.catalogue.realmOf(ch.stageId) : undefined);
	let menu = $state<HTMLDetailsElement>();
	const pending = $derived(game.claimable + (game.preview?.ready ? 1 : 0) + (game.vip && game.vip.benefits.dailyGift > 0 && !game.vip.giftClaimedToday ? 1 : 0));
	const ratio = $derived(ch ? progressRatio(ch.cultivationMilli, ch.requiredMilli) : 0);

	async function signOut() {
		await logout();
		await goto('/login');
	}
</script>

<header class="hud flex h-14 shrink-0 items-center gap-2 px-2 sm:gap-3 sm:px-4">
	{#if ch}
		<button
			type="button"
			class="flex min-w-0 items-center gap-2.5 text-left"
			onclick={() => openWindow({ kind: 'profile', characterId: ch.id, name: ch.name, stageId: ch.stageId })}
		>
			<Portrait aura={realm?.visual.aura ?? 'none'} size={40} />
			<span class="min-w-0">
				<span class="block truncate font-display text-lg leading-tight font-bold text-mist-100">{ch.name}</span>
				<span class="block truncate text-xs text-mist-300">
					{game.catalogue?.stageLabel(ch.stageId)}{#if game.myRank} · <span class="text-gold-300">Hạng {game.myRank}</span>{/if}
				</span>
			</span>
		</button>
		{#if realm}<span class="hidden lg:block"><RealmBadge name={realm.name} aura={realm.visual.aura} size="sm" /></span>{/if}
		<div class="hidden w-44 md:block" title="Tu vi tầng hiện tại (máy chủ)">
			<div class="flex justify-between text-xs text-mist-500"><span>Tu vi</span><span class="text-mist-300 tabular-nums">{formatPoints(ch.cultivationMilli)}/{formatPoints(ch.requiredMilli)}</span></div>
			<div class="mt-1 h-1.5 bg-ink-950" role="progressbar" aria-label="Tu vi" aria-valuemin={0} aria-valuemax={100} aria-valuenow={Math.round(ratio * 100)}>
				<div class="h-full" style:width="{ratio * 100}%" style:background="var(--color-aura-{realm?.visual.aura ?? 'jade'})"></div>
			</div>
		</div>
	{/if}

	<div class="ml-auto flex items-center gap-1.5 sm:gap-3">
		<span class="res" title="Linh thạch">
			<Icon name="stone" size={16} class="text-spirit-cyan" />
			<span class="tabular-nums">{game.stones.toLocaleString('vi-VN')}</span>
		</span>
		<span class="res hidden sm:inline-flex" title="Tụ Linh Phù">
			<Icon name="talisman" size={16} class="text-gold-500" />
			<span class="tabular-nums">{game.phuCount}</span>
		</span>
		<button type="button" class="res res-btn" onclick={() => openWindow({ kind: 'vip' })} title="Tiên Duyên">
			<Icon name="seal" size={16} class="text-gold-300" />
			<span class="hidden sm:inline">{game.vip?.benefits.name ?? 'Tiên Duyên'}</span>
			{#if game.vip && game.vip.benefits.dailyGift > 0 && !game.vip.giftClaimedToday}<span class="dot" aria-label="Có quà chưa nhận"></span>{/if}
		</button>
		<button type="button" class="relative inline-flex size-10 items-center justify-center text-mist-300 hover:text-mist-100" onclick={() => openWindow({ kind: 'notifications' })}>
			<Icon name="bell" label="Thông báo" />
			{#if pending}<span class="count tabular-nums">{pending}</span>{/if}
		</button>
		<span class="conn" class:on={feed.connected} title={feed.connected ? 'Đã kết nối truyền âm' : 'Đang kết nối lại…'}></span>

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
	.res-btn {
		position: relative;
		color: var(--color-gold-300);
	}
	.res-btn:hover {
		background: color-mix(in oklab, var(--color-gold-500) 12%, transparent);
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
