<!-- Player card: opened from chat names, Thiên Bảng rows or my own HUD plate. -->
<script lang="ts">
	import { mention } from '$lib/game/chat.svelte';
	import { game } from '$lib/game/store.svelte';
	import { closeWindow, openWindow } from '$lib/game/windows';
	import Icon from './Icon.svelte';
	import Portrait from './Portrait.svelte';
	import RealmBadge from './RealmBadge.svelte';
	import ReportDialog from './ReportDialog.svelte';

	interface Props {
		characterId: string;
		name: string;
		stageId: string;
		messageId?: number;
	}
	let { characterId, name, stageId, messageId }: Props = $props();

	const me = $derived(characterId === game.character?.id);
	// My own card shows live data; others show the snapshot carried by the chat message / leaderboard row.
	const stage = $derived(me && game.character ? game.character.stageId : stageId);
	const realm = $derived(game.catalogue?.realmOf(stage));
	const rank = $derived(me ? game.myRank : (game.top.find((e) => e.characterId === characterId)?.rank ?? null));
	let reportTarget = $state<{ characterId: string; name: string; messageId?: number } | null>(null);

	function doMention() {
		mention(name);
		// Desktop shows chat beside the scene; on mobile chat is a window.
		if (matchMedia('(min-width: 1024px)').matches) closeWindow();
		else openWindow({ kind: 'chat' });
	}
</script>

<div class="flex flex-col items-center text-center">
	<Portrait aura={realm?.visual.aura ?? 'none'} size={104} />
	<p class="mt-3 font-display text-3xl font-bold text-mist-100">{name}</p>
	{#if realm}<div class="mt-2"><RealmBadge name={realm.name} aura={realm.visual.aura} size="sm" /></div>{/if}
	<p class="mt-1 text-sm text-mist-300">{game.catalogue?.stageLabel(stage)}{#if rank} · <span class="text-gold-300">Hạng {rank}</span>{/if}</p>

	{#if me}
		<dl class="mt-5 grid w-full grid-cols-2 gap-2 text-sm">
			<div class="border border-gold-500/15 p-3"><dt class="eyebrow">Linh thạch</dt><dd class="mt-1 font-semibold tabular-nums">{game.stones.toLocaleString('vi-VN')}</dd></div>
			<div class="border border-gold-500/15 p-3"><dt class="eyebrow">Tụ Linh Phù</dt><dd class="mt-1 font-semibold tabular-nums">{game.phuCount}</dd></div>
		</dl>
		{#if game.character && game.character.offlineBonusSeconds <= 0 && game.phuCount > 0 && !game.character.bonusUsedToday}
			<button class="btn btn-ghost mt-3 w-full" disabled={game.busy} onclick={game.usePhu}>
				<Icon name="talisman" size={16} /> Dùng Tụ Linh Phù
			</button>
		{/if}
	{:else}
		<div class="mt-5 grid w-full grid-cols-2 gap-2">
			<button class="btn btn-jade" onclick={doMention}><Icon name="chat" size={16} /> Nhắc tên</button>
			<button class="btn btn-ghost h-11" onclick={() => (reportTarget = { characterId, name, messageId })}>
				<Icon name="flag" size={16} /> Báo cáo
			</button>
		</div>
		{#if messageId}<p class="mt-2 text-xs text-mist-500">Báo cáo từ kênh chat sẽ đính kèm tin nhắn bạn đã chọn.</p>{/if}
	{/if}
</div>

<ReportDialog target={reportTarget} onclose={() => (reportTarget = null)} />
