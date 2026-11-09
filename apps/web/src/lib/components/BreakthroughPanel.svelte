<script lang="ts">
	import type { CatalogueIndex } from '$lib/game/catalogue';
	import { trialRoutes } from '$lib/game/nav';
	import { formatBp, formatPoints } from '$lib/game/format';
	import type { BreakthroughPreview } from '$lib/game/types';
	import type { Snippet } from 'svelte';
	import Icon from './Icon.svelte';

	interface Props {
		preview: BreakthroughPreview | null;
		catalogue: CatalogueIndex;
		failStreak: number;
		busy: boolean;
		onattempt: () => void;
		/** Secondary action rendered beside the breakthrough button (e.g. harvest). */
		children?: Snippet;
	}
	let { preview, catalogue, failStreak, busy, onattempt, children }: Props = $props();
	const canTry = $derived(!!preview && !preview.missing.some((m) => m.kind === 'final_stage'));

	const fmtDay = (iso?: string | null) =>
		iso ? new Date(iso).toLocaleString('vi-VN', { hour: '2-digit', minute: '2-digit', day: '2-digit', month: '2-digit' }) : '';
	const fmtTime = (iso?: string) => (iso ? new Date(iso).toLocaleTimeString('vi-VN', { hour: '2-digit', minute: '2-digit' }) : '');
	const target = $derived(preview?.to ? catalogue.stages.get(preview.to) : undefined);
	const trib = $derived(preview?.tribulation);
	const oddsLabel: Record<string, string> = {
		base: 'cơ bản',
		pity: 'tâm cảnh',
		surplus: 'dư tu vi',
		items: 'pháp bảo',
		protector: 'hộ pháp',
		bonus_cap: 'trần cộng thêm',
		max_cap: 'trần tỉ lệ',
		guarantee: 'bảo hiểm'
	};
	const bonusParts = $derived((preview?.odds ?? []).filter((o) => o.source !== 'base' && o.bp !== 0));
	const caps = $derived(preview?.caps);
	const part = (src: string) => preview?.odds.find((o) => o.source === src);
	// Tribulation breakdown rows: every component with its own cap, then the caps that bit (parts sum to successBp).
	const rows = $derived.by(() => {
		if (!caps || !preview) return [];
		const out: { label: string; bp: number; note: string }[] = [{ label: 'Cơ bản', bp: part('base')?.bp ?? 0, note: '' }];
		out.push({ label: 'Dư tu vi', bp: part('surplus')?.bp ?? 0, note: `trần +${formatBp(caps.surplusMaxBp)}` });
		const art = caps.artifactId ? catalogue.itemName(caps.artifactId) : 'chưa có pháp bảo hộ thân';
		out.push({ label: 'Pháp bảo', bp: part('items')?.bp ?? 0, note: `${art} · trần +${formatBp(caps.itemMaxBp)}` });
		out.push({
			label: 'Hộ pháp',
			bp: part('protector')?.bp ?? 0,
			note: `${caps.protectorMembers} đồng môn × ${formatBp(caps.protectorPerMemberBp)} · trần +${formatBp(caps.protectorMaxBp)}`
		});
		const bc = part('bonus_cap');
		if (bc) out.push({ label: 'Trần cộng thêm', bp: bc.bp, note: `tổng cộng thêm tối đa +${formatBp(caps.totalBonusMaxBp)}` });
		const mc = part('max_cap');
		if (mc) out.push({ label: 'Trần tỉ lệ', bp: mc.bp, note: `tối đa ${formatBp(caps.maxSuccessBp)} (trừ lần bảo hiểm)` });
		const g = part('guarantee');
		if (g) out.push({ label: 'Bảo hiểm', bp: g.bp, note: 'thất bại liên tiếp đủ lần' });
		return out;
	});
	const targetRealm = $derived(preview?.to ? catalogue.realmOf(preview.to) : undefined);
</script>

