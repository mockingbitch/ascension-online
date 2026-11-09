<!--
	Vân Khê's dialogue box (visual novel): name plate, the line, and the controls — back, next / "Đã hiểu", "Bỏ qua",
	optional "Không nhắc lại". Keyboard: → / Enter next, ← back (Esc is handled by GuideOverlay for the whole overlay).
	A modal dialog (GuideOverlay traps focus around it and the spotlight target); the line is announced politely.
	Her portrait is drawn by GuideOverlay, beside / behind this box.
-->
<script lang="ts">
	import type { TutorialStep } from '$lib/game/types';
	import Icon from './Icon.svelte';

	interface Props {
		name: string;
		step: TutorialStep;
		i: number;
		onnext: () => void;
		onprev: () => void;
		onskip: () => void;
		/** "Không nhắc lại" (omitted for a "?" summon) */
		onmute?: () => void;
		/** move focus to the next button when shown */
		focus?: boolean;
		/** phone sizes: smaller type */
		compact?: boolean;
		busy?: boolean;
	}
	let { name, step, i, onnext, onprev, onskip, onmute, focus = true, compact = false, busy = false }: Props = $props();
	const uid = $props.id();
	const line = $derived(step.lines[i] ?? step.lines[0]!);
	const last = $derived(i >= step.lines.length - 1);
	let nextBtn = $state<HTMLButtonElement>();
	// a summon closes the help window first; the dialog hands focus back to its opener when it closes, so focus again
	// once it has gone (only if focus did not land in the panel already)
	$effect(() => {
		if (!focus) return;
		void step.id;
		const btn = nextBtn;
		btn?.focus({ preventScroll: true });
		const t = setTimeout(() => {
			if (btn && !btn.closest('[role="dialog"]')?.contains(document.activeElement)) btn.focus({ preventScroll: true });
		}, 350);
		return () => clearTimeout(t);
	});

	function onkeydown(e: KeyboardEvent) {
		if (e.key === 'ArrowRight' || (e.key === 'Enter' && (e.target as HTMLElement).tagName !== 'BUTTON')) {
			e.preventDefault();
			onnext();
		} else if (e.key === 'ArrowLeft') {
			e.preventDefault();
			onprev();
		}
	}
</script>

<div
	class="guide-panel"
	class:compact
	role="dialog"
	aria-modal="true"
	aria-labelledby="{uid}-name"
	aria-describedby="{uid}-text"
	tabindex="-1"
	{onkeydown}
	data-guide-step={step.id}
>
	<p class="flex items-baseline justify-between gap-3">
		<span id="{uid}-name" class="name font-display font-bold text-gold-300">{name}</span>
		<span class="meta truncate text-mist-500">{step.title}{step.lines.length > 1 ? ` · ${i + 1}/${step.lines.length}` : ''}</span>
	</p>
	{#key i}
		<p id="{uid}-text" class="line text-mist-100" aria-live="polite">{line.text}</p>
	{/key}
	<div class="ctls">
		{#if i > 0}
			<button type="button" class="ctl press" onclick={onprev} aria-label="Câu trước"><Icon name="arrow" size={18} class="rotate-180" /></button>
		{/if}
		<button bind:this={nextBtn} type="button" class="ctl ctl-main press" onclick={onnext} data-guide-next>
			{last ? 'Đã hiểu' : 'Tiếp'}
			{#if !last}<Icon name="arrow" size={16} />{/if}
		</button>
		<button type="button" class="ctl press" onclick={onskip}>Bỏ qua</button>
		{#if onmute}
			<button type="button" class="ctl ctl-quiet press ml-auto" onclick={onmute} disabled={busy} aria-busy={busy || undefined}>Không nhắc lại</button>
		{/if}
	</div>
</div>

<style>
	.guide-panel {
		position: relative;
		max-height: 100%;
		overflow: auto;
		padding: 1.25rem 1.5rem 1.15rem 1.75rem;
		outline: none;
		border: 1px solid color-mix(in oklab, var(--color-gold-500) 60%, transparent);
		border-left: 5px solid color-mix(in oklab, var(--color-spirit-violet) 70%, var(--color-gold-300));
		background: color-mix(in oklab, var(--color-ink-950) 93%, transparent);
		box-shadow:
			0 24px 60px -18px var(--color-ink-950),
			0 0 40px -10px color-mix(in oklab, var(--color-spirit-violet) 50%, transparent);
	}
	.guide-panel:focus-within {
		border-color: var(--color-gold-300);
	}
	.name {
		font-size: 2rem;
		line-height: 1.1;
		white-space: nowrap;
		text-shadow: 0 0 16px color-mix(in oklab, var(--color-gold-300) 40%, transparent);
	}
	.meta {
		font-size: 0.95rem;
	}
	.line {
		margin-top: 0.6rem;
		font-size: 1.35rem;
		line-height: 1.55;
	}
	.ctls {
		margin-top: 1rem;
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: 0.6rem;
	}
	.compact {
		padding: 0.85rem 1rem 0.85rem 1.15rem;
	}
	.compact .name {
		font-size: 1.5rem;
	}
	.compact .meta {
		font-size: 0.8rem;
	}
	.compact .line {
		margin-top: 0.4rem;
		font-size: 1.12rem;
		line-height: 1.5;
	}
	.compact .ctls {
		margin-top: 0.7rem;
		gap: 0.45rem;
	}
	.ctl {
		display: inline-flex;
		align-items: center;
		gap: 0.4rem;
		min-height: var(--tap, 44px);
		min-width: var(--tap, 44px);
		justify-content: center;
		padding: 0 1rem;
		font-size: 1rem;
		color: var(--color-mist-100);
		border: 1px solid color-mix(in oklab, var(--color-gold-500) 30%, transparent);
		background: color-mix(in oklab, var(--color-ink-900) 80%, transparent);
	}
	.compact .ctl {
		padding: 0 0.8rem;
		font-size: 0.9rem;
	}
	.ctl-main {
		color: var(--color-ink-950);
		font-weight: 600;
		border-color: var(--color-jade-300);
		background: linear-gradient(180deg, var(--color-jade-300), var(--color-jade-500));
	}
	.ctl-quiet {
		color: var(--color-mist-300);
		border-color: transparent;
		background: transparent;
	}
	:global(:root[data-fx='full']) .line,
	:global(:root[data-fx='lite']) .line {
		animation: line-in var(--dur-base) var(--ease-qi);
	}
	@keyframes line-in {
		from {
			opacity: 0.35;
			transform: translateY(3px);
		}
	}
</style>
