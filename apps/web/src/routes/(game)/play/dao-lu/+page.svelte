<!-- Đạo Lữ (preview): dual-cultivation bond. The real system is Phase 5 (dao_lu_bonds, permanent, server-checked);
     this shows the flow — invitation, the permanent-bond warning and confirmation — on sample state only. -->
<script lang="ts">
	import GameWindow from '$lib/components/GameWindow.svelte';
	import Icon from '$lib/components/Icon.svelte';
	import Portrait from '$lib/components/Portrait.svelte';
	import ScreenFrame from '$lib/components/ScreenFrame.svelte';
	import { chat } from '$lib/game/chat.svelte';
	import { game } from '$lib/game/store.svelte';
	import { toast } from '$lib/game/toasts.svelte';

	const ch = $derived(game.character!);
	const cat = $derived(game.catalogue!);
	const aura = $derived(cat.realmOf(ch.stageId)?.visual.aura ?? 'none');

	// Candidates: people actually seen in world chat (names only, nothing is sent to them in the preview).
	const candidates = $derived(
		[...new Map(chat.messages.filter((m) => m.characterId !== ch.id).map((m) => [m.characterId, m])).values()].slice(-8).reverse()
	);
	let invited = $state<{ id: string; name: string; stageId: string } | null>(null);
	let confirming = $state(false);
	let typed = $state('');

	function invite(c: { characterId: string; name: string; stageId: string }) {
		invited = { id: c.characterId, name: c.name, stageId: c.stageId };
		toast('ok', `Đã soạn lời mời kết đạo lữ gửi ${c.name} (bản xem trước — chưa gửi thật).`);
	}
</script>

<svelte:head><title>Đạo Lữ · Tiên Đạo</title></svelte:head>

