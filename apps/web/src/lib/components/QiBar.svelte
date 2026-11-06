<script lang="ts">
	import { formatPoints, progressRatio } from '$lib/game/format';
	import type { Aura } from '$lib/game/types';
	import Icon from './Icon.svelte';

	interface Props {
		/** Stored cultivation in the current stage, milli. */
		current: number;
		/** Stage requirement, milli. */
		required: number;
		/** Bình cảnh cap (milli); > 0 only on a realm's last stage. */
		cap?: number;
		aura?: Aura;
		label?: string;
	}
	let { current, required, cap = 0, aura = 'jade', label = 'Tu vi' }: Props = $props();
	const ratio = $derived(progressRatio(current, required));
	// Minor stages advance automatically; only the realm's last stage holds points at the bottleneck.
	const full = $derived(cap > 0 && current >= required);
</script>

<div class="w-full" style:--aura="var(--color-aura-{aura})">
	<div class="mb-2 flex items-end justify-between gap-3">
		<span class="eyebrow">{label}</span>
		<span class="tabular-nums">
			<span class="text-xl font-semibold text-mist-100">{formatPoints(current)}</span>
			<span class="text-sm text-mist-500"> / {formatPoints(required)}</span>
		</span>
	</div>
	<div
		class="track relative h-4"
		class:is-full={full}
		role="progressbar"
		aria-label={label}
		aria-valuemin={0}
		aria-valuemax={required}
		aria-valuenow={Math.min(current, required)}
		aria-valuetext="{formatPoints(current)} / {formatPoints(required)}"
	>
		<div class="fill absolute inset-y-0 left-0" style:width="{ratio * 100}%">
			<span class="sheen" aria-hidden="true"></span>
		</div>
		<span class="ticks" aria-hidden="true"></span>
	</div>
	{#if full}
		<p class="mt-2 flex items-center gap-1.5 text-sm font-medium text-gold-300">
			<Icon name="spark" size={16} /> Bình cảnh — có thể đột phá
			{#if current > required}<span class="text-mist-500">(dư {formatPoints(current - required)} / tối đa {formatPoints(cap - required)})</span>{/if}
		</p>
	{/if}
</div>

<style>
	.track {
		overflow: hidden;
		background: linear-gradient(180deg, var(--color-ink-950), var(--color-ink-900));
		border: 1px solid color-mix(in oklab, var(--color-gold-500) 35%, transparent);
		box-shadow: inset 0 2px 6px var(--color-ink-950);
	}
	.fill {
		transition: width 700ms var(--ease-qi);
		background:
			linear-gradient(180deg, color-mix(in oklab, white 22%, transparent), transparent 55%),
			linear-gradient(90deg, color-mix(in oklab, var(--aura) 35%, var(--color-ink-800)), var(--aura));
		box-shadow: 0 0 12px color-mix(in oklab, var(--aura) 55%, transparent);
		overflow: hidden;
	}
	.is-full .fill {
		background:
			linear-gradient(180deg, color-mix(in oklab, white 30%, transparent), transparent 55%),
			linear-gradient(90deg, var(--color-gold-700), var(--color-gold-300));
		box-shadow: 0 0 16px color-mix(in oklab, var(--color-gold-300) 55%, transparent);
	}
	.sheen {
		position: absolute;
		inset: 0;
		width: 40%;
		background: linear-gradient(90deg, transparent, color-mix(in oklab, white 35%, transparent), transparent);
		animation: sheen 3.2s var(--ease-qi) infinite;
	}
	@keyframes sheen {
		from {
			transform: translateX(-120%);
		}
		to {
			transform: translateX(300%);
		}
	}
	.ticks {
		position: absolute;
		inset: 0;
		background:
			linear-gradient(var(--color-ink-950), var(--color-ink-950)) 25% 0 / 1px 100%,
			linear-gradient(var(--color-ink-950), var(--color-ink-950)) 50% 0 / 1px 100%,
			linear-gradient(var(--color-ink-950), var(--color-ink-950)) 75% 0 / 1px 100%;
		background-repeat: no-repeat;
		opacity: 0.6;
	}
</style>
