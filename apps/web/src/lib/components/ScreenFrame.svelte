<!--
	Frame for a game screen inside the shell: ornamental title bar + scrolling body. `status="preview"` shows the
	sample-data ribbon so nobody mistakes mock numbers for server state. `bleed` lets a scene fill the frame.
-->
<script lang="ts">
	import type { Snippet } from 'svelte';
	import type { ScreenStatus } from '$lib/game/nav';
	import Icon, { type IconName } from './Icon.svelte';

	interface Props {
		title: string;
		subtitle?: string;
		icon?: IconName;
		status?: ScreenStatus;
		/** no inner padding / no scroll: the child manages its own layout (scenes) */
		bleed?: boolean;
		actions?: Snippet;
		children: Snippet;
	}
	let { title, subtitle, icon, status = 'live', bleed = false, actions, children }: Props = $props();
</script>

<section class="frame panel flex h-full min-h-0 flex-col" aria-label={title}>
	<header class="titlebar flex shrink-0 items-center gap-3 px-4 py-2.5 sm:px-5">
		{#if icon}<Icon name={icon} size={22} class="shrink-0 text-gold-500" />{/if}
		<div class="min-w-0 flex-1">
			<h1 class="truncate font-display text-2xl leading-tight font-bold text-gold-300">{title}</h1>
			{#if subtitle}<p class="truncate text-xs text-mist-500">{subtitle}</p>{/if}
		</div>
		{#if status === 'preview'}
			<span class="ribbon" title="Hệ thống này chưa nối máy chủ: số liệu là dữ liệu mẫu, không lưu và không ảnh hưởng nhân vật.">
				Bản xem trước
			</span>
		{/if}
		{@render actions?.()}
	</header>
	<div class={['body min-h-0 flex-1', bleed ? 'relative overflow-hidden' : 'overflow-y-auto overscroll-contain px-4 py-4 sm:px-5']}>
		{@render children()}
	</div>
</section>

<style>
	.titlebar {
		border-bottom: 1px solid color-mix(in oklab, var(--color-gold-500) 18%, transparent);
		background: linear-gradient(90deg, color-mix(in oklab, var(--color-gold-500) 10%, transparent), transparent 55%);
	}
	.ribbon {
		flex-shrink: 0;
		padding: 2px 10px;
		font-size: 0.7rem;
		font-weight: 600;
		letter-spacing: 0.08em;
		text-transform: uppercase;
		color: var(--color-ink-950);
		background: linear-gradient(180deg, var(--color-gold-300), var(--color-gold-500));
		clip-path: polygon(8px 0, 100% 0, calc(100% - 8px) 100%, 0 100%);
	}
</style>
