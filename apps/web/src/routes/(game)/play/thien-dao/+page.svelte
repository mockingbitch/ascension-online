<!-- Thiên Đạo Sử: the server story clock (GET /world) — era, progress per active player toward the next milestone
     with its safety window, ranged events, and the timeline. Card text: lib/game/timeline.ts (from the story doc). -->
<script lang="ts">
	import Icon from '$lib/components/Icon.svelte';
	import ScreenFrame from '$lib/components/ScreenFrame.svelte';
	import { formatBp, formatPoints } from '$lib/game/format';
	import { milestoneBlurbs } from '$lib/game/timeline';
	import { loadWorld, world } from '$lib/game/world.svelte';

	const v = $derived(world.view);
	const next = $derived(v?.nextMilestone ?? null);
	const state = (id: string, reached: boolean) => (reached ? 'done' : id === next?.id ? 'next' : 'future');
	const date = (iso: string | null | undefined) =>
		iso ? new Date(iso).toLocaleDateString('vi-VN', { day: '2-digit', month: '2-digit', year: 'numeric' }) : '—';
	const reasonText: Record<string, string> = { threshold: 'đủ tiến độ', max_time: 'đến hạn', admin: 'Thiên Đạo định' };
</script>

<svelte:head><title>Thiên Đạo Sử · Tiên Đạo</title></svelte:head>

<ScreenFrame title="Thiên Đạo Sử" subtitle="Cả cõi cùng tu — lịch sử Hồng Hoang đổi theo tu vi của chúng sinh" icon="scroll">
	{#if !v}
		<p class="py-10 text-center text-mist-300">
			{#if world.error}
				Không đọc được thiên cơ.
				<button type="button" class="text-jade-300 hover:underline" onclick={() => loadWorld(true)}>Thử lại</button>
			{:else}
				Đang dò thiên cơ…
			{/if}
		</p>
	{:else}
		<section class="progress p-4 sm:p-5">
			{#if !v.started}
				<p class="eyebrow">Thế giới chưa khai mở</p>
				<h2 class="mt-1 font-display text-3xl font-bold text-gold-300">Tử Tiêu Cung sắp mở cửa</h2>
				<p class="mt-1 text-sm text-mist-300">Ngày khai mở, Hồng Quân giảng đạo lần đầu và dòng thời gian bắt đầu chuyển.</p>
			{:else}
				<p class="eyebrow">
					Ngày thứ {v.serverDay} của thế giới{#if v.era} · Thời kỳ {v.era.name}{/if}{#if v.progress.yesterdayDau}
						· hôm qua {v.progress.yesterdayDau.toLocaleString('vi-VN')} đạo hữu tu luyện{/if}
				</p>
				{#if next}
					<h2 class="mt-1 font-display text-3xl font-bold text-gold-300">Sắp tới: {next.name}</h2>
					{#if milestoneBlurbs[next.id]}<p class="mt-1 text-sm text-mist-300">{milestoneBlurbs[next.id]}</p>{/if}
					<div
						class="mt-4 h-3 overflow-hidden bg-ink-950"
						role="progressbar"
						aria-label="Tiến độ tới mốc kế tiếp"
						aria-valuemin={0}
						aria-valuemax={100}
						aria-valuenow={Math.round(next.progressBp / 100)}
					>
						<div class="bar h-full" style:width="{next.progressBp / 100}%"></div>
					</div>
					<div class="mt-2 flex flex-wrap justify-between gap-2 text-sm text-mist-300 tabular-nums">
						<span>
							Tu vi trung bình mỗi đạo hữu: <strong class="text-mist-100">{formatPoints(v.progress.perActiveMilli)}</strong> /
							{formatPoints(next.thresholdPerActiveMilli)} ({formatBp(next.progressBp)})
						</span>
						{#if v.progress.todayProvisionalMilli}
							<span>Hôm nay đang góp: +{formatPoints(v.progress.todayProvisionalMilli)} (chốt sau nửa đêm)</span>
						{/if}
					</div>
					<p class="mt-2 text-sm text-mist-300 tabular-nums">
						Dự kiến: <strong class="text-mist-100">{next.estimatedAt ? date(next.estimatedAt) : 'chưa đủ dữ liệu'}</strong>
						· sớm nhất {date(next.earliestAt)} · muộn nhất {date(next.latestAt)}
					</p>
				{:else}
					<h2 class="mt-1 font-display text-3xl font-bold text-gold-300">Mọi mốc đã qua</h2>
				{/if}
				{#if v.paused}
					<p class="mt-3 text-sm text-cinnabar-300"><Icon name="lock" size={14} /> Thiên Đạo đang tạm dừng: mốc mới chưa đến cho tới khi mở lại.</p>
				{/if}
				<p class="mt-3 text-xs text-mist-500">
					Mỗi ngày bạn tu luyện, tu vi của bạn góp vào tiến độ chung (có giới hạn mỗi người mỗi ngày, tiến độ chỉ tính những ngày đã
					chốt). Mốc không đến trước ngày sớm nhất và chắc chắn đến trước ngày muộn nhất.
				</p>
			{/if}
		</section>

		{#if v.activeEvents.length}
			<section class="mt-4 grid gap-2 sm:grid-cols-2">
				{#each v.activeEvents as e (e.id)}
					<div class="event flex items-center gap-3 px-3 py-2">
						<Icon name="spark" size={18} class="shrink-0 text-gold-500" />
						<div class="min-w-0">
							<p class="truncate font-semibold text-mist-100">{e.name}</p>
							<p class="text-xs text-mist-300 tabular-nums">Đang diễn ra · kết thúc {date(e.endsAt)}</p>
						</div>
					</div>
				{/each}
			</section>
		{/if}

		<ol class="timeline mt-6">
			{#each v.timeline as m (m.id)}
				{@const st = state(m.id, m.reached)}
				<li class="item" data-state={st}>
					<span class="node" aria-hidden="true">
						{#if st === 'done'}<Icon name="check" size={14} />{:else if st === 'next'}<Icon name="spark" size={14} />{/if}
					</span>
					<div class="min-w-0">
						<p class="text-xs text-mist-500 tabular-nums">
							{#if m.reached}
								Đã diễn ra ngày {m.reachedDay}{#if v.lastMilestone?.id === m.id} · {reasonText[v.lastMilestone.reason]}{/if}
							{:else}
								Dự kiến ngày {m.targetDay}{m.eventDays ? `–${m.targetDay + m.eventDays}` : ''} · {st === 'next' ? 'Đang đến gần' : 'Chưa tới'}
							{/if}
						</p>
						<h3 class="font-display text-xl font-bold">{m.name}</h3>
						{#if milestoneBlurbs[m.id]}<p class="mt-0.5 text-sm text-mist-300">{milestoneBlurbs[m.id]}</p>{/if}
					</div>
				</li>
			{/each}
		</ol>
	{/if}
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
	.event {
		border: 1px solid color-mix(in oklab, var(--color-gold-500) 28%, transparent);
		background: color-mix(in oklab, var(--color-ink-950) 45%, transparent);
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
