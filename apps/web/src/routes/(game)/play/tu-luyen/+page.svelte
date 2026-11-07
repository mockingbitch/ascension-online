<!-- Động Phủ · Tu Luyện: the idle-cultivation core. All values are server state; the bar/clock interpolate for display. -->
<script lang="ts">
	import CultivationScene from '$lib/components/CultivationScene.svelte';
	import DailyTasks from '$lib/components/DailyTasks.svelte';
	import FortuneCard from '$lib/components/FortuneCard.svelte';
	import Icon from '$lib/components/Icon.svelte';
	import QiBar from '$lib/components/QiBar.svelte';
	import RealmBadge from '$lib/components/RealmBadge.svelte';
	import ScreenFrame from '$lib/components/ScreenFrame.svelte';
	import { formatBp, formatPoints, ratePerDay } from '$lib/game/format';
	import { useIdleClock } from '$lib/game/idle.svelte';
	import { screens } from '$lib/game/nav';
	import { game } from '$lib/game/store.svelte';

	const clock = useIdleClock();
	const ch = $derived(game.character!);
	const cat = $derived(game.catalogue!);
	const stage = $derived(cat.stages.get(ch.stageId));
	const realm = $derived(cat.realmOf(ch.stageId));
	const aura = $derived(realm?.visual.aura ?? 'jade');
	const subStage = $derived(stage && realm && stage.name.startsWith(realm.name) ? stage.name.slice(realm.name.length).trim() : (stage?.name ?? ''));
	const total = $derived((stage?.cumulativeBeforeMilli ?? 0) + clock.shown);
	const pv = $derived(game.preview);

	// One headline state, most important first.
	const state = $derived.by(() => {
		if (pv?.ready) return { tone: 'gold', icon: 'bolt', text: 'Đủ điều kiện đột phá' } as const;
		if (clock.atBottleneck) return { tone: 'gold', icon: 'lock', text: 'Bình cảnh — cần đột phá' } as const;
		if (clock.isFull) return { tone: 'gold', icon: 'clock', text: 'Linh khí đã đầy — hãy thu nhận' } as const;
		if (ch.today.pointsMilli >= ch.today.ceilingMilli) return { tone: 'mist', icon: 'check', text: 'Đã đạt hạn mức hôm nay' } as const;
		return { tone: 'jade', icon: 'lotus', text: 'Đang tu luyện' } as const;
	});
</script>

<svelte:head><title>Tu Luyện · Tiên Đạo</title></svelte:head>

