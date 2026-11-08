<!--
	Gieo quẻ (daily fortune): disclosed odds table, one draw per game day, result card. The server rolls and applies the
	idle-rate buff (until midnight, within the daily caps); this card only shows what the server answered.
	Not the lecture "ngộ đạo" moment of Tử Tiêu Cung.
-->
<script lang="ts">
	import { api, ApiError } from '$lib/api/client';
	import { formatBp } from '$lib/game/format';
	import { game } from '$lib/game/store.svelte';
	import { toast } from '$lib/game/toasts.svelte';
	import type { FortuneView } from '$lib/game/types';
	import { fxBus } from '$lib/engine/fx/bus';
	import FortuneCup from './FortuneCup.svelte';
	import Icon from './Icon.svelte';

	let view = $state<FortuneView | null>(null);
	let drawing = $state(false);
	let showOdds = $state(false);
	/** drawn in this visit: the cup and the odds stay on screen next to the result */
	let drew = $state(false);

	$effect(() => {
		api
			.fortune()
			.then((v) => (view = v))
			.catch(() => (view = null));
	});

	const time = (iso: string) => new Date(iso).toLocaleTimeString('vi-VN', { hour: '2-digit', minute: '2-digit' });

	async function draw() {
		drawing = true;
		drew = true;
		try {
			view = await api.fortuneDraw();
			const o = view.today?.outcome;
			if (o) fxBus.emit('fortune', { outcomeId: o.id, name: o.name, lucky: o.rateBonusBp > 0 });
			if (o) toast('ok', o.rateBonusBp > 0 ? `${o.name}: tốc độ tu luyện +${formatBp(o.rateBonusBp)} tới hết hôm nay.` : `${o.name}: một ngày bình thường.`);
			await game.reload();
		} catch (e) {
			toast('err', e instanceof ApiError ? e.message : String(e));
		} finally {
			drawing = false;
		}
	}
</script>

<section aria-labelledby="fq-title">
	<div class="flex items-baseline justify-between gap-2">
		<h2 id="fq-title" class="panel-title">{view?.name ?? 'Gieo quẻ'}</h2>
		<button class="text-xs text-mist-300 underline-offset-2 hover:underline" onclick={() => (showOdds = !showOdds)} aria-expanded={showOdds}>
			{showOdds ? 'Ẩn tỉ lệ' : 'Xem tỉ lệ'}
		</button>
	</div>
	{#if !view}
		<p class="mt-2 text-sm text-mist-500">Đang tải…</p>
	{:else}
		{#if !view.today || drew}<div class="mt-2"><FortuneCup shaking={drawing} /></div>{/if}
		{#if showOdds || !view.today || drew}
			<table class="odds mt-2 w-full text-xs">
				<caption class="sr-only">Tỉ lệ công bố của mỗi quẻ</caption>
				<thead><tr><th scope="col">Quẻ</th><th scope="col">Tỉ lệ</th><th scope="col">Tốc độ tu luyện</th></tr></thead>
				<tbody>
					{#each view.outcomes as o (o.id)}
						<tr class:is-mine={view.today?.outcome.id === o.id}>
							<td>{o.name}</td>
							<td class="tabular-nums">{formatBp(o.weightBp)}</td>
							<td class="tabular-nums">{o.rateBonusBp > 0 ? `+${formatBp(o.rateBonusBp)}` : '—'}</td>
						</tr>
					{/each}
				</tbody>
			</table>
			<p class="mt-1 text-[11px] leading-snug text-mist-500">
				Mỗi ngày gieo 1 lần. Hiệu lực từ lúc gieo tới hết ngày, chỉ cộng vào tu vi bế quan và vẫn trong hạn mức mỗi ngày.
			</p>
		{/if}
		{#if view.today}
			{@const o = view.today.outcome}
			<div class="result mt-3" class:is-lucky={o.rateBonusBp > 0}>
				<p class="font-display text-2xl text-gold-300">{o.name}</p>
				<p class="mt-1 text-sm leading-relaxed text-mist-100 italic">“{o.verse}”</p>
				<p class="mt-2 text-xs text-jade-300">
					{#if view.buff}Tốc độ tu luyện +{formatBp(view.buff.bonusBp)} tới {time(view.buff.endsAt)}{:else}Không có gia tăng hôm nay{/if}
				</p>
			</div>
		{:else}
			<button class="btn btn-gold mt-3 w-full" disabled={drawing || game.busy} onclick={draw}>
				<Icon name="spark" size={16} /> Gieo quẻ hôm nay
			</button>
		{/if}
	{/if}
</section>

<style>
	.odds th {
		padding: 0.2rem 0;
		font-weight: 500;
		text-align: left;
		color: var(--color-mist-500);
	}
	.odds td {
		padding: 0.2rem 0;
		color: var(--color-mist-300);
		border-top: 1px dashed color-mix(in oklab, var(--color-gold-500) 12%, transparent);
	}
	.odds tr.is-mine td {
		color: var(--color-gold-300);
		font-weight: 600;
	}
	.result {
		padding: 0.75rem 0.9rem;
		text-align: center;
		border: 1px solid color-mix(in oklab, var(--color-gold-500) 35%, transparent);
		background:
			radial-gradient(circle at 50% 0%, color-mix(in oklab, var(--color-gold-300) 14%, transparent), transparent 70%),
			color-mix(in oklab, var(--color-ink-950) 60%, transparent);
	}
	.result.is-lucky {
		box-shadow: 0 0 18px -6px var(--color-gold-300);
	}
	@media (prefers-reduced-motion: no-preference) {
		.result {
			animation: unfold 420ms var(--ease-qi) both;
		}
	}
	@keyframes unfold {
		from {
			opacity: 0;
			transform: translateY(6px) scale(0.98);
		}
	}
</style>
