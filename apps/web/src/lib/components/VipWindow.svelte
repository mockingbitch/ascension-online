<!-- Tiên Duyên inside the game shell: daily gift + tier ladder. -->
<script lang="ts">
	import { api } from '$lib/api/client';
	import { game } from '$lib/game/store.svelte';
	import type { VipTier } from '$lib/game/types';
	import Icon from './Icon.svelte';
	import VipBoard from './VipBoard.svelte';

	let tiers = $state<VipTier[] | null>(null);
	$effect(() => {
		api.vipTiers().then((t) => (tiers = t.tiers), () => (tiers = []));
	});
	const vip = $derived(game.vip);
</script>

{#if vip && vip.benefits.dailyGift > 0}
	<div class="mb-4 flex items-center justify-between gap-3 border border-gold-500/25 bg-ink-950/40 px-4 py-3">
		<span class="flex items-center gap-2 text-sm text-mist-300">
			<Icon name="gift" size={18} class="text-gold-500" /> Quà hôm nay: <strong class="text-mist-100 tabular-nums">{vip.benefits.dailyGift}</strong> linh thạch
		</span>
		{#if vip.giftClaimedToday}
			<span class="flex items-center gap-1 text-sm text-jade-300"><Icon name="check" size={16} /> Đã nhận</span>
		{:else}
			<button class="btn btn-gold h-10 px-4 text-sm" disabled={game.busy} onclick={game.claimGift}>Nhận quà</button>
		{/if}
	</div>
{/if}

{#if tiers === null}
	<p class="text-mist-500">Đang tải…</p>
{:else}
	<VipBoard {tiers} status={vip} compact />
{/if}
