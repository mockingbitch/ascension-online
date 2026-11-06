<!--
	World chat: history (scroll up for older), live messages, composer. Names open the player card.
	The posting gate shown here is a display hint from the catalogue; the server enforces it (stage_too_low).
-->
<script lang="ts">
	import { tick } from 'svelte';
	import { chat, loadOlder, markChatVisible, sendChat } from '$lib/game/chat.svelte';
	import { game } from '$lib/game/store.svelte';
	import { openWindow } from '$lib/game/windows';
	import type { ChatMessage } from '$lib/game/types';
	import Icon from './Icon.svelte';

	let { compact = false }: { compact?: boolean } = $props();

	let list = $state<HTMLOListElement>();
	let input = $state<HTMLInputElement>();
	let stick = true; // follow new messages while the user is at the bottom

	const cat = $derived(game.catalogue);
	const me = $derived(game.character);
	const minStage = $derived(chat.channel && cat ? cat.stages.get(chat.channel.minStage) : undefined);
	const myStage = $derived(me && cat ? cat.stages.get(me.stageId) : undefined);
	// Display hint only (same path compare by tier/order; the server decides).
	const canPost = $derived(!minStage || !myStage || myStage.tier > minStage.tier || (myStage.tier === minStage.tier && myStage.stageOrder >= minStage.stageOrder));

	$effect(() => {
		markChatVisible(true);
		return () => markChatVisible(false);
	});

	// Auto-scroll on new messages when pinned to the bottom.
	$effect(() => {
		void chat.messages.length;
		if (!list || !stick) return;
		void tick().then(() => list && (list.scrollTop = list.scrollHeight));
	});

	async function onscroll() {
		if (!list) return;
		stick = list.scrollHeight - list.scrollTop - list.clientHeight < 40;
		if (list.scrollTop < 60 && chat.hasOlder && !chat.loadingOlder) {
			const before = list.scrollHeight;
			await loadOlder();
			await tick();
			list.scrollTop += list.scrollHeight - before; // keep position after prepending
		}
	}

	async function submit(e: SubmitEvent) {
		e.preventDefault();
		const t = chat.draft.trim();
		if (!t) return;
		stick = true;
		if (await sendChat(t)) chat.draft = '';
		input?.focus();
	}

	const time = (iso: string) => new Date(iso).toLocaleTimeString('vi-VN', { hour: '2-digit', minute: '2-digit' });
	const auraOf = (stageId: string) => cat?.realmOf(stageId)?.visual.aura ?? 'none';
	const openProfile = (m: ChatMessage) =>
		openWindow({ kind: 'profile', characterId: m.characterId, name: m.name, stageId: m.stageId, messageId: m.id });
</script>

<section class="flex h-full min-h-0 flex-col" aria-label="Kênh trò chuyện">
	{#if !compact}
		<header class="flex items-center gap-2 border-b border-gold-500/15 px-4 py-2.5">
			<h2 class="flex-1 font-display text-lg font-bold text-gold-300">Kênh {chat.channel?.name ?? 'Thế Giới'}</h2>
		</header>
	{/if}

	<ol bind:this={list} {onscroll} class="min-h-0 flex-1 space-y-1.5 overflow-y-auto overscroll-contain px-3 py-2 text-sm" aria-live="polite" aria-relevant="additions">
		{#if chat.loadingOlder}<li class="py-1 text-center text-xs text-mist-500">Đang tải tin cũ…</li>{/if}
		{#if chat.loaded && chat.messages.length === 0}
			<li class="py-6 text-center text-mist-500">Chưa ai lên tiếng. Hãy là người đầu tiên chào đạo hữu.</li>
		{:else if !chat.loaded}
			<li class="py-6 text-center text-mist-500">Đang kết nối truyền âm…</li>
		{/if}
		{#each chat.messages as m (m.id)}
			<li class="msg leading-snug break-words" class:mine={m.characterId === me?.id}>
				<span class="mr-1 text-xs text-mist-500 tabular-nums">{time(m.sentAt)}</span>
				<button type="button" class="name font-semibold hover:underline" style:color="var(--color-aura-{auraOf(m.stageId)})" onclick={() => openProfile(m)}>
					{m.name}
				</button><span class="text-mist-500">:</span>
				<span class="text-mist-100">{m.text}</span>
			</li>
		{/each}
	</ol>

	<form onsubmit={submit} class="border-t border-gold-500/15 p-2.5">
		{#if canPost}
			<div class="flex gap-2">
				<input
					bind:this={input}
					bind:value={chat.draft}
					maxlength={chat.maxChars}
					autocomplete="off"
					enterkeyhint="send"
					placeholder="Truyền âm cho thiên hạ…"
					aria-label="Nội dung tin nhắn"
					class="h-10 min-w-0 flex-1 rounded-[2px] border border-ink-600 bg-ink-950/60 px-3 text-mist-100 outline-none placeholder:text-mist-500 focus:border-gold-500"
				/>
				<button class="btn btn-jade h-10 w-12 px-0" disabled={chat.sending || !chat.draft.trim()} aria-label="Gửi">
					<Icon name="arrow" size={18} />
				</button>
			</div>
			{#if chat.error}<p role="alert" class="mt-1.5 text-xs text-cinnabar-300">{chat.error}</p>{/if}
		{:else}
			<p class="flex items-center gap-2 px-1 py-2 text-xs text-mist-500">
				<Icon name="lock" size={14} /> Đạt {minStage?.name ?? 'cảnh giới yêu cầu'} để truyền âm. Bạn vẫn đọc được mọi tin.
			</p>
		{/if}
	</form>
</section>

<style>
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
