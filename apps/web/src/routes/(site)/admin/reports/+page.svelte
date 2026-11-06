<script lang="ts">
	import { invalidateAll } from '$app/navigation';
	import { api, ApiError } from '$lib/api/client';
	import Icon from '$lib/components/Icon.svelte';
	import type { AdminReport, ReportReason, ReportStatus } from '$lib/game/types';

	let { data } = $props();

	const tabs: { status: ReportStatus; label: string }[] = [
		{ status: 'open', label: 'Chờ xử lý' },
		{ status: 'actioned', label: 'Đã xử lý' },
		{ status: 'dismissed', label: 'Đã bỏ qua' }
	];
	const reasonLabel: Record<ReportReason, string> = { offensive_name: 'Đạo hiệu không phù hợp', harassment: 'Quấy rối' };
	const when = (iso: string) => new Date(iso).toLocaleString('vi-VN', { dateStyle: 'short', timeStyle: 'short' });

	// Per-report form state, keyed by report id.
	let notes = $state<Record<string, string>>({});
	let renames = $state<Record<string, string>>({});
	let errors = $state<Record<string, string>>({});
	let busy = $state<string | null>(null);
	let flash = $state<string | null>(null);

	async function chatAction(r: AdminReport, kind: 'hide' | 'mute') {
		busy = r.id;
		errors[r.id] = '';
		try {
			if (kind === 'hide' && r.chatMessageId) {
				await api.adminHideChat(r.chatMessageId, notes[r.id] ?? '');
				flash = 'Đã ẩn tin nhắn với mọi người.';
			} else if (kind === 'mute') {
				const m = await api.adminMuteChat(r.targetUserId, 24, notes[r.id] ?? '');
				flash = `Đã cấm ${r.targetCurrentName} truyền âm tới ${when(m.mutedUntil)}.`;
			}
		} catch (e) {
			errors[r.id] = e instanceof ApiError ? e.message : 'Lỗi không xác định';
		} finally {
			busy = null;
		}
	}

	async function resolve(r: AdminReport, outcome: 'dismissed' | 'actioned', withRename = false) {
		busy = r.id;
		errors[r.id] = '';
		flash = null;
		try {
			const res = await api.adminResolveReport(r.id, outcome, notes[r.id] ?? '', withRename ? (renames[r.id] ?? '') : '');
			flash = res.renamedTo
				? `Đã đổi "${res.renamedFrom}" thành "${res.renamedTo}"${res.alsoResolved.length ? `, đóng thêm ${res.alsoResolved.length} báo cáo cùng đối tượng` : ''}.`
				: outcome === 'dismissed'
					? 'Đã bỏ qua báo cáo.'
					: 'Đã đánh dấu xử lý.';
			await invalidateAll(); // server list is the truth
		} catch (e) {
			errors[r.id] = e instanceof ApiError ? e.message : 'Lỗi không xác định';
		} finally {
			busy = null;
		}
	}
</script>

<svelte:head><title>Báo cáo · Quản trị · Tiên Đạo</title></svelte:head>

<header class="mx-auto mb-6 max-w-4xl">
	<p class="eyebrow">Quản trị</p>
	<h1 class="mt-2 text-4xl font-bold text-gold-300">Báo cáo người chơi</h1>
	<p class="mt-2 text-sm text-mist-300">Mọi thao tác được ghi vào nhật ký quản trị. Đổi tên áp dụng cùng bộ lọc với lúc tạo nhân vật.</p>
</header>

