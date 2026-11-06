<!-- Chọn thế lực (after creating a character): full-screen in front of Tử Tiêu Cung. Preview flow — see factionChoice. -->
<script lang="ts">
	import { goto } from '$app/navigation';
	import FactionPicker from '$lib/components/FactionPicker.svelte';
	import { factionChoice, loadFactionChoice, saveFactionChoice } from '$lib/mock/factionChoice.svelte';
	import type { Faction } from '$lib/mock/factions';

	let { data } = $props();
	const ch = $derived(data.character!);
	$effect(() => loadFactionChoice(ch.id));

	async function join(f: Faction) {
		saveFactionChoice(ch.id, f.id);
		await goto('/play');
	}
</script>

<svelte:head><title>Chọn thế lực · Tiên Đạo</title></svelte:head>

<div class="h-dvh overflow-y-auto">
	<main class="mx-auto max-w-7xl px-4 py-8 sm:py-12">
		<header class="mb-8 text-center">
			<p class="eyebrow">Tử Tiêu Cung · buổi giảng đạo đầu tiên</p>
			<h1 class="mt-2 text-4xl font-bold text-gold-300 sm:text-6xl">Chọn con đường của {ch.name}</h1>
			<p class="mx-auto mt-3 max-w-2xl text-mist-300">
				Lời giảng của Hồng Quân vừa dứt, năm thế lực mở cửa thu nhận đệ tử. Nơi bạn gia nhập định cách bạn kết giao, giảng đạo và
				đi qua các đại kiếp của Thiên Nguyên Giới.
			</p>
			<p class="mt-2 text-xs text-gold-500">Bản xem trước — hệ thống thế lực chưa nối máy chủ.</p>
		</header>
		<FactionPicker current={factionChoice.id} onjoin={join} />
		<p class="mt-6 text-center"><a href="/play" class="text-sm text-mist-500 hover:text-mist-300">Để sau — vào thế giới trước</a></p>
	</main>
</div>
