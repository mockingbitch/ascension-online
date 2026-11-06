<!-- Đột Phá: current → next realm, conditions, disclosed odds, the trial that guards the next realm, and the attempt.
     Odds/loss come from GET /breakthrough/preview; the outcome only from POST /breakthrough/attempt. -->
<script lang="ts">
	import BreakthroughPanel from '$lib/components/BreakthroughPanel.svelte';
	import CultivationScene from '$lib/components/CultivationScene.svelte';
	import Icon from '$lib/components/Icon.svelte';
	import RealmBadge from '$lib/components/RealmBadge.svelte';
	import ScreenFrame from '$lib/components/ScreenFrame.svelte';
	import { formatPoints } from '$lib/game/format';
	import { useIdleClock } from '$lib/game/idle.svelte';
	import { game } from '$lib/game/store.svelte';

	const clock = useIdleClock();
	const ch = $derived(game.character!);
	const cat = $derived(game.catalogue!);
	const pv = $derived(game.preview);
	const fromRealm = $derived(cat.realmOf(ch.stageId));
	const toRealm = $derived(pv?.to ? cat.realmOf(pv.to) : undefined);
	const toStage = $derived(pv?.to ? cat.stages.get(pv.to) : undefined);

	// Which trial guards the target realm (cot-truyen §5.4). Mini games are Phase 3: shown, not yet required.
	const trial = $derived.by(() => {
		if (pv?.tribulation) return { name: pv.tribulation.name, icon: 'bolt', text: 'Thiên kiếp giáng xuống: vượt qua để bước vào cảnh giới mới. Thất bại mất một phần tu vi tầng hiện tại, không tụt tầng.', live: true } as const;
		if (toRealm?.id === 'nguyen_anh') return { name: 'Vượt tâm ma', icon: 'spark', text: 'Ván xếp khối "tâm ma" — trận thử tâm cảnh trước khi kết Nguyên Anh.', live: false } as const;
		if (toRealm?.id === 'hoa_than') return { name: 'Hóa phàm', icon: 'spark', text: 'Trở về phàm trần, chém tạp niệm như chém quả rơi — buông bỏ để hóa thần.', live: false } as const;
		return null;
	});
</script>

<svelte:head><title>Đột Phá · Tiên Đạo</title></svelte:head>

<ScreenFrame title="Đột Phá" subtitle="Thiên địa biến sắc khi bình cảnh vỡ" icon="bolt" bleed>
	<div class="grid h-full min-h-0 overflow-y-auto lg:grid-cols-[minmax(0,1fr)_400px] lg:overflow-hidden">
		<section class="stage relative flex min-h-[380px] flex-col items-center justify-center px-4 py-4 lg:min-h-0" style:--to="var(--color-aura-{toRealm?.visual.aura ?? 'jade'})">
			<div class="skyfire absolute inset-0" class:is-trib={!!pv?.tribulation} aria-hidden="true"></div>
			<div class="relative flex w-full max-w-xl items-center justify-between gap-3">
				<div class="text-center">
					<p class="eyebrow">Hiện tại</p>
					{#if fromRealm}<div class="mt-1"><RealmBadge name={fromRealm.name} aura={fromRealm.visual.aura} size="sm" /></div>{/if}
					<p class="mt-1 text-sm text-mist-300">{cat.stages.get(ch.stageId)?.name}</p>
				</div>
				<Icon name="arrow" size={28} class="text-gold-300" />
				<div class="text-center">
					<p class="eyebrow">Kế tiếp</p>
					{#if toRealm}<div class="mt-1"><RealmBadge name={toRealm.name} aura={toRealm.visual.aura} size="sm" /></div>{:else}<p class="mt-1 text-mist-500">—</p>{/if}
					<p class="mt-1 text-sm text-mist-300">{toStage?.name ?? ''}</p>
				</div>
			</div>
			<div class="scene-box relative w-full min-h-0 flex-1">
				<div class="scene-fit"><CultivationScene aura={toRealm?.visual.aura ?? 'jade'} ready={!!pv?.ready} backlash={game.backlash} /></div>
			</div>
			<p class="relative text-sm text-mist-300 tabular-nums">
				Tu vi: <span class="text-mist-100">{formatPoints(clock.shown)}</span> / {formatPoints(ch.requiredMilli)}
			</p>
		</section>

		<aside class="space-y-4 border-gold-500/15 p-4 lg:overflow-y-auto lg:border-l">
			{#if trial}
				<div class="trial p-3">
					<p class="flex items-center gap-2 font-display text-xl font-bold text-gold-300"><Icon name={trial.icon} size={18} /> {trial.name}</p>
					<p class="mt-1 text-sm text-mist-300">{trial.text}</p>
					{#if !trial.live}<p class="mt-2 text-xs text-mist-500">Trận thử này sẽ khai mở ở bản cập nhật mini game; hiện chưa bắt buộc.</p>{/if}
				</div>
			{/if}
			<BreakthroughPanel preview={pv} catalogue={cat} failStreak={ch.failStreak} busy={game.busy} onattempt={() => void game.attempt()} />
			<p class="text-xs text-mist-500">
				Tỉ lệ đã công bố gồm tâm cảnh sau thất bại và tu vi dư. Pháp bảo, đan dược hộ thân và đồng môn hộ pháp sẽ cộng vào tỉ lệ khi các hệ thống đó khai mở — luôn có trần.
			</p>
		</aside>
	</div>
</ScreenFrame>

<style>
	.skyfire {
		background: radial-gradient(ellipse at 50% 35%, color-mix(in oklab, var(--to) 22%, transparent), transparent 65%);
		animation: breathe 5s ease-in-out infinite;
	}
	.skyfire.is-trib {
		background:
			radial-gradient(ellipse at 50% 10%, color-mix(in oklab, var(--color-spirit-violet) 30%, transparent), transparent 55%),
			radial-gradient(ellipse at 50% 40%, color-mix(in oklab, var(--to) 18%, transparent), transparent 65%);
	}
	@keyframes breathe {
		50% {
			opacity: 0.6;
		}
	}
	.scene-box {
		container-type: size;
		display: grid;
		place-items: center;
		min-height: 220px;
	}
	.scene-fit {
		width: min(100cqw, 133cqh, 480px);
	}
	.trial {
		border: 1px solid color-mix(in oklab, var(--color-gold-500) 30%, transparent);
		background: color-mix(in oklab, var(--color-ink-950) 45%, transparent);
	}
</style>