<section aria-labelledby="bt-title">
	<h2 id="bt-title" class="panel-title">
		Đột phá
		{#if preview?.kind}
			<span class="order-last font-sans text-xs font-medium tracking-[0.18em] uppercase" class:text-gold-300={preview.kind === 'major'} class:text-mist-500={preview.kind !== 'major'}>
				{trib ? trib.name : 'Đại cảnh giới'}
			</span>
		{/if}
	</h2>

	{#if !preview}
		<p class="mt-3 text-sm text-mist-500">Đang dò xét bình cảnh…</p>
	{:else if preview.missing.some((m) => m.kind === 'final_stage')}
		<p class="mt-3 text-mist-300">Đã đạt đỉnh cảnh giới hiện có. Thiên Môn chưa mở.</p>
	{:else}
		<div class="mt-3 flex items-end justify-between gap-4">
			<div class="min-w-0">
				<p class="eyebrow">Mục tiêu</p>
				<p class="mt-1 truncate font-display text-2xl leading-tight font-bold" style:color="var(--color-aura-{targetRealm?.visual.aura ?? 'none'})">
					{targetRealm?.name}
					<span class="text-mist-100">· {target && targetRealm && target.name.startsWith(targetRealm.name) ? target.name.slice(targetRealm.name.length).trim() : target?.name}</span>
				</p>
			</div>
			<div class="text-right" data-tutorial="bt-odds">
				<p class="eyebrow">Tỉ lệ</p>
				<p class="mt-1 text-3xl leading-none font-semibold text-jade-300 tabular-nums">{formatBp(preview.successBp)}</p>
			</div>
		</div>
		{#if rows.length}
			<table class="odds mt-2 w-full text-xs" aria-label="Tỉ lệ vượt kiếp từng phần">
				<tbody>
					{#each rows as r (r.label)}
						<tr>
							<td class="py-0.5 pr-2 text-mist-300">{r.label}</td>
							<td class="pr-2 text-mist-500">{r.note}</td>
							<td class="text-right tabular-nums" class:text-jade-300={r.bp > 0 && r.label !== 'Cơ bản'} class:text-cinnabar-300={r.bp < 0} class:text-mist-100={r.label === 'Cơ bản'}>
								{r.bp > 0 && r.label !== 'Cơ bản' ? '+' : ''}{formatBp(r.bp)}
							</td>
						</tr>
					{/each}
					<tr class="total">
						<td class="pt-1 pr-2 font-semibold text-mist-100">Tổng</td>
						<td></td>
						<td class="pt-1 text-right font-semibold text-jade-300 tabular-nums">{formatBp(preview.successBp)}</td>
					</tr>
				</tbody>
			</table>
		{:else if bonusParts.length && !trib?.guaranteed}
			<p class="mt-1 text-right text-xs text-mist-500">
				{#each bonusParts as o, i (o.source)}{i ? ' · ' : ''}{o.bp > 0 ? '+' : ''}{formatBp(o.bp)} {oddsLabel[o.source] ?? o.source}{/each}
			</p>
		{/if}
		{#if trib}
			<div class="trib mt-3 space-y-1 px-3 py-2 text-sm" data-tutorial="bt-loss">
				{#if trib.guaranteed}
					<p class="flex items-center gap-2 text-jade-300"><Icon name="check" size={16} /> Thiên đạo thương xót: lần độ kiếp này chắc chắn thành công.</p>
				{:else}
					<p class="flex items-center gap-2 text-cinnabar-300">
						<Icon name="bolt" size={16} /> Thất bại sẽ mất <span class="font-semibold text-mist-100 tabular-nums">{formatPoints(trib.lossMilli)}</span> tu vi
						<span class="text-mist-500">({formatBp(trib.lossBp)} tu vi tầng hiện tại, không tụt tầng)</span>
					</p>
				{/if}
				<p class="text-mist-300">
					Bảo hiểm: <span class="font-semibold text-mist-100 tabular-nums">{Math.min(trib.failStreak, trib.guaranteeAfter)}/{trib.guaranteeAfter}</span>
					<span class="text-mist-500">— thất bại {trib.guaranteeAfter} lần liên tiếp thì lần sau chắc chắn qua · chờ {Math.round(trib.cooldownSeconds / 3600)} canh giờ sau mỗi lần thất bại</span>
				</p>
			</div>
		{:else if failStreak > 0}
			<p class="mt-1 text-sm text-mist-300">Tâm cảnh vững thêm sau {failStreak} lần thất bại — đã cộng vào tỉ lệ.</p>
		{/if}

		{#if preview.trial?.passed}
			<p class="mt-2 flex items-center gap-2 text-sm text-mist-300">
				<Icon name="check" size={16} class="text-gold-300" />
				Đã {preview.trial.name.toLowerCase()} — ấn chứng tới {fmtDay(preview.trial.validUntil)}, giữ qua lần thất bại.
			</p>
		{/if}

		{#if preview.consumes.length}
			<p class="mt-2 text-sm text-mist-300">
				Tiêu hao (kể cả khi thất bại):
				{#each preview.consumes as c, i (c.itemId)}{i ? ', ' : ''}<span class="text-mist-100">{catalogue.itemName(c.itemId)} ×{c.quantity}</span>{/each}
			</p>
		{/if}

		{#if preview.missing.length}
			<ul class="mt-3 space-y-1.5 text-sm">
				{#each preview.missing as m, i (i)}
					<li class="flex items-center gap-2 text-mist-300">
						{#if m.kind === 'cooldown'}
							<Icon name="clock" size={16} class="text-gold-500" />
							Kinh mạch đang hồi phục, thử lại lúc {fmtTime(m.readyAt)}
						{:else}
							<Icon name="cross" size={16} class="text-cinnabar-300" />
							{#if m.kind === 'cultivation'}
								Tu vi còn thiếu <span class="text-mist-100 tabular-nums">{formatPoints(m.need - m.have)}</span> (tới Viên Mãn cảnh giới)
							{:else if m.kind === 'item'}
								Thiếu {catalogue.itemName(m.itemId ?? '')}: {m.have}/{m.need}
							{:else if m.kind === 'trial'}
								<span>
									Chưa {preview.trial?.name.toLowerCase() ?? 'vượt thử luyện'}
									{#if trialRoutes[m.trialKind ?? '']}
										— <a class="text-jade-300 underline underline-offset-2" href={trialRoutes[m.trialKind ?? '']}>vào thử luyện</a>
									{/if}
								</span>
							{/if}
						{/if}
					</li>
				{/each}
			</ul>
		{/if}

	{/if}

	<div class={['mt-4 grid gap-3', children && 'sm:grid-cols-2']}>
		{@render children?.()}
		{#if canTry && preview}
			<button class="btn btn-gold h-12 w-full text-base" class:is-ready={preview.ready && !busy} disabled={!preview.ready || busy} onclick={onattempt} data-tutorial="bt-attempt">
				<Icon name="bolt" size={18} stroke={1.8} />
				{busy ? 'Đang trùng kích…' : trib ? `Độ ${trib.name}` : 'Trùng kích bình cảnh'}
			</button>
		{/if}
	</div>
</section>

<style>
	.odds td {
		vertical-align: top;
	}
	.odds .total td {
		border-top: 1px solid var(--color-ink-700);
	}
	.trib {
		border-left: 2px solid color-mix(in oklab, var(--color-cinnabar-300) 70%, transparent);
		background: color-mix(in oklab, var(--color-ink-950) 55%, transparent);
	}
	.is-ready {
		animation: ready-glow 2.4s ease-in-out infinite;
	}
	@keyframes ready-glow {
		0%,
		100% {
			box-shadow:
				inset 0 1px 0 color-mix(in oklab, white 35%, transparent),
				0 0 0 0 color-mix(in oklab, var(--color-gold-300) 0%, transparent);
		}
		50% {
			box-shadow:
				inset 0 1px 0 color-mix(in oklab, white 35%, transparent),
				0 0 22px 2px color-mix(in oklab, var(--color-gold-300) 45%, transparent);
		}
	}
</style>
