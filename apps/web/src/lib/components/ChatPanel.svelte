<!--
	The one chat box of the game shell. Tabs: "Luận Đạo" (Tử Tiêu Cung hall channel — only while the hall screen is
	open) and "Thế Giới" (world). History (world: scroll up for older), live messages, composer; names open the
	player card. The posting gate shown here is a display hint from the catalogue; the server enforces it.
-->
<script lang="ts">
	import NameFx from './NameFx.svelte';
	import { tick } from 'svelte';
	import { chat, loadOlder, markChatVisible, sendChat } from '$lib/game/chat.svelte';
	import { hall, markHallChatVisible } from '$lib/game/lecture.svelte';
	import { game } from '$lib/game/store.svelte';
	import { news } from '$lib/game/news.svelte';
	import { chatLines, type ChatLine } from '$lib/game/newsRules';
	import { openWindow } from '$lib/game/windows';
	import type { ChatMessage } from '$lib/game/types';
	import Icon from './Icon.svelte';

	let { compact = false }: { compact?: boolean } = $props();

	type Tab = 'room' | 'world';
	let picked = $state<Tab>('room');
	const tab = $derived<Tab>(hall.active && picked === 'room' ? 'room' : 'world');

	let list = $state<HTMLOListElement>();
	let input = $state<HTMLInputElement>();
	let stick = true; // follow new messages while the user is at the bottom

	const cat = $derived(game.catalogue);
	const me = $derived(game.character);
	const messages = $derived(tab === 'room' ? hall.messages : chat.messages);
	// world tab: other players' news as client-side system lines (shown, never stored as chat — GĐ-D)
	const lines = $derived<ChatLine[]>(tab === 'world' ? chatLines(chat.messages, news.items) : messages.map((m) => ({ kind: 'msg' as const, key: `m${m.id}`, at: 0, msg: m })));
	const loaded = $derived(tab === 'room' ? hall.chatLoaded : chat.loaded);
	const channel = $derived(tab === 'room' ? hall.chatChannel : chat.channel);
	const draft = $derived(tab === 'room' ? hall.draft : chat.draft);
	const sending = $derived(tab === 'room' ? hall.sending : chat.sending);
	const error = $derived(tab === 'room' ? hall.chatError : chat.error);
	const maxChars = $derived(tab === 'room' ? hall.maxChars : chat.maxChars);
	const placeholder = $derived(tab === 'room' ? 'Luận đạo cùng đạo hữu trong điện…' : 'Truyền âm cho thiên hạ…');
	const empty = $derived(tab === 'room' ? 'Điện còn tĩnh lặng. Hãy là người đầu tiên luận đạo.' : 'Chưa ai lên tiếng. Hãy là người đầu tiên chào đạo hữu.');

	const minStage = $derived(channel && cat ? cat.stages.get(channel.minStage) : undefined);
	const myStage = $derived(me && cat ? cat.stages.get(me.stageId) : undefined);
	// Display hint only (same path compare by tier/order; the server decides).
	const canPost = $derived(
		!minStage || !myStage || myStage.tier > minStage.tier || (myStage.tier === minStage.tier && myStage.stageOrder >= minStage.stageOrder)
	);

	$effect(() => {
		markChatVisible(tab === 'world');
		markHallChatVisible(tab === 'room');
		return () => {
			markChatVisible(false);
			markHallChatVisible(false);
		};
	});

	// Auto-scroll on new messages / tab switch when pinned to the bottom.
	$effect(() => {
		void lines.length;
		void tab;
		if (!list || !stick) return;
		void tick().then(() => list && (list.scrollTop = list.scrollHeight));
	});

	async function onscroll() {
		if (!list) return;
		stick = list.scrollHeight - list.scrollTop - list.clientHeight < 40;
		if (tab === 'world' && list.scrollTop < 60 && chat.hasOlder && !chat.loadingOlder) {
			const before = list.scrollHeight;
			await loadOlder();
			await tick();
			list.scrollTop += list.scrollHeight - before; // keep position after prepending
		}
	}

	function setDraft(v: string) {
		if (tab === 'room') hall.draft = v;
		else chat.draft = v;
	}

	async function submit(e: SubmitEvent) {
		e.preventDefault();
		const t = draft.trim();
		if (!t) return;
		stick = true;
		if (tab === 'room') {
			if (await hall.sendChat(t)) hall.draft = '';
		} else if (await sendChat(t)) chat.draft = '';
		input?.focus();
	}

	function pick(t: Tab) {
		picked = t;
		stick = true;
	}

	const time = (iso: string) => new Date(iso).toLocaleTimeString('vi-VN', { hour: '2-digit', minute: '2-digit' });
	const auraOf = (stageId: string) => cat?.realmOf(stageId)?.visual.aura ?? 'none';
	const openProfile = (m: ChatMessage) =>
		openWindow({ kind: 'profile', characterId: m.characterId, name: m.name, stageId: m.stageId, messageId: m.id, nameFx: m.nameFx, look: m.look });
</script>

