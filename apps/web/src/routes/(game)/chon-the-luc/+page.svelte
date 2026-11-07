<!-- Chọn thế lực for players who already have a character but no faction (created before onboarding §9.6 joined in
     the same intent). The game guard sends them here until they join (POST /factions/join), then to Tử Tiêu Cung. -->
<script lang="ts">
	import { goto } from '$app/navigation';
	import FactionPicker from '$lib/components/FactionPicker.svelte';
	import { changeFaction, factions } from '$lib/game/faction.svelte';
	import type { FactionView, PathId } from '$lib/game/types';

	let { data } = $props();
	const ch = $derived(data.character!);
	const path = $derived(data.catalogue.stages.get(ch.stageId)?.path ?? 'tien');
	const pathName = (p: PathId) => data.catalogue.paths.get(p)?.name ?? p;
	const open = $derived(data.list.factions.filter((f) => f.open));
	let error = $state<string | null>(null);

	const cooldownDays = $derived(Math.round((factions.list?.switchCooldownSeconds ?? 0) / 86_400));

	async function join(f: FactionView) {
		error = await changeFaction('join', f.id);
		if (!error) await goto('/play/tu-tieu-cung', { invalidateAll: true });
	}
</script>

<svelte:head><title>Chọn thế lực · Tiên Đạo</title></svelte:head>

<div class="h-dvh overflow-y-auto">
	<main class="mx-auto max-w-7xl px-4 py-8 sm:py-12">
		<header class="mb-8 text-center">
			<p class="eyebrow">Tử Tiêu Cung · buổi giảng đạo đầu tiên</p>
			<h1 class="mt-2 text-4xl font-bold text-gold-300 sm:text-6xl">Chọn con đường của {ch.name}</h1>
			<p class="mx-auto mt-3 max-w-2xl text-mist-300">
				Lời giảng của Hồng Quân vừa dứt, các thế lực mở cửa thu nhận đệ tử. Nơi bạn gia nhập định cách bạn kết giao, giảng đạo và
				đi qua các đại kiếp của Hồng Hoang.
			</p>
		</header>
		{#if error}<p role="alert" class="mb-4 text-center text-cinnabar-300">{error}</p>{/if}
		<FactionPicker factions={open} {path} {pathName} busy={factions.busy} onjoin={join}
			note={cooldownDays ? `Gia nhập xong phải chờ ${cooldownDays} ngày mới đổi phe được.` : undefined} />
	</main>
</div>
