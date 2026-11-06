<!-- Bottom navigation of the game shell (screens are nested routes under /play; the shell stays mounted). -->
<script lang="ts">
	import { page } from '$app/state';
	import { dockScreens, screens } from '$lib/game/nav';
	import { game } from '$lib/game/store.svelte';
	import Icon from './Icon.svelte';

	const path = $derived(page.url.pathname);
	const isActive = (href: string) => (href === screens.world.href ? path === href : path.startsWith(href));
	// Small red dot when something on that screen wants attention.
	const badge = (id: string) => (id === 'cultivate' && (game.claimable > 0 || game.preview?.ready) ? 1 : 0);
</script>

<nav class="dock grid shrink-0 grid-cols-6 md:flex md:justify-center md:gap-1" aria-label="Điều hướng trong game">
	{#each dockScreens as s (s.id)}
		<a href={s.href} class="dock-item" class:is-active={isActive(s.href)} aria-current={isActive(s.href) ? 'page' : undefined} data-sveltekit-noscroll>
			<span class="relative">
				<Icon name={s.icon} size={24} />
				{#if badge(s.id)}<span class="dot" aria-label="Có việc cần làm"></span>{/if}
			</span>
			<span class="truncate">{s.label}</span>
		</a>
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
		min-width: 0;
		flex-direction: column;
		align-items: center;
		gap: 2px;
		padding: 6px 4px 4px;
		font-size: 0.72rem;
		font-weight: 500;
		color: var(--color-mist-300);
		border-bottom: 2px solid transparent;
		transition: color 200ms, background-color 200ms;
	}
	@media (min-width: 768px) {
		.dock-item {
			min-width: 92px;
			font-size: 0.8rem;
		}
	}
	.dock-item:hover {
		color: var(--color-mist-100);
		background: color-mix(in oklab, var(--color-gold-500) 8%, transparent);
	}
	.dock-item.is-active {
		color: var(--color-gold-300);
		border-bottom-color: var(--color-gold-500);
		background: linear-gradient(0deg, color-mix(in oklab, var(--color-gold-500) 16%, transparent), transparent);
	}
	.dot {
		position: absolute;
		top: -2px;
		right: -4px;
		width: 9px;
		height: 9px;
		border-radius: 9999px;
		background: var(--color-cinnabar-500);
		box-shadow: 0 0 0 2px var(--color-ink-900);
	}
</style>
