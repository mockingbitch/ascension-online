<!-- Thiên Đạo Sử (preview): the server story clock — community progress toward the next milestone + the timeline.
     Milestones/blurbs are the draft canon; progress numbers are sample until GET /world exists (Phase 2.1). -->
<script lang="ts">
	import Icon from '$lib/components/Icon.svelte';
	import ScreenFrame from '$lib/components/ScreenFrame.svelte';
	import { milestones, sampleWorld } from '$lib/mock/timeline';

	const w = sampleWorld;
	const nextIdx = $derived(milestones.findIndex((m) => m.targetDay > w.serverDay));
	const next = $derived(milestones[nextIdx]);
	const ratio = $derived(Math.min(1, w.communityPoints / w.nextThresholdPoints));
	const state = (i: number) => (i < nextIdx ? 'done' : i === nextIdx ? 'next' : 'future');
</script>

<svelte:head><title>Thiên Đạo Sử · Tiên Đạo</title></svelte:head>

<ScreenFrame title="Thiên Đạo Sử" subtitle="Cả cõi cùng tu — lịch sử Thiên Nguyên Giới đổi theo tổng tu vi của chúng sinh" icon="scroll" status="preview">
	{#if next}
		<section class="progress p-4 sm:p-5">
			<p class="eyebrow">Ngày thứ {w.serverDay} của thế giới · {w.activePlayers.toLocaleString('vi-VN')} đạo hữu đang tu luyện</p>
			<h2 class="mt-1 font-display text-3xl font-bold text-gold-300">Sắp tới: {next.title}</h2>
			<p class="mt-1 text-sm text-mist-300">{next.blurb}</p>
			<div class="mt-4 h-3 overflow-hidden bg-ink-950" role="progressbar" aria-label="Tiến độ tới mốc kế tiếp" aria-valuemin={0} aria-valuemax={100} aria-valuenow={Math.round(ratio * 100)}>
				<div class="bar h-full" style:width="{ratio * 100}%"></div>
			</div>
			<div class="mt-2 flex flex-wrap justify-between gap-2 text-sm text-mist-300 tabular-nums">
				<span>Tổng tu vi chúng sinh: <strong class="text-mist-100">{w.communityPoints.toLocaleString('vi-VN')}</strong> / {w.nextThresholdPoints.toLocaleString('vi-VN')}</span>
				<span>Dự kiến: khoảng ngày {next.targetDay} (sớm nhất ngày {Math.ceil(next.targetDay * 0.7)})</span>
			</div>
			<p class="mt-3 text-xs text-mist-500">
				Mỗi ngày bạn tu luyện, tu vi của bạn góp vào tiến trình chung (tối đa 100 điểm/người/ngày). Mốc không đến trước 70% và chắc chắn đến trước 150% ngày dự kiến.
			</p>
		</section>
	{/if}

	<ol class="timeline mt-6">
		{#each milestones as m, i (m.slug)}
			<li class="item" data-state={state(i)}>
				<span class="node" aria-hidden="true">
					{#if state(i) === 'done'}<Icon name="check" size={14} />{:else if state(i) === 'next'}<Icon name="spark" size={14} />{/if}
				</span>
				<div class="min-w-0">
					<p class="text-xs text-mist-500 tabular-nums">Ngày {m.targetDay}{m.untilDay ? `–${m.untilDay}` : ''} · {state(i) === 'done' ? 'Đã diễn ra' : state(i) === 'next' ? 'Đang đến gần' : 'Chưa tới'}</p>
					<h3 class="font-display text-xl font-bold">{m.title}</h3>
					<p class="mt-0.5 text-sm text-mist-300">{m.blurb}</p>
				</div>
			</li>
		{/each}
	</ol>
</ScreenFrame>

<style>
	.progress {
		border: 1px solid color-mix(in oklab, var(--color-gold-500) 35%, transparent);
		background:
			radial-gradient(ellipse at 80% 0%, color-mix(in oklab, var(--color-gold-300) 14%, transparent), transparent 60%),
			color-mix(in oklab, var(--color-ink-950) 45%, transparent);
	}
	.bar {
		background: linear-gradient(90deg, var(--color-gold-700), var(--color-gold-300));
		box-shadow: 0 0 14px var(--color-gold-500);
	}
	.timeline {
		position: relative;
		padding-left: 2.25rem;
	}
	.timeline::before {
		content: '';
		position: absolute;
		left: 0.8rem;
		top: 0.5rem;
		bottom: 0.5rem;
		width: 2px;
		background: linear-gradient(180deg, var(--color-gold-500), color-mix(in oklab, var(--color-gold-500) 15%, transparent));
	}
	.item {
		position: relative;
		display: flex;
		gap: 1rem;
		padding: 0.75rem 0;
	}
	.node {
		position: absolute;
		left: -2.25rem;
		top: 1rem;
		display: grid;
		width: 1.75rem;
		height: 1.75rem;
		place-items: center;
		border-radius: 9999px;
		border: 2px solid var(--color-ink-600);
		background: var(--color-ink-950);
		color: var(--color-ink-950);
	}
	.item[data-state='done'] .node {
		border-color: var(--color-gold-500);
		background: var(--color-gold-500);
	}
	.item[data-state='done'] h3 {
		color: var(--color-mist-300);
	}
	.item[data-state='next'] .node {
		border-color: var(--color-gold-300);
		color: var(--color-gold-300);
		box-shadow: 0 0 16px var(--color-gold-500);
		animation: var(--animate-qi-breathe);
	}
	.item[data-state='next'] h3 {
		color: var(--color-gold-300);
	}
	.item[data-state='future'] h3 {
		color: var(--color-mist-100);
	}
</style>
