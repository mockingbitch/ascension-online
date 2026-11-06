<script lang="ts">
	import Icon from '$lib/components/Icon.svelte';
	import { progressRatio } from '$lib/game/format';

	let { data } = $props();

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
		const cur = data.tiers[i]!.benefits;
		const prev = i > 0 ? data.tiers[i - 1]!.benefits : null;
		const out = cur.cosmetics.filter((c) => !prev?.cosmetics.includes(c)).map(perkLabel);
		if (prev && cur.alchemySlots > prev.alchemySlots) out.push('+1 ô luyện đan');
		for (const f of cur.flags) if (!prev?.flags.includes(f)) out.push(f === 'garden_auto_harvest' ? 'Linh Viên tự thu hoạch' : f);
		return out;
	};
	const cur = $derived(data.status?.benefits.tier ?? -1);
	const nextReq = $derived(data.tiers.find((t) => t.tier === data.status?.nextTier)?.pointsRequired);
	const curReq = $derived(data.tiers.find((t) => t.tier === cur)?.pointsRequired ?? 0);
	const toNext = $derived(
		data.status && nextReq !== undefined ? progressRatio(data.status.points - curReq, nextReq - curReq) : 1
	);
</script>

<svelte:head><title>VIP · Tiên Đạo</title></svelte:head>

<header class="mb-8 text-center">
	<p class="eyebrow">Tiên Duyên Lệnh</p>
	<h1 class="mt-2 text-5xl font-bold text-gold-300">Đặc quyền VIP</h1>
	<p class="mx-auto mt-3 max-w-2xl text-sm text-mist-300">
		VIP kéo dài thời gian bế quan và mở các đặc quyền tiện ích, ngoại hình. VIP <strong class="text-mist-100">không</strong> tăng tốc
		độ tu luyện, tỉ lệ đột phá hay điểm Thiên Bảng.
	</p>
</header>

{#if data.status}
	<section class="panel mx-auto mb-8 flex max-w-3xl flex-col items-center gap-6 p-6 sm:flex-row" aria-label="Cấp VIP hiện tại">
		<div class="seal grid size-28 shrink-0 place-items-center rounded-full text-gold-300">
			<div class="text-center">
				<Icon name="seal" size={30} class="mx-auto" />
				<p class="mt-1 font-display text-xl leading-none font-bold">{data.status.benefits.name}</p>
			</div>
		</div>
		<div class="w-full min-w-0 flex-1">
			<p class="eyebrow">Cấp hiện tại</p>
			<p class="mt-1 font-display text-3xl font-bold text-gold-300">{data.status.benefits.name}</p>
			<p class="mt-1 text-sm text-mist-300">
				{data.status.points.toLocaleString('vi-VN')} điểm · Bế quan tối đa
				<span class="font-semibold text-mist-100">{data.status.benefits.offlineCapHours} canh giờ</span>
			</p>
			{#if data.status.nextTier !== undefined}
				<div class="mt-3 h-2 bg-ink-950" role="progressbar" aria-label="Tiến độ lên VIP {data.status.nextTier}" aria-valuemin={0} aria-valuemax={100} aria-valuenow={Math.round(toNext * 100)}>
					<div class="h-full bg-gradient-to-r from-gold-700 to-gold-300" style:width="{toNext * 100}%"></div>
				</div>
				<p class="mt-2 text-sm text-mist-300">
					Còn <span class="font-semibold text-mist-100 tabular-nums">{data.status.pointsToNext?.toLocaleString('vi-VN')}</span> điểm để lên VIP {data.status.nextTier}
				</p>
			{/if}
		</div>
	</section>
{/if}

<section class="panel mx-auto max-w-5xl p-2 sm:p-4" aria-labelledby="tiers-title">
	<h2 id="tiers-title" class="panel-title px-2 pt-2">Các cấp Tiên Duyên</h2>
	<ol class="mt-3 divide-y divide-gold-500/10">
		{#each data.tiers as t, i (t.tier)}
			{@const perks = newPerks(i)}
			<li class="tier grid grid-cols-[56px_1fr] items-center gap-x-4 gap-y-1 px-3 py-3 sm:grid-cols-[56px_150px_110px_110px_1fr]" class:is-current={cur === t.tier} class:is-reached={cur > t.tier}>
				<span class="row-span-2 grid size-12 place-items-center rounded-full border text-base font-semibold sm:row-span-1 tier-seal">{t.tier}</span>
				<span class="font-display text-xl font-semibold text-mist-100">
					{t.benefits.name}
					{#if cur === t.tier}<span class="ml-1 align-middle font-sans text-xs font-medium tracking-wider text-gold-300 uppercase">· hiện tại</span>{/if}
				</span>
				<span class="text-sm text-mist-300 sm:text-right"><span class="sm:hidden">Điểm:&nbsp;</span><span class="tabular-nums text-mist-100">{t.pointsRequired.toLocaleString('vi-VN')}</span></span>
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
