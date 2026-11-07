<!-- DEV ONLY: simulated gateway checkout. Each button records the charge at the "provider" and (unless "mất webhook")
     sends a correctly signed webhook through the real verification path. Nothing here is real money. -->
<script lang="ts">
	import { api, ApiError } from '$lib/api/client';
	import Icon from '$lib/components/Icon.svelte';
	import type { PaymentOrder, SandboxCompleteResult } from '$lib/game/types';

	let { data } = $props();
	let order = $state<PaymentOrder | null>(null);
	$effect(() => {
		order = data.order;
	});
	let result = $state<SandboxCompleteResult | null>(null);
	let busy = $state(false);
	let error = $state<string | null>(null);
	const vnd = (n: number) => `${n.toLocaleString('vi-VN')}đ`;

	async function act(outcome: 'paid' | 'failed', deliver = true) {
		busy = true;
		error = null;
		try {
			result = await api.sandboxComplete(data.orderId, outcome, deliver);
			order = result.order;
		} catch (e) {
			error =
				e instanceof ApiError && e.status === 404 && e.code !== 'order_not_found'
					? 'Cổng thử nghiệm chỉ có ở môi trường dev.'
					: e instanceof ApiError
						? e.message
						: 'Lỗi không xác định';
		} finally {
			busy = false;
		}
	}
</script>

<svelte:head><title>Cổng thử nghiệm · Tiên Đạo</title></svelte:head>

<section class="panel mx-auto mt-6 max-w-md p-6">
	<p class="eyebrow flex items-center gap-2"><Icon name="bolt" size={14} /> Cổng thử nghiệm · không có tiền thật</p>
	<h1 class="mt-2 font-display text-3xl font-bold text-gold-300">Thanh toán giả lập</h1>

	{#if !data.signedIn}
		<p class="mt-4 text-mist-300">Hãy <a class="text-gold-300 underline" href="/login">đăng nhập</a> bằng tài khoản đã tạo đơn.</p>
	{:else if !order}
		<p class="mt-4 text-mist-300">Không tìm thấy đơn này trong tài khoản của bạn.</p>
	{:else}
		<dl class="mt-4 grid grid-cols-[auto_1fr] gap-x-4 gap-y-1 text-sm">
			<dt class="text-mist-500">Gói</dt>
			<dd class="text-mist-100">{order.packageName}</dd>
			<dt class="text-mist-500">Số tiền</dt>
			<dd class="text-mist-100 tabular-nums">{vnd(order.amountVnd)}</dd>
			<dt class="text-mist-500">Mã đơn</dt>
			<dd class="font-mono text-xs break-all text-mist-300">{order.id}</dd>
			<dt class="text-mist-500">Trạng thái</dt>
			<dd class="text-mist-100">{order.status}</dd>
		</dl>
		<div class="mt-5 grid gap-2">
			<button class="btn btn-jade h-11" disabled={busy} onclick={() => act('paid')}>Thanh toán thành công</button>
			<button class="btn btn-ghost h-11" disabled={busy} onclick={() => act('failed')}>Thanh toán thất bại</button>
			<button class="btn btn-ghost h-11 text-sm" disabled={busy} onclick={() => act('paid', false)}>Thành công nhưng mất webhook (chờ đối soát)</button>
		</div>
		{#if result}
			<p class="mt-4 text-sm text-mist-300" aria-live="polite">
				{#if result.webhook}
					Webhook đã ký gửi tới máy chủ: <strong class="text-mist-100">{result.webhook.outcome}</strong>{result.webhook.detail ? ` (${result.webhook.detail})` : ''}.
				{:else}
					Cổng đã ghi nhận nhưng không gửi webhook — tác vụ đối soát của worker sẽ hỏi lại cổng.
				{/if}
			</p>
		{/if}
		<a class="btn btn-gold mt-5 h-11 w-full" href="/tien-duyen?order={order.id}">Quay lại Tiên Duyên</a>
	{/if}
	{#if error}<p class="mt-3 text-sm text-cinnabar-300" role="alert">{error}</p>{/if}
</section>
