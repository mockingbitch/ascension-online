<script lang="ts">
	import type { CatalogueIndex } from '$lib/game/catalogue';
	import type { DailyBoard, TaskReward } from '$lib/game/types';
	import Icon from './Icon.svelte';

	interface Props {
		board: DailyBoard | null;
		catalogue: CatalogueIndex;
		busy: boolean;
		onclaim: (taskId: string) => void;
	}
	let { board, catalogue, busy, onclaim }: Props = $props();

	const reward = (r: TaskReward) =>
		r.itemId ? `${catalogue.itemName(r.itemId)} ×${r.quantity}` : `${r.amount} ${r.currency === 'spirit_stone' ? 'linh thạch' : r.currency}`;
	const resetTime = (iso: string) => new Date(iso).toLocaleTimeString('vi-VN', { hour: '2-digit', minute: '2-digit' });
</script>

<section aria-labelledby="dt-title">
	<h2 id="dt-title" class="panel-title">Nhiệm vụ ngày</h2>
	{#if board}<p class="mt-1 text-xs text-mist-500">Làm mới lúc {resetTime(board.resetsAt)}</p>{/if}
	{#if !board}
		<p class="mt-3 text-sm text-mist-500">Đang tải…</p>
	{:else}
		<ul class="mt-3 divide-y divide-gold-500/10">
			{#each board.tasks as t (t.id)}
				{@const pct = Math.min(1, t.progress / t.target)}
				<li class="flex items-center gap-3 py-3" class:opacity-60={t.claimed}>
					<div class="min-w-0 flex-1">
						<p class="flex items-baseline gap-2 text-mist-100">
							<span class="font-medium">{t.name}</span>
							<span class="text-xs text-mist-500 tabular-nums">{Math.min(t.progress, t.target)}/{t.target}</span>
						</p>
						<p class="text-xs text-mist-500">{t.description}</p>
						<div class="mt-1.5 h-1 bg-ink-950" aria-hidden="true"><div class="h-full bg-jade-500" style:width="{pct * 100}%"></div></div>
						<p class="mt-1 text-xs text-jade-300">{t.rewards.map(reward).join(' · ')}</p>
					</div>
					{#if t.claimed}
						<span class="flex items-center gap-1 text-sm text-jade-300"><Icon name="check" size={16} />Đã nhận</span>
					{:else}
						<button class="btn btn-jade h-10 px-3 text-sm" disabled={!t.complete || busy} onclick={() => onclaim(t.id)} title={t.description}>
							Nhận
						</button>
					{/if}
				</li>
			{/each}
		</ul>
	{/if}
</section>