<section class="flex h-full min-h-0 flex-col" aria-label="Kênh trò chuyện">
	{#if hall.active}
		<div class="tabs flex shrink-0" role="tablist" aria-label="Kênh trò chuyện">
			{#each [{ id: 'room', label: 'Luận Đạo', unread: hall.unread }, { id: 'world', label: 'Thế Giới', unread: chat.unread }] as t (t.id)}
				<button
					type="button"
					role="tab"
					class="tab"
					aria-selected={tab === t.id}
					aria-controls="chat-log"
					onclick={() => pick(t.id as Tab)}
				>
					{t.label}
					{#if t.unread && tab !== t.id}<span class="badge tabular-nums">{t.unread > 9 ? '9+' : t.unread}</span>{/if}
				</button>
			{/each}
		</div>
	{:else if !compact}
		<header class="flex items-center gap-2 border-b border-gold-500/15 px-4 py-2.5">
			<h2 class="flex-1 font-display text-lg font-bold text-gold-300">Kênh {chat.channel?.name ?? 'Thế Giới'}</h2>
		</header>
	{/if}

	<ol
		id="chat-log"
		bind:this={list}
		{onscroll}
		class="min-h-0 flex-1 space-y-1.5 overflow-y-auto overscroll-contain px-3 py-2 text-sm"
		aria-live="polite"
		aria-relevant="additions"
	>
		{#if tab === 'world' && chat.loadingOlder}<li class="py-1 text-center text-xs text-mist-500">Đang tải tin cũ…</li>{/if}
		{#if loaded && messages.length === 0}
			<li class="py-6 text-center text-mist-300">{empty}</li>
		{:else if !loaded}
			<li class="py-6 text-center text-mist-300">Đang kết nối truyền âm…</li>
		{/if}
		{#each lines as l (l.key)}
			{#if l.kind === 'news'}
				<li class="sys leading-snug break-words" data-chat-news={l.news.kind}>
					<span class="mr-1 text-xs text-mist-500 tabular-nums">{time(l.news.createdAt)}</span>
					<Icon name={l.news.kind === 'rank_top10' ? 'rank' : 'bolt'} size={12} class="mr-0.5 inline align-[-1px] text-gold-500" />
					<span class="text-gold-300">{l.news.text}</span>
				</li>
			{:else}
			{@const m = l.msg}
			<li class="msg leading-snug break-words" class:mine={m.characterId === me?.id}>
				<span class="mr-1 text-xs text-mist-500 tabular-nums">{time(m.sentAt)}</span>
				<button type="button" class="name font-semibold hover:underline" style:color="var(--color-aura-{auraOf(m.stageId)})" onclick={() => openProfile(m)}>
					<NameFx fx={m.nameFx}>{m.name}</NameFx>
				</button><span class="text-mist-500">:</span>
				<span class="text-mist-100">{m.text}</span>
			</li>
			{/if}
		{/each}
	</ol>

	<form onsubmit={submit} class="border-t border-gold-500/15 p-2.5">
		{#if canPost}
			<div class="flex gap-2">
				<input
					bind:this={input}
					value={draft}
					oninput={(e) => setDraft(e.currentTarget.value)}
					maxlength={maxChars}
					autocomplete="off"
					enterkeyhint="send"
					{placeholder}
					aria-label={tab === 'room' ? 'Tin nhắn luận đạo trong Tử Tiêu Cung' : 'Tin nhắn kênh Thế Giới'}
					class="h-10 min-w-0 flex-1 rounded-[2px] border border-ink-600 bg-ink-950/60 px-3 text-mist-100 outline-none placeholder:text-mist-500 focus:border-gold-500"
				/>
				<button class="send" disabled={sending || !draft.trim()} aria-label="Gửi">
					<Icon name="arrow" size={18} />
				</button>
			</div>
			{#if error}<p role="alert" class="mt-1.5 text-xs text-cinnabar-300">{error}</p>{/if}
		{:else}
			<p class="flex items-center gap-2 px-1 py-2 text-xs text-mist-300">
				<Icon name="lock" size={14} /> Đạt {minStage?.name ?? 'cảnh giới yêu cầu'} để {tab === 'room' ? 'luận đạo' : 'truyền âm'}. Bạn vẫn đọc được mọi tin.
			</p>
		{/if}
	</form>
</section>

<style>
	.tabs {
		border-bottom: 1px solid color-mix(in oklab, var(--color-gold-500) 15%, transparent);
	}
	.tab {
		display: inline-flex;
		flex: 1;
		align-items: center;
		justify-content: center;
		gap: 0.4rem;
		min-height: 2.75rem;
		font-family: var(--font-display);
		font-size: 1.05rem;
		font-weight: 700;
		color: var(--color-mist-300);
		border-bottom: 2px solid transparent;
		transition:
			color 150ms,
			border-color 150ms;
	}
	.tab:hover {
		color: var(--color-mist-100);
	}
	.tab[aria-selected='true'] {
		color: var(--color-gold-300);
		border-bottom-color: var(--color-gold-500);
	}
	.badge {
		min-width: 18px;
		height: 18px;
		padding: 0 5px;
		border-radius: 9999px;
		font-family: var(--font-sans);
		font-size: 0.65rem;
		line-height: 18px;
		color: var(--color-mist-100);
		background: var(--color-cinnabar-500);
	}
	.send {
		display: inline-grid;
		place-items: center;
		width: 2.75rem;
		height: 2.5rem;
		flex-shrink: 0;
		color: var(--color-mist-300);
		border: 1px solid var(--color-ink-600);
		border-radius: 2px;
		background: transparent;
		transition:
			color 150ms,
			border-color 150ms;
	}
	.send:not(:disabled):hover,
	.send:not(:disabled):focus-visible {
		color: var(--color-gold-300);
		border-color: var(--color-gold-500);
	}
	.send:disabled {
		opacity: 0.45;
	}
	.msg {
		padding: 2px 6px;
		border-radius: 2px;
	}
	.msg.mine {
		background: color-mix(in oklab, var(--color-jade-500) 10%, transparent);
	}
	.name {
		cursor: pointer;
	}
</style>
