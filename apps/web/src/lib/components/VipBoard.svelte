<script lang="ts">
	import Icon from '$lib/components/Icon.svelte';
	import { progressRatio } from '$lib/game/format';
	import type { Snippet } from 'svelte';

	import type { VipActivity, VipActivityToday, VipStatus, VipTiersResponse } from '$lib/game/types';

	/** compact: inside the game window (no page hero). topup: the nạp panel, rendered under the current level. */
	let {
		info,
		status,
		compact = false,
		topup
	}: { info: VipTiersResponse; status: VipStatus | null; compact?: boolean; topup?: Snippet } = $props();
	const tiers = $derived(info.tiers);

	const perkLabel = (p: string) => {
		const [k, id] = p.split(':');
		return (
			{ title: 'Danh hiệu', avatar_frame: 'Khung avatar', chat_aura: 'Hào quang chat', cave_decoration: 'Trang trí động phủ', breakthrough_fx: 'Hiệu ứng đột phá' }[
				k ?? ''
			] ?? k
		) + (id ? ` · ${id.replaceAll('_', ' ')}` : '');
	};
	// Show only perks NEW at each tier.
	const newPerks = (i: number) => {
		const cur = tiers[i]!.benefits;
		const prev = i > 0 ? tiers[i - 1]!.benefits : null;
		const out = cur.cosmetics.filter((c) => !prev?.cosmetics.includes(c)).map(perkLabel);
		if (prev && cur.alchemySlots > prev.alchemySlots) out.push('+1 ô luyện đan');
		for (const f of cur.flags) if (!prev?.flags.includes(f)) out.push(f === 'garden_auto_harvest' ? 'Linh Viên tự thu hoạch' : f);
		return out;
	};
	const cur = $derived(status?.benefits.tier ?? -1);
	const nextReq = $derived(tiers.find((t) => t.tier === status?.nextTier)?.pointsRequired);
	const curReq = $derived(tiers.find((t) => t.tier === cur)?.pointsRequired ?? 0);
	const toNext = $derived(
		status && nextReq !== undefined ? progressRatio(status.points - curReq, nextReq - curReq) : 1
	);
	const nextName = $derived(tiers.find((t) => t.tier === status?.nextTier)?.benefits.name ?? '');
	// Earning sources: today's progress when signed in, the static table otherwise.
	const activities = $derived<(VipActivity & Partial<VipActivityToday>)[]>(status?.activities.length ? status.activities : info.activities);
	const earnCap = $derived(status?.earnDailyCap ?? info.earnDailyCap);
	const earnedRatio = $derived(status ? progressRatio(status.earnedToday, earnCap) : 0);
	const vnd = (n: number) => n.toLocaleString('vi-VN');
</script>

