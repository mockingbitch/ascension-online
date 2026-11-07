<script lang="ts">
	import { page } from '$app/state';
	import TopupPanel from '$lib/components/TopupPanel.svelte';
	import VipBoard from '$lib/components/VipBoard.svelte';
	import type { VipStatus } from '$lib/game/types';

	let { data } = $props();
	// A verified top-up replaces the level shown (server state, from GET /payments/orders/{id}).
	let fresh = $state<VipStatus | null>(null);
	const status = $derived(fresh ?? data.status);
	const orderId = $derived(page.url.searchParams.get('order'));
	const maxOffline = $derived(data.info.tiers.at(-1)?.benefits.offlineCapHours ?? null);
</script>

<svelte:head><title>Tiên Duyên · Tiên Đạo</title></svelte:head>

<VipBoard info={data.info} {status}>
	{#snippet topup()}
		<TopupPanel signedIn={!!data.status} initialOrderId={orderId} hardMaxOfflineHours={maxOffline} onvip={(v) => (fresh = v)} />
	{/snippet}
</VipBoard>
