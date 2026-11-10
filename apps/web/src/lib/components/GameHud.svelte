<!-- Top HUD of the game shell: who I am, world progress, resources, Tiên Duyên, connection, menu. Currency chips are
     icon + number and open the related window (Linh thạch → Túi Đồ › Cửa hàng, Tụ Linh Phù → Túi Đồ).
     GĐ4 (§4.4): tu vi and linh thạch count up to each new server value (held while orbs/items fly to them); the tu vi
     bar shimmers while accumulating, pulses gold when full (bình cảnh) and shakes when a settle hit today's ceiling.
     All of it display only — the numbers and aria values are the server's.
     UI polish U1: the gear menu is a popover (top layer, popover="auto"): it closes on an outside press, Esc, or when a
     window opens (showModal closes auto popovers), focus returns to the gear, and while closed it is not rendered
     (nothing in the hit-test tree). Targets are ≥ 44px on touch. -->
<script lang="ts">
	import { goto } from '$app/navigation';
	import { feed } from '$lib/api/realtime.svelte';
	import { logout, session } from '$lib/api/session.svelte';
	import { factionColor, factionOf, factions } from '$lib/game/faction.svelte';
	import { formatPoints, progressRatio } from '$lib/game/format';
	import { idleClock } from '$lib/game/idle.svelte';
	import { game } from '$lib/game/store.svelte';
	import { fullscreen } from '$lib/engine/fullscreen.svelte';
	import { openWindow } from '$lib/game/windows';
	import { hudFx } from '$lib/engine/fx/hud.svelte';
	import CountUp from './CountUp.svelte';
	import FactionEmblem from './FactionEmblem.svelte';
	import Icon from './Icon.svelte';
	import Portrait from './Portrait.svelte';
	import WorldProgress from './WorldProgress.svelte';

	const ch = $derived(game.character);
	const realm = $derived(ch && game.catalogue ? game.catalogue.realmOf(ch.stageId) : undefined);
	const faction = $derived(factionOf(factions.me?.factionId));
	let menu = $state<HTMLElement>();
	let gear = $state<HTMLButtonElement>();
	let menuOpen = $state(false);
	// place the popover under the gear (top layer: fixed coordinates, clamped to the viewport)
	function place(e: ToggleEvent) {
		menuOpen = e.newState === 'open';
		if (!menuOpen || !menu || !gear) return;
		const r = gear.getBoundingClientRect();
		menu.style.top = `${Math.round(r.bottom + 4)}px`;
		menu.style.right = `${Math.max(8, Math.round(innerWidth - r.right))}px`;
	}
	function closeMenu() {
		if (menu?.matches(':popover-open')) menu.hidePopover();
	}
	$effect(() => {
		if (menuOpen) menu?.querySelector<HTMLElement>('[role=menuitem]')?.focus({ preventScroll: true });
	});
	function menuKeys(e: KeyboardEvent) {
		if (e.key !== 'ArrowDown' && e.key !== 'ArrowUp') return;
		e.preventDefault();
		const items = [...(menu?.querySelectorAll<HTMLElement>('[role=menuitem]') ?? [])];
		const i = items.indexOf(document.activeElement as HTMLElement);
		items[(i + (e.key === 'ArrowDown' ? 1 : items.length - 1)) % items.length]?.focus();
	}
	const pending = $derived(game.claimable + (game.preview?.ready ? 1 : 0) + (game.vip && game.vip.benefits.dailyGift > 0 && !game.vip.giftClaimedToday ? 1 : 0));
	// A9: the same display value as Động Phủ's card (one shared idle clock + CountUp), never a second interpolation
	const qiShown = $derived(idleClock.shown);
	const ratio = $derived(ch ? progressRatio(qiShown, ch.requiredMilli) : 0);
	// while orbs fly to the bar it stays where it was, then grows when they land (display only)
	let heldRatio = $state(0);
	$effect.pre(() => {
		const r = ratio;
		if (!hudFx.held('qi')) heldRatio = r;
	});
	// What the bar leads to: the next tầng, or the next realm when this is the bình cảnh stage.
	const target = $derived.by(() => {
		if (!ch || !game.catalogue) return '';
		const next = game.catalogue.nextStage(ch.stageId);
		if (!next) return '';
		const label = game.catalogue.stageLabel(next.id);
		return next.realmId !== ch.realmId ? `đột phá ${label}` : label;
	});
	// bar state (server snapshot): full = bình cảnh reached; accumulating = idle qi still has room today
	const atBottleneck = $derived(idleClock.atBottleneck);
	const accumulating = $derived(
		!!ch && !atBottleneck && ch.idleRateMilliPerHour > 0 && ch.today.pointsMilli < ch.today.ceilingMilli && ch.bottleneckRoomMilli > 0
	);
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
				<Portrait aura={realm?.visual.aura ?? 'none'} size={40} frameId={ch.appearance?.avatarFrameId} look={ch.appearance?.look} fxTier={realm?.visual.fxTier} mine />
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
						<button type="button" class="faction hidden shrink-0 items-center gap-1 sm:inline-flex" style:color={factionColor(faction)} title="Thế lực: {faction.name}" onclick={() => openWindow({ kind: 'faction' })}>
							<FactionEmblem id={faction.id} size={14} />{faction.name}
						</button>
					{/if}
					<button type="button" class="vip-badge hidden sm:inline-flex" onclick={() => openWindow({ kind: 'vip' })} title="Tiên Duyên — xem đặc quyền">
						<Icon name="seal" size={11} />
						{game.vip?.benefits.name ?? 'Tiên Duyên'}
						{#if game.vip && game.vip.benefits.dailyGift > 0 && !game.vip.giftClaimedToday}<span class="dot" aria-label="Có quà chưa nhận"></span>{/if}
					</button>
				</span>
			</span>
		</div>

		<!-- tu vi: the one progress the whole game is about, centred -->
		<div class="qi hidden w-[min(22rem,30vw)] shrink-0 md:block" title="Tu vi tầng hiện tại (số liệu máy chủ)" data-fx-target="qi" data-tutorial="qi">
			<div class="flex items-baseline justify-between gap-2 text-xs">
				<span class="text-mist-300"
					>Tu vi <strong class="text-sm font-semibold text-mist-100 tabular-nums"
						><CountUp value={qiShown} format={formatPoints} hold={hudFx.held('qi')} />/{formatPoints(ch.requiredMilli)}</strong
					></span
				>
				{#if atBottleneck}<span class="truncate text-gold-300">Bình cảnh</span>{:else if target}<span class="truncate text-mist-300">→ {target}</span>{/if}
			</div>
			{#key hudFx.capShake}
				<div
					class="qi-track mt-1"
					class:is-shake={hudFx.capShake > 0}
					class:is-full={atBottleneck}
					class:is-flowing={accumulating}
					role="progressbar"
					aria-label="Tu vi tầng hiện tại"
					aria-valuemin={0}
					aria-valuemax={100}
					aria-valuenow={Math.round(ratio * 100)}
					data-hud-qi
				>
					<div class="qi-fill" style:width="{(hudFx.held('qi') ? heldRatio : ratio) * 100}%"><span class="shimmer" aria-hidden="true"></span></div>
				</div>
			{/key}
		</div>
	{/if}

	<div class="flex shrink-0 items-center justify-end gap-2 sm:flex-1 sm:gap-3">
		<WorldProgress />
		<button
			type="button"
			class="res press"
			title="Linh thạch — mở Cửa hàng"
			aria-label="Linh thạch: {game.stones.toLocaleString('vi-VN')} — mở Cửa hàng"
			onclick={() => openWindow({ kind: 'inventory', tab: 'shop' })}
			data-chip="stones"
		>
			<Icon name="stone" size={16} class="text-spirit-cyan" />
			<CountUp value={game.stones} hold={hudFx.held('stones')} />
		</button>
		<button
			type="button"
			class="res press hidden sm:inline-flex"
			title={phuTitle}
			aria-label="{phuName}: {game.phuCount} — mở Túi Đồ"
			onclick={() => openWindow({ kind: 'inventory', tab: 'bag' })}
			data-chip="phu"
		>
			<Icon name="talisman" size={16} class="text-gold-500" />
			<span class="tabular-nums" aria-hidden="true">{game.phuCount}</span>
		</button>
		<button type="button" class="hbtn relative" onclick={() => openWindow({ kind: 'notifications' })}>
			<Icon name="bell" label="Thông báo" />
			{#if pending}{#key pending}<span class="count badge-pop tabular-nums">{pending}</span>{/key}{/if}
		</button>
		<span
			class="conn"
			class:on={feed.connected}
			role="status"
			title={feed.connected ? 'Trực tuyến — đã kết nối truyền âm' : 'Mất kết nối — đang kết nối lại…'}
			aria-label={feed.connected ? 'Trực tuyến' : 'Đang kết nối lại'}
		></span>

		<button
			bind:this={gear}
			type="button"
			class="hbtn"
			popovertarget="hud-menu"
			aria-haspopup="menu"
			aria-expanded={menuOpen}
			data-hud-menu
		>
			<Icon name="gear" label="Tùy chọn" />
		</button>
		<!-- svelte-ignore a11y_no_noninteractive_element_interactions (arrow keys move between the menu items) -->
		<div bind:this={menu} id="hud-menu" class="hud-menu panel p-1.5 text-sm" popover="auto" role="menu" tabindex="-1" ontoggle={place} onkeydown={menuKeys}>
			{#if menuOpen}
				<button
					role="menuitem"
					type="button"
					class="menu-item w-full text-left"
					onclick={() => {
						closeMenu();
						openWindow({ kind: 'settings' });
					}}
				>
					<Icon name="gear" size={16} /> Cài đặt
				</button>
				{#if fullscreen.supported}
					<button
						role="menuitem"
						type="button"
						class="menu-item w-full text-left"
						data-fullscreen
						onclick={() => {
							closeMenu();
							void fullscreen.toggle();
						}}
					>
						<Icon name={fullscreen.active ? 'shrink' : 'expand'} size={16} />
						{fullscreen.active ? 'Thoát toàn màn hình' : 'Toàn màn hình'}
					</button>
				{/if}
				<a role="menuitem" class="menu-item" href="/chinh-sach-du-lieu" onclick={closeMenu}>Chính sách dữ liệu</a>
				{#if session.user?.role === 'admin'}
					<a role="menuitem" class="menu-item" href="/admin/reports" onclick={closeMenu}>Quản trị báo cáo</a>
				{/if}
				<button role="menuitem" type="button" class="menu-item w-full text-left text-cinnabar-300" onclick={signOut}>
					<Icon name="exit" size={16} /> Đăng xuất
				</button>
			{/if}
		</div>
	</div>
</header>

<style>
	.hud {
		/* layer: the shell's HUD region (--z-hud); the gear menu is a top-layer popover */
		position: relative;
		background: linear-gradient(180deg, color-mix(in oklab, var(--color-ink-950) 94%, transparent), color-mix(in oklab, var(--color-ink-950) 72%, transparent));
		border-bottom: 1px solid color-mix(in oklab, var(--color-gold-500) 22%, transparent);
		backdrop-filter: blur(6px);
	}
	.res {
		display: inline-flex;
		height: var(--tap);
		align-items: center;
		gap: 0.375rem;
		padding: 0 0.625rem;
		font-size: 0.875rem;
		font-weight: 600;
		color: var(--color-mist-100);
		border: 1px solid color-mix(in oklab, var(--color-gold-500) 22%, transparent);
		background: color-mix(in oklab, var(--color-ink-950) 55%, transparent);
		border-radius: 2px;
		transition: border-color var(--dur-fast);
	}
	.res:hover {
		border-color: color-mix(in oklab, var(--color-gold-300) 55%, transparent);
	}
	.vip-badge {
		/* display from the classes: phones reach Tiên Duyên through "Thêm" (an 18px chip is no tap target) */
		position: relative;
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
		position: relative;
		height: 100%;
		overflow: hidden;
		border-radius: inherit;
		background: linear-gradient(90deg, var(--color-jade-700), var(--color-jade-500) 45%, var(--color-spirit-cyan));
		box-shadow: 0 0 8px color-mix(in oklab, var(--color-spirit-cyan) 55%, transparent);
		transition: width 600ms var(--ease-qi);
	}
	.is-full .qi-fill {
		background: linear-gradient(90deg, var(--color-gold-700), var(--color-gold-300));
	}
	/* running shimmer while qi accumulates (transform only) */
	.shimmer {
		position: absolute;
		inset: 0;
		width: 35%;
		background: linear-gradient(90deg, transparent, color-mix(in oklab, var(--color-mist-100) 45%, transparent), transparent);
		opacity: 0;
	}
	:global(:root[data-fx='full']) .is-flowing .shimmer {
		opacity: 1;
		animation: shimmer 2.6s linear infinite;
	}
	:global(:root[data-fx='lite']) .is-flowing .shimmer {
		opacity: 0.6;
		animation: shimmer 5s linear infinite;
	}
	@keyframes shimmer {
		from {
			transform: translateX(-110%);
		}
		to {
			transform: translateX(300%);
		}
	}
	/* bình cảnh: the bar pulses (a glow layer's opacity) */
	.qi-track {
		position: relative;
	}
	.is-full::after {
		content: '';
		position: absolute;
		inset: 0;
		border-radius: 9999px;
		background: color-mix(in oklab, var(--color-mist-100) 45%, transparent);
		opacity: 0.6;
		pointer-events: none;
	}
	:global(:root[data-fx='full']) .is-full::after,
	:global(:root[data-fx='lite']) .is-full::after {
		animation: full-pulse 1.8s var(--ease-in-out-sine) infinite;
	}
	@keyframes full-pulse {
		50% {
			opacity: 0.1;
		}
	}
	/* daily ceiling hit: one light shake */
	:global(:root[data-fx='full']) .is-shake,
	:global(:root[data-fx='lite']) .is-shake {
		animation: cap-shake 360ms var(--ease-qi) 1;
	}
	@keyframes cap-shake {
		20% {
			transform: translateX(-3px);
		}
		45% {
			transform: translateX(3px);
		}
		70% {
			transform: translateX(-2px);
		}
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
	.hbtn {
		display: inline-flex;
		width: var(--tap);
		height: var(--tap);
		flex-shrink: 0;
		align-items: center;
		justify-content: center;
		color: var(--color-mist-300);
	}
	.hbtn:hover {
		color: var(--color-mist-100);
	}
	.hud-menu {
		position: fixed;
		inset: auto;
		margin: 0;
		width: 14rem;
		color: inherit;
		background-color: var(--color-ink-900);
	}
	.hud-menu:not(:popover-open) {
		display: none;
	}
	.menu-item {
		display: flex;
		align-items: center;
		gap: 0.5rem;
		min-height: var(--tap);
		padding: 0 0.75rem;
		color: var(--color-mist-100);
	}
	.menu-item:hover {
		background: color-mix(in oklab, var(--color-gold-500) 10%, transparent);
	}
</style>
