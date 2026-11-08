<!--
	Floating chat, bottom-left of the shell (spec §3.4, §4.8) — replaces the fixed chat column.
	Collapsed: a round chat button with the unread count and, above it, the 3 newest messages as bubbles that slide up,
	fade in and fade away after 8 s (your own message flashes jade). Click a bubble or the button to expand into the
	tabbed panel (ChatPanel: Luận Đạo while in Tử Tiêu Cung, Thế Giới). Esc or the chevron collapses it.
	Not a modal: the scene stays playable while chatting. Speech bubbles over characters in the hall stay (HallView).
-->
<script lang="ts">
	import { tick, untrack } from 'svelte';
	import { fade, fly } from 'svelte/transition';
	import { settings } from '$lib/engine/settings.svelte';
	import { addBubbles, expireBubbles, nextExpiry, type Bubble, type BubbleMsg } from '$lib/game/chatBubbles';
	import { chat, chatUi } from '$lib/game/chat.svelte';
	import { hall } from '$lib/game/lecture.svelte';
	import { game } from '$lib/game/store.svelte';
	import type { ChatMessage } from '$lib/game/types';
	import ChatPanel from './ChatPanel.svelte';
	import Icon from './Icon.svelte';
	import NameFx from './NameFx.svelte';

	let bubbles = $state<Bubble[]>([]);
	let toggle = $state<HTMLButtonElement>();
	let panel = $state<HTMLElement>();
	const seen = { world: -1, hall: -1 };

	const unread = $derived(chat.unread + (hall.active ? hall.unread : 0));
	const me = $derived(game.character?.id);
	const toMsg = (ch: 'world' | 'hall', m: ChatMessage): BubbleMsg => ({
		key: `${ch}:${m.id}`,
		name: m.name,
		text: m.text,
		stageId: m.stageId,
		nameFx: m.nameFx,
		mine: m.characterId === me
	});

	// New messages become bubbles. The first load seeds the newest three (each still fades after 8 s).
	function take(ch: 'world' | 'hall', list: ChatMessage[], loaded: boolean) {
		if (!loaded || !list.length) return;
		const last = list.at(-1)!.id;
		const fresh = seen[ch] < 0 ? list.slice(-3) : list.filter((m) => m.id > seen[ch]);
		seen[ch] = Math.max(seen[ch], last);
		if (fresh.length) untrack(() => !chatUi.open && (bubbles = addBubbles(bubbles, fresh.map((m) => toMsg(ch, m)), Date.now())));
	}
	$effect(() => take('world', chat.messages, chat.loaded));
	$effect(() => {
		if (hall.active) take('hall', hall.messages, hall.chatLoaded);
		else seen.hall = -1;
	});
	// One timer for the next expiry (no rAF; nothing runs when there are no bubbles).
	$effect(() => {
		const wait = nextExpiry(bubbles, Date.now());
		if (wait === null) return;
		const t = setTimeout(() => (bubbles = expireBubbles(bubbles, Date.now())), wait + 20);
		return () => clearTimeout(t);
	});

	async function expand() {
		chatUi.open = true;
		bubbles = [];
		await tick();
		panel?.querySelector<HTMLInputElement>('input')?.focus({ preventScroll: true });
	}
	async function collapse() {
		chatUi.open = false;
		await tick();
		toggle?.focus({ preventScroll: true });
	}
	const dur = (full: number) => (settings.fx === 'off' ? 0 : settings.fx === 'lite' ? Math.round(full / 2) : full);
	const auraOf = (stageId: string) => game.catalogue?.realmOf(stageId)?.visual.aura ?? 'none';
</script>