<ScreenFrame title="Đào Hoa Đình" subtitle="Đạo lữ song tu — một lời thề, trọn một đời tu" icon="heart" status="preview">
	<section class="bond relative overflow-hidden px-4 py-8">
		<div class="petals" aria-hidden="true">
			{#each Array.from({ length: 14 }, (_, i) => i) as i (i)}<span style:left="{(i * 37) % 100}%" style:animation-delay="{(i * 0.7) % 8}s"></span>{/each}
		</div>
		<div class="relative mx-auto flex max-w-2xl items-center justify-between gap-2">
			<div class="flex flex-col items-center text-center">
				<div class="ring"><Portrait {aura} size={104} /></div>
				<p class="mt-2 font-display text-xl font-bold text-mist-100">{ch.name}</p>
				<p class="text-xs text-mist-300">{cat.stageLabel(ch.stageId)}</p>
			</div>
			<div class="link flex-1" class:is-on={!!invited} aria-hidden="true"></div>
			<div class="flex flex-col items-center text-center">
				{#if invited}
					<div class="ring"><Portrait aura={cat.realmOf(invited.stageId)?.visual.aura ?? 'none'} size={104} /></div>
					<p class="mt-2 font-display text-xl font-bold text-mist-100">{invited.name}</p>
					<p class="text-xs text-gold-300">Đang chờ hồi đáp</p>
				{:else}
					<div class="ring empty grid size-[112px] place-items-center text-mist-500"><Icon name="heart" size={36} /></div>
					<p class="mt-2 font-display text-xl text-mist-500">Chưa có đạo lữ</p>
				{/if}
			</div>
		</div>
		<dl class="relative mx-auto mt-6 grid max-w-2xl grid-cols-3 gap-2 text-center text-sm">
			<div><dt class="eyebrow">Hiệu quả song tu</dt><dd class="text-jade-300">tăng hiệu quả</dd></div>
			<div><dt class="eyebrow">Thời gian</dt><dd>2 canh giờ/ngày</dd></div>
			<div><dt class="eyebrow">Điều kiện</dt><dd>cả hai cùng online</dd></div>
		</dl>
		<p class="relative mx-auto mt-2 max-w-2xl text-center text-xs text-mist-500">Song tu vẫn nằm trong trần 150 điểm/ngày — không ai vượt trần nhờ đạo lữ.</p>
	</section>

	<div class="mt-5 grid gap-5 lg:grid-cols-2">
		<section>
			<h2 class="panel-title">Đạo hữu vừa truyền âm</h2>
			{#if candidates.length}
				<ul class="mt-2 divide-y divide-gold-500/10">
					{#each candidates as c (c.characterId)}
						<li class="flex items-center gap-3 py-2">
							<span class="min-w-0 flex-1 truncate"><span class="font-semibold" style:color="var(--color-aura-{cat.realmOf(c.stageId)?.visual.aura ?? 'none'})">{c.name}</span> <span class="text-xs text-mist-500">{cat.stageLabel(c.stageId)}</span></span>
							<button class="btn btn-ghost h-9 px-3 text-sm" disabled={invited?.id === c.characterId} onclick={() => invite(c)}>Mời</button>
						</li>
					{/each}
				</ul>
			{:else}
				<p class="mt-2 text-sm text-mist-500">Chưa thấy ai trong kênh Thế Giới. Hãy truyền âm làm quen trước.</p>
			{/if}
		</section>
		<section>
			<h2 class="panel-title">Kết trọn đời</h2>
			<p class="mt-2 text-sm text-mist-300">Khi đối phương chấp nhận, hai bên xác nhận lần cuối. Liên kết là <strong class="text-mist-100">vĩnh viễn</strong>, chỉ chấm dứt bởi lượng kiếp, luân hồi hoặc ban quản trị.</p>
			<button class="btn btn-gold mt-3 w-full" disabled={!invited} onclick={() => ((typed = ''), (confirming = true))}>
				<Icon name="heart" size={16} /> Xác nhận kết đạo lữ
			</button>
		</section>
	</div>
</ScreenFrame>

<GameWindow open={confirming} title="Lời thề đạo lữ" onclose={() => (confirming = false)}>
	{#if invited}
		<p class="flex items-start gap-2 border-l-2 border-cinnabar-500 bg-ink-950/50 p-3 text-sm text-cinnabar-300">
			<Icon name="bolt" size={18} class="mt-0.5 shrink-0" />
			<span>Kết đạo lữ với <strong class="text-mist-100">{invited.name}</strong> là ràng buộc <strong>vĩnh viễn</strong>. Không thể tự hủy. Mỗi người chỉ có một đạo lữ.</span>
		</p>
		<label class="mt-4 block text-sm text-mist-300">
			Gõ đúng đạo hiệu <strong class="text-mist-100">{invited.name}</strong> để xác nhận
			<input bind:value={typed} autocomplete="off" class="mt-1 h-11 w-full rounded-[2px] border border-ink-600 bg-ink-950/60 px-3 text-mist-100 outline-none focus:border-gold-500" />
		</label>
		<div class="mt-4 grid grid-cols-2 gap-2">
			<button class="btn btn-ghost h-11" onclick={() => (confirming = false)}>Suy nghĩ thêm</button>
			<button class="btn btn-gold" disabled={typed.trim() !== invited.name} onclick={() => { confirming = false; toast('warn', 'Bản xem trước: lời thề chưa được ghi. Hệ thống đạo lữ sẽ mở ở bản cập nhật Xã hội.'); }}>
				Lập lời thề
			</button>
		</div>
	{/if}
</GameWindow>

<style>
	.bond {
		border: 1px solid color-mix(in oklab, var(--color-cinnabar-300) 30%, transparent);
		background:
			radial-gradient(ellipse at 50% 40%, color-mix(in oklab, var(--color-cinnabar-300) 16%, transparent), transparent 60%),
			radial-gradient(ellipse at 50% 100%, color-mix(in oklab, var(--color-gold-300) 14%, transparent), transparent 60%),
			color-mix(in oklab, var(--color-ink-950) 45%, transparent);
	}
	.ring {
		border-radius: 9999px;
		box-shadow:
			0 0 0 2px color-mix(in oklab, var(--color-gold-300) 60%, transparent),
			0 0 30px -4px color-mix(in oklab, var(--color-cinnabar-300) 60%, transparent);
	}
	.ring.empty {
		border: 1px dashed var(--color-mist-500);
		box-shadow: none;
	}
	.link {
		height: 2px;
		margin: 0 0.5rem 3rem;
		background: repeating-linear-gradient(90deg, var(--color-mist-500) 0 6px, transparent 6px 12px);
		opacity: 0.5;
	}
	.link.is-on {
		background: linear-gradient(90deg, var(--color-gold-300), var(--color-cinnabar-300), var(--color-gold-300));
		box-shadow: 0 0 12px var(--color-cinnabar-300);
		opacity: 1;
		animation: var(--animate-qi-breathe);
	}
	.petals span {
		position: absolute;
		top: -10px;
		width: 8px;
		height: 6px;
		border-radius: 60% 0;
		background: var(--color-cinnabar-300);
		opacity: 0.55;
		animation: fall 9s linear infinite;
	}
	@keyframes fall {
		to {
			transform: translate(40px, 420px) rotate(320deg);
			opacity: 0;
		}
	}
</style>
