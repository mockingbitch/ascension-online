<script lang="ts">
	import VipBoard from '$lib/components/VipBoard.svelte';

	let { data } = $props();
</script>

<svelte:head><title>Tiên Duyên · Tiên Đạo</title></svelte:head>

<VipBoard tiers={data.tiers} status={data.status} />
