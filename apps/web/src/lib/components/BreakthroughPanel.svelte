<script lang="ts">
	import type { CatalogueIndex } from '$lib/game/catalogue';
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

	const fmtTime = (iso?: string) => (iso ? new Date(iso).toLocaleTimeString('vi-VN', { hour: '2-digit', minute: '2-digit' }) : '');
	const target = $derived(preview?.to ? catalogue.stages.get(preview.to) : undefined);
	const trib = $derived(preview?.tribulation);
	const oddsLabel: Record<string, string> = { base: 'cơ bản', pity: 'tâm cảnh', surplus: 'dư tu vi', items: 'pháp bảo', protector: 'hộ pháp' };
	const bonusParts = $derived((preview?.odds ?? []).filter((o) => o.source !== 'base' && o.bp > 0));
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
			<div class="text-right">
				<p class="eyebrow">Tỉ lệ</p>
				<p class="mt-1 text-3xl leading-none font-semibold text-jade-300 tabular-nums">{formatBp(preview.successBp)}</p>
			</div>
		</div>
		{#if bonusParts.length && !trib?.guaranteed}
			<p class="mt-1 text-right text-xs text-mist-500">
				{#each bonusParts as o, i (o.source)}{i ? ' · ' : ''}+{formatBp(o.bp)} {oddsLabel[o.source] ?? o.source}{/each}
			</p>
		{/if}
		{#if trib}
			<div class="trib mt-3 space-y-1 px-3 py-2 text-sm">
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
			<button class="btn btn-gold h-12 w-full text-base" class:is-ready={preview.ready && !busy} disabled={!preview.ready || busy} onclick={onattempt}>
				<Icon name="bolt" size={18} stroke={1.8} />
				{busy ? 'Đang trùng kích…' : trib ? `Độ ${trib.name}` : 'Trùng kích bình cảnh'}
			</button>
		{/if}
	</div>
</section>

<style>
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
