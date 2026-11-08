<!-- Player card (window): opened from chat names, Thiên Bảng rows or my own HUD plate. -->
<script lang="ts">
	import { chatUi, mention } from '$lib/game/chat.svelte';
	import { cosmetics } from '$lib/game/cosmetics.svelte';
	import { game } from '$lib/game/store.svelte';
	import { closeWindow, openWindow } from '$lib/game/windows';
	import type { WindowProps } from '$lib/windows/registry';
	import Icon from './Icon.svelte';
	import NameFx from './NameFx.svelte';
	import Portrait from './Portrait.svelte';
	import RealmBadge from './RealmBadge.svelte';
	import ReportDialog from './ReportDialog.svelte';

	let { win }: WindowProps = $props();
	const p = $derived(win.kind === 'profile' ? win : { characterId: '', name: '', stageId: '' });
	const characterId = $derived(p.characterId);
	const name = $derived(p.name);
	const stageId = $derived(p.stageId);
	const messageId = $derived('messageId' in p ? p.messageId : undefined);
	const avatarFrameId = $derived('avatarFrameId' in p ? p.avatarFrameId : undefined);
	const titleId = $derived('titleId' in p ? p.titleId : undefined);
	const nameFx = $derived('nameFx' in p ? p.nameFx : undefined);

	const me = $derived(characterId === game.character?.id);
	// My own card shows live data; others show the snapshot carried by the chat message / leaderboard row.
	const stage = $derived(me && game.character ? game.character.stageId : stageId);
	const realm = $derived(game.catalogue?.realmOf(stage));
	// Cosmetics (looks only): live for me, the snapshot carried by the row/message for others.
	const look = $derived(me && game.character?.appearance ? { ...game.character.appearance, nameFx: game.character.appearance.chatAuraId } : { avatarFrameId, titleId, nameFx });
	const title = $derived(cosmetics.name(look.titleId));
	const rank = $derived(me ? game.myRank : (game.top.find((e) => e.characterId === characterId)?.rank ?? null));
	let reportTarget = $state<{ characterId: string; name: string; messageId?: number } | null>(null);

	// Đạo lữ: Đào Hoa Đình over this card with the proposal confirmation (the server checks every gate).
	const propose = () => openWindow({ kind: 'companion', propose: { characterId, name, stageId: stage } });
	// Trao đổi: Túi Đồ › Trao đổi over this card, composer addressed to this character (the server checks every gate).
	const trade = () => openWindow({ kind: 'inventory', tab: 'trade', to: { characterId, name, stageId: stage } });

	function doMention() {
		mention(name);
		closeWindow();
		chatUi.open = true; // the floating chat expands with the mention in its composer
	}
</script>

<div class="flex flex-col items-center text-center">
	<Portrait aura={realm?.visual.aura ?? 'none'} size={104} frameId={look.avatarFrameId} />
	<p class="mt-3 font-display text-3xl font-bold text-mist-100"><NameFx fx={look.nameFx}>{name}</NameFx></p>
	{#if title}<p class="mt-0.5 text-sm text-gold-300">「{title}」</p>{/if}
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
		<div class="mt-2 grid w-full grid-cols-2 gap-2">
			<button class="btn btn-ghost h-11" onclick={trade}><Icon name="gift" size={16} /> Trao đổi</button>
			<button class="btn btn-gold" onclick={propose}><Icon name="heart" size={16} /> Cầu thân</button>
		</div>
		{#if messageId}<p class="mt-2 text-xs text-mist-500">Báo cáo từ kênh chat sẽ đính kèm tin nhắn bạn đã chọn.</p>{/if}
	{/if}
</div>

<ReportDialog target={reportTarget} onclose={() => (reportTarget = null)} />
