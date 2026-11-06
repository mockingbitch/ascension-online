<script lang="ts">
	import { api, ApiError, newIdempotencyKey } from '$lib/api/client';
	import type { ReportReason } from '$lib/game/types';
	import Icon from './Icon.svelte';

	/** target = null closes the dialog. */
	let { target, onclose }: { target: { characterId: string; name: string; messageId?: number } | null; onclose: () => void } = $props();

	let dialog = $state<HTMLDialogElement>();
	let reason = $state<ReportReason>('offensive_name');
	let note = $state('');
	let busy = $state(false);
	let error = $state<string | null>(null);
	let done = $state(false);
	let key = '';

	const reasons: { value: ReportReason; label: string; hint: string }[] = [
		{ value: 'offensive_name', label: 'Đạo hiệu không phù hợp', hint: 'Tục tĩu, xúc phạm, giả danh ban quản trị hoặc nhân vật.' },
		{ value: 'harassment', label: 'Quấy rối', hint: 'Lăng mạ, đe dọa, làm phiền người khác.' }
	];

	$effect(() => {
		if (!dialog) return;
		if (target && !dialog.open) {
			reason = target.messageId ? 'harassment' : 'offensive_name';
			note = '';
			error = null;
			done = false;
			key = newIdempotencyKey(); // one key per dialog: retries of this report are idempotent
			dialog.showModal();
		} else if (!target && dialog.open) {
			dialog.close();
		}
	});

	async function submit(e: SubmitEvent) {
		e.preventDefault();
		if (!target) return;
		busy = true;
		error = null;
		try {
			await api.report(target.characterId, reason, note.trim(), key, target.messageId ?? 0);
			done = true;
		} catch (err) {
			error = err instanceof ApiError ? err.message : 'Không gửi được báo cáo, vui lòng thử lại.';
		} finally {
			busy = false;
		}
	}
</script>

<dialog bind:this={dialog} onclose={onclose} class="panel m-auto w-[min(28rem,calc(100vw-2rem))] p-0 text-mist-100 backdrop:bg-ink-950/70" aria-labelledby="report-title">
	{#if target}
		<div class="p-6">
			<div class="flex items-start justify-between gap-4">
				<h2 id="report-title" class="font-display text-2xl font-bold text-gold-300">Báo cáo đạo hữu</h2>
				<button type="button" class="-mt-1 -mr-2 inline-flex size-10 items-center justify-center text-mist-300 hover:text-mist-100" onclick={() => dialog?.close()}>
					<Icon name="cross" label="Đóng" />
				</button>
			</div>
			<p class="mt-1 truncate text-sm text-mist-300">Đối tượng: <strong class="text-mist-100">{target.name}</strong></p>
			{#if target.messageId}<p class="mt-1 text-xs text-mist-500">Kèm tin nhắn đã chọn làm bằng chứng.</p>{/if}

			{#if done}
				<p role="status" class="mt-5 flex items-start gap-2 text-sm text-jade-300">
					<Icon name="check" size={18} class="mt-0.5 shrink-0" /> Đã ghi nhận. Ban quản trị sẽ xem xét, người bị báo cáo không biết ai đã gửi.
				</p>
				<button type="button" class="btn btn-ghost mt-5 h-11 w-full" onclick={() => dialog?.close()}>Đóng</button>
			{:else}
				<form onsubmit={submit} class="mt-5 space-y-4">
					<fieldset class="space-y-2">
						<legend class="mb-2 text-sm text-mist-300">Lý do</legend>
						{#each reasons as r (r.value)}
							<label class="flex cursor-pointer items-start gap-3 border border-ink-600 p-3 has-checked:border-gold-500">
								<input type="radio" name="reason" value={r.value} bind:group={reason} class="mt-1 accent-gold-500" />
								<span>
									<span class="block font-medium">{r.label}</span>
									<span class="block text-xs text-mist-500">{r.hint}</span>
								</span>
							</label>
						{/each}
					</fieldset>
					<label class="block">
						<span class="text-sm text-mist-300">Mô tả thêm (không bắt buộc)</span>
						<textarea
							bind:value={note}
							rows="3"
							class="mt-1 w-full resize-none rounded-[2px] border border-ink-600 bg-ink-950/60 p-3 text-mist-100 outline-none focus:border-gold-500"
						></textarea>
					</label>
					{#if error}<p role="alert" class="text-sm text-cinnabar-300">{error}</p>{/if}
					<button class="btn btn-gold h-11 w-full" disabled={busy}>{busy ? 'Đang gửi…' : 'Gửi báo cáo'}</button>
				</form>
			{/if}
		</div>
	{/if}
</dialog>
