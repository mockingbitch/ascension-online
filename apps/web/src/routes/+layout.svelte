<script lang="ts">
	import '../app.css';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { logout, session } from '$lib/api/session.svelte';
	import AnnouncementTicker from '$lib/components/AnnouncementTicker.svelte';
	import Icon, { type IconName } from '$lib/components/Icon.svelte';
	import WorldBackdrop from '$lib/components/WorldBackdrop.svelte';

	let { children } = $props();

	async function signOut() {
		await logout();
		await goto('/login');
	}

	// Bottom dock. Entries without `href` are systems not built yet: shown locked, not clickable.
	const dock: { label: string; icon: IconName; href?: string }[] = [
		{ label: 'Tu Luyện', icon: 'lotus', href: '/cultivate' },
		{ label: 'Động Phủ', icon: 'cave' },
		{ label: 'Công Pháp', icon: 'scroll' },
		{ label: 'Đan Dược', icon: 'pill' },
		{ label: 'Tông Môn', icon: 'sect' },
		{ label: 'Thiên Bảng', icon: 'rank', href: '/leaderboard' },
		{ label: 'Túi Đồ', icon: 'bag' }
	];
	const mobileDock: { label: string; icon: IconName; href: string }[] = [
		{ label: 'Tu Luyện', icon: 'lotus', href: '/cultivate' },
		{ label: 'Thiên Bảng', icon: 'rank', href: '/leaderboard' },
		{ label: 'VIP', icon: 'seal', href: '/vip' }
	];
	const active = (href: string) => page.url.pathname.startsWith(href);
</script>

<WorldBackdrop />

<header class="hud sticky top-0 z-30">
	<div class="mx-auto flex h-14 max-w-7xl items-center gap-4 px-4">
		<a href="/" class="flex items-center gap-2.5" aria-label="Tiên Đạo — trang chủ">
			<svg viewBox="0 0 64 64" class="size-9 text-gold-500" aria-hidden="true">
				<circle cx="32" cy="32" r="29" fill="none" stroke="currentColor" stroke-width="2" />
				<circle cx="32" cy="32" r="24" fill="none" stroke="currentColor" stroke-width="0.8" opacity="0.6" />
				<path d="M14 42 26 24l6 8 6-12 12 22Z" fill="currentColor" opacity="0.9" />
				<path d="M18 46c6-3 10-3 14 0s8 3 14 0" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" />
			</svg>
			<span class="font-display text-2xl leading-none font-bold tracking-wide text-gold-300">Tiên Đạo</span>
		</a>

		<div class="hidden min-w-0 flex-1 md:block"><AnnouncementTicker /></div>

		<div class="ml-auto flex items-center gap-2 text-sm">
			<a
				href="/vip"
				class="hidden h-9 items-center gap-1.5 border px-3 text-gold-300 transition hover:bg-gold-500/10 md:inline-flex"
				class:border-gold-500={active('/vip')}
				class:border-gold-700={!active('/vip')}
			>
				<Icon name="seal" size={18} /> VIP
			</a>
			{#if session.user}
				<span class="hidden max-w-40 truncate px-2 text-mist-300 lg:inline">{session.user.displayName || session.user.email}</span>
				<button class="inline-flex size-10 items-center justify-center text-mist-300 hover:text-mist-100" onclick={signOut} title="Đăng xuất">
					<Icon name="exit" label="Đăng xuất" />
				</button>
			{:else}
				<a href="/login" class="btn btn-ghost">Nhập Đạo</a>
			{/if}
		</div>
	</div>
	<div class="md:hidden"><AnnouncementTicker /></div>
</header>

<main class="mx-auto max-w-7xl px-4 pt-5 pb-28 md:pb-32">
	{@render children()}
</main>

<!-- Desktop / tablet dock -->
<nav class="dock fixed bottom-4 left-1/2 z-30 hidden -translate-x-1/2 md:flex" aria-label="Điều hướng chính">
	{#each dock as d (d.label)}
		{#if d.href}
			<a href={d.href} class="dock-item" class:is-active={active(d.href)} aria-current={active(d.href) ? 'page' : undefined}>
				<Icon name={d.icon} size={24} />
				<span>{d.label}</span>
			</a>
		{:else}
			<span class="dock-item is-locked" aria-disabled="true" title="{d.label} — chưa khai mở">
				<span class="relative">
					<Icon name={d.icon} size={24} />
					<Icon name="lock" size={12} class="absolute -right-1.5 -bottom-1 text-mist-500" stroke={2} />
				</span>
				<span>{d.label}</span>
			</span>
		{/if}
	{/each}
</nav>

<!-- Mobile tab bar: only playable systems -->
<nav class="dock fixed inset-x-0 bottom-0 z-30 flex justify-around md:hidden" aria-label="Điều hướng chính">
	{#each mobileDock as d (d.href)}
		<a href={d.href} class="dock-item flex-1" class:is-active={active(d.href)} aria-current={active(d.href) ? 'page' : undefined}>
			<Icon name={d.icon} size={24} />
			<span>{d.label}</span>
		</a>
	{/each}
</nav>

<style>
	.hud {
		background: linear-gradient(180deg, color-mix(in oklab, var(--color-ink-950) 92%, transparent), color-mix(in oklab, var(--color-ink-950) 70%, transparent));
		border-bottom: 1px solid color-mix(in oklab, var(--color-gold-500) 22%, transparent);
		backdrop-filter: blur(6px);
	}
	.dock {
		gap: 2px;
		padding: 6px;
		background: var(--grain), linear-gradient(180deg, color-mix(in oklab, var(--color-ink-800) 94%, transparent), color-mix(in oklab, var(--color-ink-950) 96%, transparent));
		border: 1px solid color-mix(in oklab, var(--color-gold-500) 35%, transparent);
		box-shadow: 0 -10px 30px -12px var(--color-ink-950);
		backdrop-filter: blur(6px);
	}
	@media (min-width: 768px) {
		.dock {
			border-radius: 3px;
		}
	}
	.dock-item {
		display: flex;
		min-width: 84px;
		flex-direction: column;
		align-items: center;
		gap: 3px;
		padding: 8px 10px 6px;
		font-size: 0.8rem;
		font-weight: 500;
		color: var(--color-mist-300);
		border-bottom: 2px solid transparent;
		transition: color 200ms, background-color 200ms;
	}
	a.dock-item:hover {
		color: var(--color-mist-100);
		background: color-mix(in oklab, var(--color-gold-500) 8%, transparent);
	}
	.dock-item.is-active {
		color: var(--color-gold-300);
		border-bottom-color: var(--color-gold-500);
		background: linear-gradient(0deg, color-mix(in oklab, var(--color-gold-500) 16%, transparent), transparent);
	}
	.dock-item.is-locked {
		color: var(--color-mist-500);
		opacity: 0.6;
		cursor: not-allowed;
	}
</style>