{#if !compact}
<header class="mb-8 text-center">
	<p class="eyebrow">Tiên Duyên Lệnh</p>
	<h1 class="mt-2 text-5xl font-bold text-gold-300">{info.displayName}</h1>
	<p class="mx-auto mt-3 max-w-2xl text-sm text-mist-300">
		Tích {info.pointName} qua tu luyện, nghe giảng, nhiệm vụ ngày — cấp {info.displayName} là vĩnh viễn. Tiên Duyên kéo dài thời gian bế
		quan và mở các đặc quyền tiện ích, ngoại hình, nhưng <strong class="text-mist-100">không</strong> tăng tốc độ tu luyện, tỉ lệ đột phá
		hay điểm Thiên Bảng.
	</p>
</header>
{:else}
<p class="mb-4 text-sm text-mist-300">
	Tiên Duyên kéo dài thời gian bế quan và mở đặc quyền ngoại hình. <strong class="text-mist-100">Không</strong> tăng tốc độ tu luyện, tỉ lệ đột phá hay điểm Thiên Bảng.
</p>
{/if}

{#if status}
	<section class="panel mx-auto mb-8 flex max-w-3xl flex-col items-center gap-6 p-6 sm:flex-row" aria-label="Cấp Tiên Duyên hiện tại">
		<div class="seal grid size-28 shrink-0 place-items-center rounded-full text-gold-300">
			<div class="text-center">
				<Icon name="seal" size={30} class="mx-auto" />
				<p class="mt-1 font-display text-xl leading-none font-bold">{status.benefits.name}</p>
			</div>
		</div>
		<div class="w-full min-w-0 flex-1">
			<p class="eyebrow">Cấp hiện tại</p>
			<p class="mt-1 font-display text-3xl font-bold text-gold-300">{status.benefits.name}</p>
			<p class="mt-1 text-sm text-mist-300">
				{status.points.toLocaleString('vi-VN')} {status.pointName} · Bế quan tối đa
				<span class="font-semibold text-mist-100">{status.benefits.offlineCapHours} canh giờ</span>
			</p>
			{#if status.nextTier !== undefined}
				<div class="mt-3 h-2 bg-ink-950" role="progressbar" aria-label="Tiến độ lên {nextName}" aria-valuemin={0} aria-valuemax={100} aria-valuenow={Math.round(toNext * 100)}>
					<div class="h-full bg-gradient-to-r from-gold-700 to-gold-300" style:width="{toNext * 100}%"></div>
				</div>
				<p class="mt-2 text-sm text-mist-300">
					Còn <span class="font-semibold text-mist-100 tabular-nums">{status.pointsToNext?.toLocaleString('vi-VN')}</span> {status.pointName} để lên {nextName}
				</p>
			{/if}
		</div>
	</section>
{/if}

{@render topup?.()}

<section class="panel mx-auto mb-8 max-w-5xl p-4" aria-labelledby="earn-title">
	<div class="flex flex-wrap items-baseline justify-between gap-2">
		<h2 id="earn-title" class="panel-title">Kết duyên mỗi ngày</h2>
		{#if status}
			<p class="text-sm text-mist-300">
				Hôm nay <span class="font-semibold text-mist-100 tabular-nums">{status.earnedToday} / {earnCap}</span> {info.pointName}
			</p>
		{:else}
			<p class="text-sm text-mist-300">Tối đa <span class="tabular-nums text-mist-100">{earnCap}</span> {info.pointName} mỗi ngày</p>
		{/if}
	</div>
	{#if status}
		<div class="mt-2 h-1.5 bg-ink-950" role="progressbar" aria-label="{info.pointName} hôm nay" aria-valuemin={0} aria-valuemax={earnCap} aria-valuenow={status.earnedToday}>
			<div class="h-full bg-gradient-to-r from-jade-500 to-jade-300" style:width="{earnedRatio * 100}%"></div>
		</div>
	{/if}
	<ul class="mt-3 grid gap-x-6 gap-y-1 sm:grid-cols-2">
		{#each activities as a (a.id)}
			{@const done = a.timesToday ?? 0}
			<li class="earn flex items-center justify-between gap-3 py-1.5 text-sm" class:is-off={!a.live} class:is-full={a.live && done >= a.dailyMaxTimes}>
				<span class="text-mist-100">{a.name}</span>
				<span class="tabular-nums text-mist-300">
					{#if !a.live}
						<span class="text-mist-500">sắp mở</span>
					{:else if status}
						{done}/{a.dailyMaxTimes} lần · +{a.points}
					{:else}
						+{a.points} × {a.dailyMaxTimes}/ngày
					{/if}
				</span>
			</li>
		{/each}
	</ul>
	<p class="mt-3 text-xs text-mist-500">
		Nạp: {vnd(1000)}đ = {info.pointsPer1000Vnd} {info.pointName}, không tính vào trần {earnCap} {info.pointName}/ngày của hoạt động. Hoàn tiền sẽ trừ đúng số {info.pointName} đã cộng, cấp có thể giảm.
	</p>
</section>

<section class="panel mx-auto max-w-5xl p-2 sm:p-4" aria-labelledby="tiers-title">
	<h2 id="tiers-title" class="panel-title px-2 pt-2">Các cấp Tiên Duyên</h2>
	<ol class="mt-3 divide-y divide-gold-500/10">
		{#each tiers as t, i (t.tier)}
			{@const perks = newPerks(i)}
			<li class="tier grid grid-cols-[56px_1fr] items-center gap-x-4 gap-y-1 px-3 py-3 sm:grid-cols-[56px_150px_110px_110px_1fr]" class:is-current={cur === t.tier} class:is-reached={cur > t.tier}>
				<span class="row-span-2 grid size-12 place-items-center rounded-full border text-base font-semibold sm:row-span-1 tier-seal">{t.tier}</span>
				<span class="font-display text-xl font-semibold text-mist-100">
					{t.benefits.name}
					{#if cur === t.tier}<span class="ml-1 align-middle font-sans text-xs font-medium tracking-wider text-gold-300 uppercase">· hiện tại</span>{/if}
				</span>
				<span class="text-sm text-mist-300 sm:text-right"><span class="sm:hidden">Điểm duyên:&nbsp;</span><span class="tabular-nums text-mist-100">{t.pointsRequired.toLocaleString('vi-VN')}</span></span>
				<span class="col-start-2 text-sm text-mist-300 sm:col-start-auto sm:text-right">
					<Icon name="clock" size={14} class="mr-1 inline align-[-2px] text-jade-300" /><span class="tabular-nums text-mist-100">{t.benefits.offlineCapHours}h</span>
					{#if t.benefits.dailyGift}<span class="ml-3 sm:hidden"><Icon name="gift" size={14} class="mr-1 inline align-[-2px] text-gold-500" />{t.benefits.dailyGift}</span>{/if}
				</span>
				<span class="col-start-2 text-sm text-mist-300 sm:col-start-auto">
					<span class="hidden sm:inline">{#if t.benefits.dailyGift}<Icon name="gift" size={14} class="mr-1 inline align-[-2px] text-gold-500" /><span class="mr-3 tabular-nums text-mist-100">{t.benefits.dailyGift}</span>{/if}</span>{perks.join(' · ') || ''}
				</span>
			</li>
		{/each}
	</ol>
</section>

<style>
	.seal {
		background: radial-gradient(circle, color-mix(in oklab, var(--color-gold-500) 22%, var(--color-ink-800)), var(--color-ink-950));
		box-shadow:
			0 0 0 1px var(--color-gold-300),
			0 0 0 6px var(--color-ink-950),
			0 0 0 7px color-mix(in oklab, var(--color-gold-500) 50%, transparent),
			0 0 30px -4px color-mix(in oklab, var(--color-gold-300) 45%, transparent);
	}
	.earn {
		border-bottom: 1px solid color-mix(in oklab, var(--color-gold-500) 10%, transparent);
	}
	.earn.is-off {
		opacity: 0.6;
	}
	.earn.is-full span:last-child {
		color: var(--color-jade-300);
	}
	.tier-seal {
		color: var(--color-mist-500);
		border-color: var(--color-ink-600);
	}
	.is-reached .tier-seal {
		color: var(--color-gold-500);
		border-color: color-mix(in oklab, var(--color-gold-500) 50%, transparent);
	}
	.is-current {
		background: linear-gradient(90deg, color-mix(in oklab, var(--color-gold-500) 16%, transparent), transparent 70%);
		box-shadow: inset 2px 0 0 var(--color-gold-300);
	}
	.is-current .tier-seal {
		color: var(--color-ink-950);
		border-color: var(--color-gold-300);
		background: linear-gradient(180deg, var(--color-gold-300), var(--color-gold-500));
	}
</style>