<div class="floating-chat" data-floating-chat data-open={chatUi.open}>
	{#if chatUi.open}
		<!-- svelte-ignore a11y_no_noninteractive_element_interactions (Esc collapses the panel from any control in it) -->
		<section
			bind:this={panel}
			class="expanded panel"
			aria-label="Truyền âm"
			data-chat-panel
			transition:fly={{ y: 16, duration: dur(220) }}
			onkeydown={(e) => {
				if (e.key === 'Escape') {
					e.stopPropagation();
					void collapse();
				}
			}}
		>
			<header class="flex items-center gap-2 border-b border-gold-500/15 py-1 pr-1 pl-3">
				<Icon name="chat" size={16} class="text-gold-500" />
				<h2 class="flex-1 font-display text-lg font-bold text-gold-300">Truyền âm</h2>
				<button type="button" class="hbtn press" onclick={collapse} data-chat-collapse>
					<Icon name="chevron" label="Thu gọn kênh trò chuyện" />
				</button>
			</header>
			<div class="min-h-0 flex-1"><ChatPanel compact /></div>
		</section>
	{:else}
		<ol class="bubbles" aria-label="Tin nhắn mới">
			{#each bubbles as b (b.key)}
				<li in:fly={{ y: 14, duration: dur(240) }} out:fade={{ duration: dur(400) }}>
					<button type="button" class="bubble" class:mine={b.mine} onclick={expand}>
						<span class="font-semibold" style:color="var(--color-aura-{auraOf(b.stageId)})"><NameFx fx={b.nameFx}>{b.name}</NameFx></span><span class="text-mist-500">:</span>
						<span class="text-mist-100">{b.text}</span>
					</button>
				</li>
			{/each}
		</ol>
		<button bind:this={toggle} type="button" class="toggle press" onclick={expand} aria-expanded="false" data-chat-toggle>
			<Icon name="chat" size={20} label="Mở kênh trò chuyện" />
			{#if unread}<span class="badge tabular-nums" aria-label="{unread} tin chưa đọc">{unread > 9 ? '9+' : unread}</span>{/if}
		</button>
	{/if}
</div>

<style>
	.floating-chat {
		position: absolute;
		left: 0.5rem;
		/* a page can lift it above its own bottom band (the hall subtitles) with --chat-lift */
		bottom: calc(0.5rem + var(--chat-lift, 0px));
		z-index: 20;
		display: flex;
		flex-direction: column;
		align-items: flex-start;
		gap: 0.4rem;
		max-width: min(22rem, calc(100% - 1rem));
		pointer-events: none;
	}
	.floating-chat > :global(*) {
		pointer-events: auto;
	}
	.bubbles {
		display: flex;
		flex-direction: column;
		align-items: flex-start;
		gap: 0.3rem;
		pointer-events: none;
	}
	.bubbles li {
		max-width: 100%;
	}
	.bubble {
		display: block;
		max-width: 100%;
		overflow: hidden;
		padding: 0.3rem 0.65rem;
		font-size: 0.8rem;
		line-height: 1.3;
		text-align: left;
		text-overflow: ellipsis;
		white-space: nowrap;
		pointer-events: auto;
		border: 1px solid color-mix(in oklab, var(--color-gold-500) 20%, transparent);
		border-radius: 12px 12px 12px 3px;
		background: color-mix(in oklab, var(--color-ink-950) 82%, transparent);
		backdrop-filter: blur(3px);
	}
	.bubble.mine {
		border-color: color-mix(in oklab, var(--color-jade-300) 55%, transparent);
		animation: jade-flash var(--dur-slow) var(--ease-qi) 2;
	}
	:global(:root[data-fx='off']) .bubble.mine {
		animation: none;
	}
	.toggle {
		position: relative;
		display: grid;
		width: 2.75rem;
		height: 2.75rem;
		place-items: center;
		border-radius: 9999px;
		color: var(--color-gold-300);
		border: 1px solid color-mix(in oklab, var(--color-gold-500) 40%, transparent);
		background: radial-gradient(circle at 50% 30%, var(--color-ink-700), var(--color-ink-950));
		box-shadow: 0 6px 16px -8px var(--color-ink-950);
	}
	.badge {
		position: absolute;
		top: -4px;
		right: -6px;
		min-width: 18px;
		height: 18px;
		padding: 0 4px;
		border-radius: 9999px;
		font-size: 0.68rem;
		font-weight: 700;
		line-height: 18px;
		text-align: center;
		color: var(--color-mist-100);
		background: var(--color-cinnabar-500);
		animation: pop var(--dur-slow) var(--ease-out-back) both;
	}
	:global(:root[data-fx='off']) .badge {
		animation: none;
	}
	.expanded {
		display: flex;
		width: min(22rem, calc(100vw - 1rem));
		height: min(28rem, 58dvh);
		flex-direction: column;
		overflow: hidden;
	}
	.hbtn {
		display: inline-grid;
		width: 2.5rem;
		height: 2.5rem;
		place-items: center;
		color: var(--color-mist-300);
	}
	.hbtn:hover {
		color: var(--color-gold-300);
	}
	@keyframes jade-flash {
		50% {
			background: color-mix(in oklab, var(--color-jade-500) 40%, var(--color-ink-950));
		}
	}
	@keyframes pop {
		from {
			transform: scale(0.6);
		}
		60% {
			transform: scale(1.1);
		}
	}
</style>
