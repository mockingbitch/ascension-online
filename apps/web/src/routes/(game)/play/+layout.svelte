<!--
	Game shell: a fixed screen that never scrolls. HUD, world chat (docked on desktop, peek strip on mobile), dock,
	toasts and windows stay mounted; only the centre swaps between screens (nested routes), with a short fade.
	All numbers come from the server via lib/game/store; screens on sample data are labelled.
-->
<script lang="ts">
	import { page } from '$app/state';
	import { MediaQuery } from 'svelte/reactivity';
	import { fade } from 'svelte/transition';
	import { feed, startRealtime } from '$lib/api/realtime.svelte';
	import BreakthroughCelebration from '$lib/components/BreakthroughCelebration.svelte';
	import ChatPanel from '$lib/components/ChatPanel.svelte';
	import DailyTasks from '$lib/components/DailyTasks.svelte';
	import GameDock from '$lib/components/GameDock.svelte';
	import GameHud from '$lib/components/GameHud.svelte';
	import GameWindow from '$lib/components/GameWindow.svelte';
	import Icon from '$lib/components/Icon.svelte';
	import NotificationList from '$lib/components/NotificationList.svelte';
	import ProfileWindow from '$lib/components/ProfileWindow.svelte';
	import Toaster from '$lib/components/Toaster.svelte';
	import VipWindow from '$lib/components/VipWindow.svelte';
	import { chat } from '$lib/game/chat.svelte';
	import { game } from '$lib/game/store.svelte';
	import { closeWindow, openWindow } from '$lib/game/windows';

	let { data, children } = $props();

	// Synchronously, before child screens render (they read game.character / game.catalogue), then on data changes.
	// svelte-ignore state_referenced_locally
	game.init(data.catalogue, data.character!);
	$effect(() => game.init(data.catalogue, data.character!));
	$effect(() => startRealtime(true));

	const cat = $derived(data.catalogue);
	const win = $derived(page.state.win);
	const wide = new MediaQuery('min-width: 1024px'); // chat docked beside the screen
	$effect(() => {
		if (wide.current && win?.kind === 'chat') closeWindow(); // resized to desktop: chat is already visible
	});
	const lastChat = $derived(chat.messages.at(-1));
	const titles: Record<string, string> = {
		tasks: 'Nhiệm vụ ngày',
		vip: 'Tiên Duyên',
		notifications: 'Thông báo',
		chat: 'Truyền âm · Thế Giới'
	};
</script>

<div class="shell flex h-dvh flex-col overflow-hidden">
	<GameHud />
	{#if feed.items[0]}
		<p class="ticker flex shrink-0 items-center gap-2 truncate px-3 py-1 text-sm text-gold-300" aria-live="polite">
			<Icon name="bolt" size={14} class="shrink-0 text-gold-500" />
			<span class="truncate">{feed.items[0].data.text}</span>
		</p>
	{/if}

	<div class="main grid min-h-0 flex-1 gap-3 p-2 sm:p-3 lg:grid-cols-[minmax(0,1fr)_340px]">
		<div class="screen relative min-h-0">
			{#key page.url.pathname}
				<div class="absolute inset-0" in:fade={{ duration: 220 }}>
					{@render children()}
				</div>
			{/key}
		</div>
		{#if wide.current}
			<aside class="panel flex min-h-0 flex-col"><ChatPanel /></aside>
		{/if}
	</div>

	{#if !wide.current}
		<!-- Mobile/tablet: latest world message, tap to open the channel -->
		<button type="button" class="chat-peek flex shrink-0 items-center gap-2 px-3 py-1.5 text-left text-sm" onclick={() => openWindow({ kind: 'chat' })}>
			<Icon name="chat" size={16} class="shrink-0 text-gold-500" />
			{#if lastChat}
				<span class="min-w-0 flex-1 truncate">
					<span class="font-semibold" style:color="var(--color-aura-{cat.realmOf(lastChat.stageId)?.visual.aura ?? 'none'})">{lastChat.name}</span><span class="text-mist-500">:</span>
					<span class="text-mist-100">{lastChat.text}</span>
				</span>
			{:else}
				<span class="flex-1 text-mist-500">Kênh Thế Giới — chạm để trò chuyện</span>
			{/if}
			{#if chat.unread}<span class="badge tabular-nums">{chat.unread > 9 ? '9+' : chat.unread}</span>{/if}
		</button>
	{/if}

	<GameDock />
</div>

<Toaster />

<GameWindow open={!!win} title={win ? (win.kind === 'profile' ? win.name : (titles[win.kind] ?? '')) : ''} size={win?.kind === 'vip' ? 'lg' : 'md'} onclose={closeWindow}>
	{#if win?.kind === 'tasks'}
		<DailyTasks board={game.board} catalogue={cat} busy={game.busy} onclaim={game.claimTask} />
	{:else if win?.kind === 'vip'}
		<VipWindow />
	{:else if win?.kind === 'notifications'}
		<NotificationList />
	{:else if win?.kind === 'chat' && !wide.current}
		<div class="-mx-5 -my-4 h-[70dvh]"><ChatPanel compact /></div>
	{:else if win?.kind === 'profile'}
		{#key win.characterId}
			<ProfileWindow characterId={win.characterId} name={win.name} stageId={win.stageId} messageId={win.messageId} />
		{/key}
	{/if}
</GameWindow>

{#if game.celebrate}
	{@const c = game.celebrate}
	{@const r = cat.realmOf(c.to)}
	<BreakthroughCelebration
		realmName={r?.name ?? ''}
		stageName={cat.stages.get(c.to)?.name ?? ''}
		aura={r?.visual.aura ?? 'jade'}
		major={c.kind === 'major'}
		tribulation={c.tribulation ? (game.preview?.tribulation?.name ?? 'Lôi Kiếp') : undefined}
		onclose={() => (game.celebrate = null)}
	/>
{/if}

<style>
	.shell {
		overscroll-behavior: none;
		user-select: none;
	}
	.shell :global(input),
	.shell :global(textarea),
	.shell :global(.msg) {
		user-select: text;
	}
	.ticker {
		background: linear-gradient(90deg, transparent, color-mix(in oklab, var(--color-gold-500) 10%, transparent) 15%, color-mix(in oklab, var(--color-gold-500) 10%, transparent) 85%, transparent);
	}
	.chat-peek {
		border-top: 1px solid color-mix(in oklab, var(--color-gold-500) 18%, transparent);
		background: color-mix(in oklab, var(--color-ink-950) 75%, transparent);
	}
	.badge {
		min-width: 20px;
		height: 20px;
		padding: 0 5px;
		border-radius: 9999px;
		font-size: 0.7rem;
		font-weight: 700;
		line-height: 20px;
		text-align: center;
		color: var(--color-mist-100);
		background: var(--color-cinnabar-500);
	}
</style>
