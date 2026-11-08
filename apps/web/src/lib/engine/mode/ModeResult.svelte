<!--
	Result screen of a mini-game mode (spec §4.9): a pass / fail badge (icon + text, never colour alone), the heading,
	the server's stats, the reward lines (they fly into the bag from here: `data-fx-source`, target = the bag button),
	and "Chơi lại" / "Về". Shown only after the server's verdict (or a local practice end). Display only.
-->
<script lang="ts" module>
	export type Verdict = 'pass' | 'fail' | 'void' | 'practice';
</script>

<script lang="ts">
	import type { Snippet } from 'svelte';
	import Icon from '$lib/components/Icon.svelte';
	interface Props {
		verdict: Verdict;
		heading: string;
		reason?: string;
		stats?: { label: string; value: string }[];
		rewards?: { label: string; rarity?: string; artifact?: boolean }[];
		replayLabel?: string;
		onreplay: () => void;
		onback: () => void;
		/** extra lines (rules notes, cooldown, pass validity…) */
		children?: Snippet;
		/** a primary next step (e.g. "Tới Đột Phá") */
		actions?: Snippet;
	}
	let { verdict, heading, reason, stats = [], rewards, replayLabel = 'Chơi lại', onreplay, onback, children, actions }: Props = $props();

	const badge: Record<Verdict, { icon: 'check' | 'cross' | 'lotus'; text: string }> = {
		pass: { icon: 'check', text: 'Đạt' },
		fail: { icon: 'cross', text: 'Chưa đạt' },
		void: { icon: 'cross', text: 'Không ghi nhận' },
		practice: { icon: 'lotus', text: 'Luyện tập' }
	};
	const b = $derived(badge[verdict]);
</script>

<div class="result" data-mode-result={verdict}>
	<div class="card panel">
		<p class="badge" data-tone={verdict}><Icon name={b.icon} size={16} /> {b.text}</p>
		<h2 class="mt-3 font-display text-3xl font-bold" class:text-gold-300={verdict === 'pass'} class:text-mist-100={verdict !== 'pass'}>{heading}</h2>
		{#if reason}<p class="mt-2 text-mist-300">{reason}</p>{/if}
		{#if stats.length}
			<dl class="stats mt-4 grid gap-2" style:grid-template-columns="repeat({Math.min(3, stats.length)}, minmax(0, 1fr))">
				{#each stats as st (st.label)}
					<div>
						<dt>{st.label}</dt>
						<dd class="tabular-nums">{st.value}</dd>
					</div>
				{/each}
			</dl>
		{/if}
		{#if rewards}
			<div class="loot mt-4 p-3 text-left" data-fx-source>
				<div class="flex items-center justify-between gap-2">
					<p class="eyebrow">Phần thưởng</p>
					<a href="/play/tui-do" class="bag" data-bar="inventory" aria-label="Mở Túi Đồ"><Icon name="bag" size={18} /></a>
				</div>
				<ul class="mt-2 space-y-1.5">
					{#each rewards as line (line.label)}
						<li class="flex items-center gap-2" class:artifact={line.artifact}>
							<span class="gem" style:--r="var(--color-rarity-{line.rarity ?? 'linh'})" aria-hidden="true"></span>
							<span class:text-gold-300={line.artifact} class:font-semibold={line.artifact}>{line.label}</span>
							{#if line.artifact}<span class="ml-auto text-xs text-gold-300">Pháp bảo mới</span>{/if}
						</li>
					{:else}
						<li class="text-sm text-mist-500">Không có phần thưởng lượt này.</li>
					{/each}
				</ul>
			</div>
		{/if}
		{#if children}<div class="mt-3 space-y-1 text-sm text-mist-300">{@render children()}</div>{/if}
		{#if actions}<div class="mt-4">{@render actions()}</div>{/if}
		<div class="mt-4 grid grid-cols-2 gap-2">
			<button type="button" class="btn btn-ghost h-11" onclick={onreplay} data-mode-replay><Icon name="lotus" size={16} /> {replayLabel}</button>
			<button type="button" class="btn btn-ghost h-11" onclick={onback} data-mode-back><Icon name="map" size={16} /> Về</button>
		</div>
	</div>
</div>

<style>
	.result {
		position: absolute;
		inset: 0;
		display: grid;
		place-items: center;
		overflow-y: auto;
		padding: 1rem;
	}
	.card {
		width: min(100%, 28rem);
		padding: 1.25rem;
		text-align: center;
		animation: result-in var(--dur-slow) var(--ease-qi);
	}
	@keyframes result-in {
		from {
			opacity: 0;
			transform: translateY(10px) scale(0.98);
		}
	}
	.badge {
		display: inline-flex;
		align-items: center;
		gap: 0.35rem;
		padding: 3px 12px;
		border: 1px solid currentColor;
		border-radius: 9999px;
		font-size: 0.85rem;
		font-weight: 600;
		color: var(--color-mist-300);
	}
	.badge[data-tone='pass'] {
		color: var(--color-gold-300);
	}
	.badge[data-tone='fail'],
	.badge[data-tone='void'] {
		color: var(--color-cinnabar-300);
	}
	.badge[data-tone='practice'] {
		color: var(--color-jade-300);
	}
	:global(:root[data-fx='full']) .badge {
		animation: badge-pop var(--dur-slow) var(--ease-out-back) 120ms both;
	}
	.stats > div {
		padding: 0.45rem 0.25rem;
		border: 1px solid color-mix(in oklab, var(--color-gold-500) 15%, transparent);
		background: color-mix(in oklab, var(--color-ink-950) 45%, transparent);
	}
	.stats dt {
		font-size: 0.7rem;
		color: var(--color-mist-300);
	}
	.stats dd {
		font-size: 1.15rem;
		font-weight: 700;
		color: var(--color-mist-100);
	}
	.loot {
		border: 1px solid color-mix(in oklab, var(--color-gold-500) 25%, transparent);
		background: color-mix(in oklab, var(--color-ink-950) 50%, transparent);
	}
	.bag {
		display: grid;
		place-items: center;
		width: 2.5rem;
		height: 2.5rem;
		border-radius: 9999px;
		color: var(--color-gold-300);
		border: 1px solid color-mix(in oklab, var(--color-gold-500) 40%, transparent);
	}
	.gem {
		display: inline-block;
		width: 10px;
		height: 10px;
		flex-shrink: 0;
		transform: rotate(45deg);
		background: var(--r);
	}
	.artifact {
		margin: 0 -6px;
		padding: 4px 6px;
		background: color-mix(in oklab, var(--color-gold-500) 10%, transparent);
	}
</style>
