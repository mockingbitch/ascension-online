<!-- Out-of-game pages (landing, login, policy, public Thiên Bảng / Tiên Duyên, admin). The game itself is /play. -->
<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { ensureSession, logout, session } from '$lib/api/session.svelte';
	import AnnouncementTicker from '$lib/components/AnnouncementTicker.svelte';
	import Icon from '$lib/components/Icon.svelte';

	let { children } = $props();

	// Restore the session on public pages too (the refresh cookie is HttpOnly), so the header knows who is here.
	$effect(() => {
		void ensureSession();
	});

	async function signOut() {
		await logout();
		await goto('/login');
	}
	const active = (href: string) => page.url.pathname.startsWith(href);
	const links = [
		{ href: '/leaderboard', label: 'Thiên Bảng' },
		{ href: '/vip', label: 'Tiên Duyên' }
	];
</script>

<header class="hud sticky top-0 z-30">
	<div class="mx-auto flex h-14 max-w-7xl items-center gap-3 px-4">
		<a href="/" class="flex items-center gap-2.5" aria-label="Tiên Đạo — trang chủ">
			<svg viewBox="0 0 64 64" class="size-9 text-gold-500" aria-hidden="true">
				<circle cx="32" cy="32" r="29" fill="none" stroke="currentColor" stroke-width="2" />
				<circle cx="32" cy="32" r="24" fill="none" stroke="currentColor" stroke-width="0.8" opacity="0.6" />
				<path d="M14 42 26 24l6 8 6-12 12 22Z" fill="currentColor" opacity="0.9" />
				<path d="M18 46c6-3 10-3 14 0s8 3 14 0" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" />
			</svg>
			<span class="hidden font-display text-2xl leading-none font-bold tracking-wide text-gold-300 sm:inline">Tiên Đạo</span>
		</a>

		<nav class="flex items-center gap-1 text-sm" aria-label="Trang">
			{#each links as l (l.href)}
				<a href={l.href} class="px-2.5 py-2 text-mist-300 hover:text-mist-100" class:text-gold-300={active(l.href)} aria-current={active(l.href) ? 'page' : undefined}>{l.label}</a>
			{/each}
		</nav>

		<div class="hidden min-w-0 flex-1 md:block"><AnnouncementTicker /></div>

		<div class="ml-auto flex items-center gap-2 text-sm">
			{#if session.user}
				{#if session.user.role === 'admin'}
					<a href="/admin/reports" class="inline-flex size-10 items-center justify-center text-mist-300 hover:text-mist-100" title="Báo cáo người chơi">
						<Icon name="flag" label="Quản trị báo cáo" />
					</a>
				{/if}
				<a href="/play" class="btn btn-gold h-10 px-4">Vào game</a>
				<button class="inline-flex size-10 items-center justify-center text-mist-300 hover:text-mist-100" onclick={signOut} title="Đăng xuất">
					<Icon name="exit" label="Đăng xuất" />
				</button>
			{:else}
				<a href="/login" class="btn btn-ghost">Nhập Đạo</a>
			{/if}
		</div>
	</div>
</header>

<main class="mx-auto max-w-7xl px-4 pt-5 pb-16">
	{@render children()}
</main>

<style>
	.hud {
		background: linear-gradient(180deg, color-mix(in oklab, var(--color-ink-950) 92%, transparent), color-mix(in oklab, var(--color-ink-950) 70%, transparent));
		border-bottom: 1px solid color-mix(in oklab, var(--color-gold-500) 22%, transparent);
		backdrop-filter: blur(6px);
	}
</style>
