<!-- Notification centre: everything waiting for the player, each with its action. Derived from server state. -->
<script lang="ts">
	import { feed } from '$lib/api/realtime.svelte';
	import { game } from '$lib/game/store.svelte';
	import { news } from '$lib/game/news.svelte';
	import { prominence } from '$lib/game/newsRules';
	import { openWindow } from '$lib/game/windows';
	import EmptyState from './EmptyState.svelte';
	import Icon from './Icon.svelte';

	const tasks = $derived(game.board?.tasks.filter((t) => t.complete && !t.claimed) ?? []);
	const gift = $derived(game.vip && game.vip.benefits.dailyGift > 0 && !game.vip.giftClaimedToday ? game.vip.benefits.dailyGift : 0);
	const ready = $derived(!!game.preview?.ready);
	// world / faction news from the live feed; breakthrough news from the history (GĐ-D, newest first)
	const world = $derived(feed.items.filter((a) => !a.type.startsWith('announcement.')));
	const recent = $derived(news.items.slice(0, 20));
	$effect(() => void news.load());
</script>

<ul class="stagger space-y-2">
	{#if ready}
		<li class="note">
			<Icon name="bolt" size={20} class="text-gold-300" />
			<span class="flex-1">Bình cảnh đã viên mãn — có thể đột phá.</span>
			<button class="btn btn-gold h-9 px-3 text-sm" onclick={() => openWindow({ kind: 'breakthrough' })}>Đột phá</button>
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
	{#each world as a (a.type + a.id)}
		<li class="note text-mist-300">
			<Icon name="spark" size={20} class="text-gold-500" />
			<span class="flex-1">{a.data.text}</span>
		</li>
	{/each}
	{#each recent as n (n.id)}
		<li class="note text-mist-300" data-news-item={n.kind} data-news-prominence={prominence(n)}>
			<Icon name={n.kind === 'rank_top10' ? 'rank' : 'bolt'} size={20} class={prominence(n) === 'major' ? 'text-gold-300' : 'text-gold-500'} />
			<span class="flex-1">{n.text}</span>
			<time class="shrink-0 text-xs text-mist-500 tabular-nums">{new Date(n.createdAt).toLocaleTimeString('vi-VN', { hour: '2-digit', minute: '2-digit' })}</time>
		</li>
	{/each}
	{#if !ready && !tasks.length && !gift && !world.length && !recent.length}
		<li><EmptyState icon="bell" title="Thiên địa tĩnh lặng" hint="Chưa có việc gì chờ bạn — tu luyện rồi quay lại." /></li>
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
