<!-- Bottom dock of the game shell. Entries open windows over the scene; systems not built yet are shown locked. -->
<script lang="ts">
	import { page } from '$app/state';
	import { chat } from '$lib/game/chat.svelte';
	import { game } from '$lib/game/store.svelte';
	import { closeWindow, openWindow, type WindowKind } from '$lib/game/windows';
	import Icon, { type IconName } from './Icon.svelte';

	type Entry = { label: string; icon: IconName; win?: WindowKind | 'home'; badge?: number; mobileOnly?: boolean; desktopOnly?: boolean };
	const entries: Entry[] = $derived([
		{ label: 'Tu Luyện', icon: 'lotus', win: 'home' },
		{ label: 'Nhiệm Vụ', icon: 'scroll', win: 'tasks', badge: game.claimable },
		{ label: 'Truyền Âm', icon: 'chat', win: 'chat', badge: chat.unread, mobileOnly: true },
		{ label: 'Thiên Bảng', icon: 'rank', win: 'leaderboard' },
		{ label: 'Tiên Duyên', icon: 'seal', win: 'vip' },
		{ label: 'Động Phủ', icon: 'cave', desktopOnly: true },
		{ label: 'Đan Dược', icon: 'pill', desktopOnly: true },
		{ label: 'Tông Môn', icon: 'sect', desktopOnly: true },
		{ label: 'Túi Đồ', icon: 'bag', desktopOnly: true }
	]);
	const current = $derived(page.state.win?.kind ?? 'home');

	function go(win: WindowKind | 'home') {
		if (win === 'home') closeWindow();
		else if (win === current) closeWindow();
		else openWindow({ kind: win } as Parameters<typeof openWindow>[0]);
	}
</script>

<nav class="dock flex shrink-0 justify-around md:justify-center md:gap-1" aria-label="Điều hướng trong game">
	{#each entries as d (d.label)}
		{#if d.win}
			<button
				type="button"
				class={['dock-item', d.mobileOnly && 'mobile-only', current === d.win && 'is-active']}
				aria-current={current === d.win ? 'page' : undefined}
				onclick={() => go(d.win!)}
			>
				<span class="relative">
					<Icon name={d.icon} size={24} />
					{#if d.badge}<span class="badge tabular-nums">{d.badge > 9 ? '9+' : d.badge}</span>{/if}
				</span>
				<span>{d.label}</span>
			</button>
		{:else}
			<span class="dock-item is-locked desktop-only" aria-disabled="true" title="{d.label} — chưa khai mở">
				<span class="relative">
					<Icon name={d.icon} size={24} />
					<Icon name="lock" size={12} class="absolute -right-1.5 -bottom-1 text-mist-500" stroke={2} />
				</span>
				<span>{d.label}</span>
			</span>
		{/if}
	{/each}
</nav>

<style>
	.dock {
		padding: 4px 4px max(4px, env(safe-area-inset-bottom));
		background: var(--grain), linear-gradient(180deg, color-mix(in oklab, var(--color-ink-800) 94%, transparent), color-mix(in oklab, var(--color-ink-950) 98%, transparent));
		border-top: 1px solid color-mix(in oklab, var(--color-gold-500) 35%, transparent);
	}
	.dock-item {
		display: flex;
		min-width: 64px;
		flex-direction: column;
		align-items: center;
		gap: 2px;
		padding: 6px 8px 4px;
		font-size: 0.75rem;
		font-weight: 500;
		color: var(--color-mist-300);
		border-bottom: 2px solid transparent;
		transition: color 200ms, background-color 200ms;
	}
	@media (min-width: 768px) {
		.dock-item {
			min-width: 84px;
			font-size: 0.8rem;
		}
	}
	button.dock-item:hover {
		color: var(--color-mist-100);
		background: color-mix(in oklab, var(--color-gold-500) 8%, transparent);
	}
	.dock-item.is-active {
		color: var(--color-gold-300);
		border-bottom-color: var(--color-gold-500);
		background: linear-gradient(0deg, color-mix(in oklab, var(--color-gold-500) 16%, transparent), transparent);
	}
	/* Scoped display rules (utility classes like `hidden` lose to .dock-item's display). */
	.desktop-only {
		display: none;
	}
	@media (min-width: 768px) {
		.desktop-only {
			display: flex;
		}
	}
	@media (min-width: 1024px) {
		.mobile-only {
			display: none;
		}
	}
	.dock-item.is-locked {
		color: var(--color-mist-500);
		opacity: 0.6;
		cursor: not-allowed;
	}
	.badge {
		position: absolute;
		top: -6px;
		right: -10px;
		min-width: 18px;
		height: 18px;
		padding: 0 4px;
		border-radius: 9999px;
		font-size: 0.7rem;
		font-weight: 700;
		line-height: 18px;
		text-align: center;
		color: var(--color-mist-100);
		background: var(--color-cinnabar-500);
	}
</style>
