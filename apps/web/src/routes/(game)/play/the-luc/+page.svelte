<!-- Thế Lực (preview): my faction hall — or the picker when I have none. Sample numbers, labelled. -->
<script lang="ts">
	import FactionEmblem from '$lib/components/FactionEmblem.svelte';
	import FactionPicker from '$lib/components/FactionPicker.svelte';
	import Icon from '$lib/components/Icon.svelte';
	import ScreenFrame from '$lib/components/ScreenFrame.svelte';
	import { chat } from '$lib/game/chat.svelte';
	import { game } from '$lib/game/store.svelte';
	import { toast } from '$lib/game/toasts.svelte';
	import { factionById, factionColor, factions, type Faction } from '$lib/mock/factions';
	import { factionChoice, loadFactionChoice, saveFactionChoice } from '$lib/mock/factionChoice.svelte';

	const ch = $derived(game.character!);
	$effect(() => loadFactionChoice(ch.id));
	const f = $derived(factionById(factionChoice.id));
	const rank = $derived(f ? [...factions].sort((a, b) => b.totalCultivation - a.totalCultivation).findIndex((x) => x.id === f.id) + 1 : 0);
	let picking = $state(false);
	// Recent activity: real world-chat lines from this faction's members are not available yet → show the hall's own notices.
	const notices = $derived(
		f
			? [
					`${f.leaderTitle} mở đạo đài giảng pháp vào giờ Thìn và giờ Dậu mỗi ngày.`,
					`Đệ tử mới nhập môn: ${ch.name}.`,
					'Góp công đức và hộ pháp cho đồng môn sẽ khai mở cùng hệ thống thế lực.'
				]
			: []
	);
	function join(x: Faction) {
		saveFactionChoice(ch.id, x.id);
		picking = false;
		toast('ok', `Đã gia nhập ${x.name} (bản xem trước).`);
	}
	const soon = (what: string) => toast('warn', `${what} sẽ khai mở cùng hệ thống thế lực (Phase 2).`);
</script>

<svelte:head><title>Thế Lực · Tiên Đạo</title></svelte:head>

<ScreenFrame title={f && !picking ? f.name : 'Chọn thế lực'} subtitle={f && !picking ? `“${f.motto}”` : 'Năm thế lực của Thiên Nguyên Giới'} icon="sect" status="preview">
	{#if !f || picking}
		<FactionPicker current={factionChoice.id} onjoin={join} />
	{:else}
		<div class="grid gap-5 lg:grid-cols-[320px_minmax(0,1fr)]" style:--fc={factionColor(f)}>
			<section class="hall flex flex-col items-center px-4 py-6 text-center">
				<span style:color="var(--fc)"><FactionEmblem id={f.id} size={120} /></span>
				<p class="mt-3 text-sm text-mist-300">Chủ sự: <span class="text-mist-100">{f.leaderTitle}</span></p>
				<dl class="mt-4 grid w-full grid-cols-3 gap-2 text-center text-sm">
					<div><dt class="eyebrow">Đệ tử</dt><dd class="font-semibold tabular-nums">{f.members.toLocaleString('vi-VN')}</dd></div>
					<div><dt class="eyebrow">Tu vi</dt><dd class="font-semibold tabular-nums">{(f.totalCultivation / 1000).toLocaleString('vi-VN')}k</dd></div>
					<div><dt class="eyebrow">Hạng</dt><dd class="font-semibold text-gold-300">#{rank}</dd></div>
				</dl>
				<button class="mt-5 text-xs text-mist-500 hover:text-mist-300" onclick={() => (picking = true)}>Đổi thế lực (bản xem trước)</button>
			</section>

			<div class="space-y-5">
				<section>
					<h2 class="panel-title">Hành động</h2>
					<div class="mt-3 grid grid-cols-2 gap-2 sm:grid-cols-4">
						<button class="act" onclick={() => soon('Danh sách thành viên')}><Icon name="person" size={22} />Thành viên</button>
						<button class="act" onclick={() => { chat.draft = `[${f.name}] `; toast('ok', 'Kênh thế lực chưa mở — tin sẽ gửi vào kênh Thế Giới.'); }}><Icon name="chat" size={22} />Truyền âm</button>
						<button class="act" onclick={() => soon('Góp công đức')}><Icon name="gift" size={22} />Góp công đức</button>
						<button class="act" onclick={() => soon('Hộ pháp đồng môn')}><Icon name="talisman" size={22} />Hộ pháp</button>
					</div>
				</section>
				<section>
					<h2 class="panel-title">Đặc điểm</h2>
					<p class="mt-2 text-mist-100">{f.description}</p>
					<ul class="mt-2 flex flex-wrap gap-2">
						{#each f.traits as t (t)}<li class="trait">{t}</li>{/each}
					</ul>
				</section>
				<section>
					<h2 class="panel-title">Hoạt động gần đây</h2>
					<ul class="mt-2 space-y-2 text-sm text-mist-300">
						{#each notices as n (n)}<li class="flex gap-2"><Icon name="bolt" size={14} class="mt-1 shrink-0 text-gold-500" />{n}</li>{/each}
					</ul>
				</section>
			</div>
		</div>
	{/if}
</ScreenFrame>

<style>
	.hall {
		border: 1px solid color-mix(in oklab, var(--fc) 40%, transparent);
		background:
			radial-gradient(circle at 50% 20%, color-mix(in oklab, var(--fc) 22%, transparent), transparent 60%),
			color-mix(in oklab, var(--color-ink-950) 45%, transparent);
	}
	.act {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 0.4rem;
		padding: 0.9rem 0.5rem;
		font-size: 0.85rem;
		color: var(--color-mist-100);
		border: 1px solid color-mix(in oklab, var(--fc) 30%, transparent);
		background: color-mix(in oklab, var(--color-ink-950) 45%, transparent);
		transition: box-shadow 200ms, transform 200ms var(--ease-qi);
	}
	.act:hover {
		transform: translateY(-2px);
		box-shadow: 0 0 20px -6px var(--fc);
	}
	.trait {
		padding: 2px 10px;
		font-size: 0.8rem;
		color: var(--fc);
		border: 1px solid color-mix(in oklab, var(--fc) 45%, transparent);
		border-radius: 9999px;
	}
</style>
