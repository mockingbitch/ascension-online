<!-- Notification centre: everything waiting for the player, each with its action. Derived from server state. -->
<script lang="ts">
	import { goto } from '$app/navigation';
	import { feed } from '$lib/api/realtime.svelte';
	import { screens } from '$lib/game/nav';
	import { game } from '$lib/game/store.svelte';
	import { closeWindow, openWindow } from '$lib/game/windows';
	import Icon from './Icon.svelte';

	const tasks = $derived(game.board?.tasks.filter((t) => t.complete && !t.claimed) ?? []);
	const gift = $derived(game.vip && game.vip.benefits.dailyGift > 0 && !game.vip.giftClaimedToday ? game.vip.benefits.dailyGift : 0);
	const ready = $derived(!!game.preview?.ready);

	async function go(href: string) {
		closeWindow();
		await goto(href);
	}
</script>

<ul class="space-y-2">
	{#if ready}
		<li class="note">
			<Icon name="bolt" size={20} class="text-gold-300" />
			<span class="flex-1">Bình cảnh đã viên mãn — có thể đột phá.</span>
			<button class="btn btn-gold h-9 px-3 text-sm" onclick={() => go(screens.breakthrough.href)}>Đột phá</button>
		</li>
	{/if}
	{#each tasks as t (t.id)}
		<li class="note">
			<Icon name="scroll" size={20} class="text-jade-300" />
			<span class="flex-1">Nhiệm vụ <strong class="text-mist-100">{t.name}</strong> đã hoàn thành.</span>
			<button class="btn btn-jade h-9 px-3 text-sm" disabled={game.busy} onclick={() => game.claimTask(t.id)}>Nhận</button>
		</li>
	{/each}
	{#if gift}
		<li class="note">
			<Icon name="gift" size={20} class="text-gold-500" />
			<span class="flex-1">Quà Tiên Duyên hôm nay: {gift} linh thạch.</span>
			<button class="btn btn-ghost h-9 px-3 text-sm" disabled={game.busy} onclick={game.claimGift}>Nhận</button>
		</li>
	{/if}
	{#each feed.items as a (a.id)}
		<li class="note text-mist-300">
			<Icon name="spark" size={20} class="text-gold-500" />
			<span class="flex-1">{a.data.text}</span>
		</li>
	{/each}
	{#if !ready && !tasks.length && !gift && !feed.items.length}
		<li class="py-8 text-center text-mist-500">Thiên địa tĩnh lặng — chưa có việc gì chờ bạn.</li>
	{/if}
</ul>
<button class="mt-4 inline-flex items-center gap-1 text-sm text-jade-300 hover:underline" onclick={() => openWindow({ kind: 'tasks' })}>
	Xem tất cả nhiệm vụ ngày <Icon name="arrow" size={14} />
</button>

<style>
	.note {
		display: flex;
		align-items: center;
		gap: 0.75rem;
		padding: 0.625rem 0.75rem;
		font-size: 0.875rem;
		border: 1px solid color-mix(in oklab, var(--color-gold-500) 16%, transparent);
		background: color-mix(in oklab, var(--color-ink-950) 45%, transparent);
	}
</style>