<ScreenFrame title="Động Phủ" subtitle="Bế quan tu luyện — linh khí tích lũy cả khi bạn rời đi" icon="lotus" bleed>
	<div class="grid h-full min-h-0 overflow-y-auto lg:grid-cols-[minmax(0,1fr)_320px] lg:overflow-hidden">
		<!-- scene + bar + actions -->
		<section class="flex min-h-[420px] flex-col px-3 pt-3 pb-3 sm:px-6 lg:min-h-0">
			<header class="text-center">
				<span class="chip" data-tone={state.tone}><Icon name={state.icon} size={14} /> {state.text}</span>
				<div class="mt-2 flex flex-col items-center gap-1">
					{#if realm}<RealmBadge name={realm.name} aura={realm.visual.aura} size="lg" />{/if}
					<span class="font-display text-lg text-mist-100">{subStage}</span>
				</div>
			</header>

			<div class="scene-box relative min-h-0 flex-1">
				<div class="scene-fit"><CultivationScene {aura} ready={clock.atBottleneck} backlash={game.backlash} decorations={game.character?.appearance?.caveDecorationIds} /></div>
				<div class="pointer-events-none absolute inset-0 flex items-center justify-center" aria-live="polite">
					{#each game.floats as f (f.id)}<span class="float font-display text-3xl font-bold text-jade-300">{f.text}</span>{/each}
				</div>
			</div>

			<div class="mx-auto w-full max-w-xl shrink-0">
				<QiBar current={clock.shown} required={ch.requiredMilli} cap={ch.capMilli} {aura} label="Tu vi · ước tính" />
				<div class="mt-3 grid grid-cols-2 gap-2.5">
					<button class="btn btn-jade h-12 px-2 text-sm whitespace-nowrap sm:text-base" onclick={game.settle} disabled={game.busy} title="Số hiển thị là ước tính — máy chủ kết toán khi bạn thu nhận.">
						<Icon name="lotus" size={18} /> Thu nhận tu vi
					</button>
					<a href={screens.breakthrough.href} class="btn btn-gold h-12 px-2 text-sm whitespace-nowrap sm:text-base" class:is-ready={pv?.ready}>
						<Icon name="bolt" size={18} stroke={1.8} /> Đột phá{#if pv && !pv.missing.some((m) => m.kind === 'final_stage')}<span class="tabular-nums opacity-80">{formatBp(pv.successBp)}</span>{/if}
					</a>
				</div>
			</div>
		</section>

		<!-- details -->
		<aside class="details space-y-4 border-gold-500/15 p-4 lg:overflow-y-auto lg:border-l">
			<dl class="info">
				<div><dt>Cảnh giới</dt><dd style:color="var(--color-aura-{aura})">{realm?.name}</dd></div>
				<div><dt>Tiểu cảnh giới</dt><dd>{subStage}</dd></div>
				<div><dt>Tổng tu vi</dt><dd class="tabular-nums">{formatPoints(total)}</dd></div>
				<div><dt>Tu vi tầng này</dt><dd class="tabular-nums">{formatPoints(clock.shown)} / {formatPoints(ch.requiredMilli)}</dd></div>
				<div><dt>Tốc độ</dt><dd class="text-jade-300 tabular-nums">+{formatPoints(ch.idleRateMilliPerHour)} / giờ <span class="text-xs text-mist-500">({ratePerDay(ch.idleRateMilliPerHour)})</span></dd></div>
				<div><dt>Đã tích lũy</dt><dd class="tabular-nums" class:text-gold-300={clock.isFull}>{clock.elapsedLabel}</dd></div>
				<div><dt>Giới hạn bế quan</dt><dd class="tabular-nums">{Math.round(clock.capSeconds / 3600)} canh giờ{#if ch.offlineBonusSeconds > 0}<span class="block text-xs text-jade-300">gồm +{ch.offlineBonusSeconds / 3600}h Tụ Linh Phù</span>{/if}</dd></div>
				<div><dt>Còn tới khi đầy</dt><dd class="tabular-nums">{clock.isFull ? 'Đã đầy' : clock.untilFullLabel}</dd></div>
				<div>
					<dt>Hôm nay</dt>
					<dd class="tabular-nums" class:text-gold-300={ch.today.pointsMilli >= ch.today.ceilingMilli}>{formatPoints(ch.today.pointsMilli)} / {formatPoints(ch.today.ceilingMilli)} điểm</dd>
				</div>
			</dl>
			{#if ch.today.catchUp.active}
				<p class="text-xs text-gold-300">Thiên đạo bù đắp người đến sau: tu vi nhận ×{ch.today.catchUp.multiplierBp / 10000}.</p>
			{/if}
			{#if ch.offlineBonusSeconds <= 0 && game.phuCount > 0 && !ch.bonusUsedToday}
				<button class="btn btn-ghost w-full" disabled={game.busy} onclick={game.usePhu}><Icon name="talisman" size={16} /> Dùng Tụ Linh Phù (+4 canh giờ)</button>
			{/if}
			<div class="border-t border-gold-500/15 pt-3">
				<FortuneCard />
			</div>
			<div class="border-t border-gold-500/15 pt-3">
				<DailyTasks board={game.board} catalogue={cat} busy={game.busy} onclaim={game.claimTask} onincense={game.offerIncense} />
			</div>
		</aside>
	</div>
</ScreenFrame>

<style>
	.scene-box {
		container-type: size;
		display: grid;
		place-items: center;
		min-height: 220px;
	}
	.scene-fit {
		width: min(100cqw, 133cqh, 520px);
	}
	.chip {
		display: inline-flex;
		align-items: center;
		gap: 0.35rem;
		padding: 2px 10px;
		font-size: 0.75rem;
		font-weight: 600;
		border: 1px solid currentColor;
		border-radius: 9999px;
		background: color-mix(in oklab, var(--color-ink-950) 60%, transparent);
	}
	.chip[data-tone='jade'] {
		color: var(--color-jade-300);
	}
	.chip[data-tone='gold'] {
		color: var(--color-gold-300);
	}
	.chip[data-tone='mist'] {
		color: var(--color-mist-300);
	}
	.info > div {
		display: flex;
		justify-content: space-between;
		gap: 1rem;
		padding: 0.4rem 0;
		font-size: 0.875rem;
		border-bottom: 1px dashed color-mix(in oklab, var(--color-gold-500) 12%, transparent);
	}
	.info dt {
		color: var(--color-mist-300);
	}
	.info dd {
		text-align: right;
		font-weight: 600;
		color: var(--color-mist-100);
	}
	.float {
		position: absolute;
		animation: float-up 1.6s var(--ease-qi) forwards;
		text-shadow: 0 0 12px var(--color-jade-500);
	}
	@keyframes float-up {
		from {
			opacity: 0;
			transform: translateY(20px) scale(0.9);
		}
		20% {
			opacity: 1;
			transform: translateY(0) scale(1);
		}
		to {
			opacity: 0;
			transform: translateY(-60px);
		}
	}
	.is-ready {
		animation: ready-glow 2.4s ease-in-out infinite;
	}
	@keyframes ready-glow {
		50% {
			box-shadow: 0 0 22px 2px color-mix(in oklab, var(--color-gold-300) 45%, transparent);
		}
	}
</style>