<nav class="mx-auto mb-4 flex max-w-4xl gap-1 overflow-x-auto" aria-label="Trạng thái báo cáo">
	{#each tabs as t (t.status)}
		<a
			href="?status={t.status}"
			class="btn btn-ghost h-10 shrink-0"
			class:is-current={t.status === data.status}
			aria-current={t.status === data.status ? 'page' : undefined}>{t.label}</a
		>
	{/each}
</nav>

{#if flash}<p role="status" class="mx-auto mb-4 flex max-w-4xl items-center gap-2 text-sm text-jade-300"><Icon name="check" size={16} /> {flash}</p>{/if}

{#if data.page.reports.length === 0}
	<p class="panel mx-auto max-w-4xl px-4 py-8 text-center text-mist-300">Không có báo cáo nào.</p>
{:else}
	<ul class="mx-auto max-w-4xl space-y-3">
		{#each data.page.reports as r (r.id)}
			<li class="panel p-4">
				<div class="flex flex-wrap items-baseline justify-between gap-x-4 gap-y-1">
					<p class="min-w-0 font-display text-xl font-bold text-mist-100">
						{r.targetCurrentName}
						{#if r.targetCurrentName !== r.targetName}<span class="text-sm font-normal text-mist-500">(lúc báo cáo: {r.targetName})</span>{/if}
					</p>
					<span class="text-xs text-mist-500 tabular-nums">{when(r.createdAt)}</span>
				</div>
				<p class="mt-1 text-sm">
					<span class="text-gold-300">{reasonLabel[r.reason]}</span>
					<span class="text-mist-500">· người báo cáo: {r.reporterName || 'chưa có nhân vật'}</span>
					{#if r.status === 'open' && r.openAgainstTarget > 1}
						<span class="ml-1 inline-flex items-center gap-1 text-cinnabar-300"><Icon name="flag" size={14} /> {r.openAgainstTarget} báo cáo đang mở</span>
					{/if}
				</p>
				{#if r.evidence}
					<p class="mt-2 border-l-2 border-cinnabar-500 bg-ink-950/40 py-1 pl-3 text-sm text-mist-100">
						<span class="mr-1 text-xs text-mist-500">Tin nhắn:</span>{r.evidence}
					</p>
				{/if}
				{#if r.note}<p class="mt-2 border-l-2 border-ink-600 pl-3 text-sm whitespace-pre-line text-mist-300">{r.note}</p>{/if}

				{#if r.status === 'open'}
					<div class="mt-4 grid gap-3 sm:grid-cols-2">
						<label class="block">
							<span class="text-xs text-mist-500">Ghi chú xử lý</span>
							<input bind:value={notes[r.id]} class="field" autocomplete="off" />
						</label>
						{#if r.reason === 'offensive_name'}
							<label class="block">
								<span class="text-xs text-mist-500">Đổi đạo hiệu thành</span>
								<input bind:value={renames[r.id]} class="field" autocomplete="off" placeholder="Ví dụ: Vô Danh Khách" />
							</label>
						{/if}
					</div>
					{#if errors[r.id]}<p role="alert" class="mt-2 text-sm text-cinnabar-300">{errors[r.id]}</p>{/if}
					<div class="mt-3 flex flex-wrap gap-2">
						<button class="btn btn-ghost h-10" disabled={busy === r.id} onclick={() => resolve(r, 'dismissed')}>Bỏ qua</button>
						<button class="btn btn-jade h-10" disabled={busy === r.id} onclick={() => resolve(r, 'actioned')}>Đã xử lý</button>
						{#if r.chatMessageId}
							<button class="btn btn-ghost h-10" disabled={busy === r.id} onclick={() => chatAction(r, 'hide')}>Ẩn tin nhắn</button>
						{/if}
						<button class="btn btn-ghost h-10" disabled={busy === r.id} onclick={() => chatAction(r, 'mute')}>Cấm chat 24h</button>
						{#if r.reason === 'offensive_name'}
							<button class="btn btn-gold h-10" disabled={busy === r.id || !renames[r.id]?.trim()} onclick={() => resolve(r, 'actioned', true)}>
								Đổi tên &amp; xử lý
							</button>
						{/if}
					</div>
				{:else}
					<p class="mt-3 text-sm text-mist-500">
						{r.status === 'actioned' ? 'Đã xử lý' : 'Đã bỏ qua'}{r.resolvedAt ? ` lúc ${when(r.resolvedAt)}` : ''}{r.resolutionNote ? `: ${r.resolutionNote}` : ''}
					</p>
				{/if}
			</li>
		{/each}
	</ul>
{/if}

<nav class="mx-auto mt-5 flex max-w-4xl justify-between text-sm">
	{#if data.page.offset > 0}
		<a class="btn btn-ghost" href="?status={data.status}&offset={Math.max(0, data.page.offset - data.pageSize)}">Trước</a>
	{:else}<span></span>{/if}
	{#if data.page.reports.length === data.pageSize}
		<a class="btn btn-ghost" href="?status={data.status}&offset={data.page.offset + data.pageSize}">Sau</a>
	{/if}
</nav>

<style>
	.field {
		margin-top: 0.25rem;
		height: 2.5rem;
		width: 100%;
		border-radius: 2px;
		border: 1px solid var(--color-ink-600);
		background: color-mix(in oklab, var(--color-ink-950) 60%, transparent);
		padding: 0 0.75rem;
		color: var(--color-mist-100);
		outline: none;
	}
	.field:focus {
		border-color: var(--color-gold-500);
	}
	.is-current {
		color: var(--color-gold-300);
		border-color: var(--color-gold-500);
	}
</style>
