<script lang="ts">
	import { page } from '$app/state';
	import { googleLoginUrl } from '$lib/api/session.svelte';

	const errors: Record<string, string> = {
		session_expired: 'Phiên đăng nhập đã hết hạn, vui lòng thử lại.',
		invalid_state: 'Yêu cầu đăng nhập không hợp lệ, vui lòng thử lại.',
		access_denied: 'Bạn đã hủy đăng nhập Google.',
		google_failed: 'Không xác thực được với Google, vui lòng thử lại.',
		email_unverified: 'Email Google của bạn chưa được xác minh.',
		email_taken: 'Email này đã gắn với một tài khoản khác.',
		server_error: 'Máy chủ gặp sự cố, vui lòng thử lại sau.'
	};
	const error = $derived(errors[page.url.searchParams.get('error') ?? ''] ?? null);
	const returnTo = $derived(page.url.searchParams.get('return') ?? '/cultivate');
</script>

<svelte:head><title>Đăng nhập · Tiên Đạo</title></svelte:head>

<section class="panel mx-auto mt-10 max-w-sm p-8 text-center">
	<p class="eyebrow">Thiên Nguyên Giới</p>
	<h1 class="mt-2 text-4xl font-bold text-gold-300">Nhập Đạo</h1>
	<p class="mt-2 text-sm text-mist-300">Đăng nhập để bắt đầu con đường tu tiên.</p>

	{#if error}
		<p role="alert" class="mt-4 border-l-2 border-cinnabar-500 bg-ink-950/50 p-3 text-left text-sm text-cinnabar-300">{error}</p>
	{/if}

	<a
		href={googleLoginUrl(returnTo)}
		class="mt-6 inline-flex h-11 w-full items-center justify-center gap-3 rounded-[2px] bg-mist-100 font-semibold text-ink-900 transition hover:brightness-110"
	>
		<svg aria-hidden="true" viewBox="0 0 24 24" class="size-5">
			<path fill="#4285F4" d="M22.6 12.3c0-.8-.1-1.6-.2-2.3H12v4.4h5.9a5 5 0 0 1-2.2 3.3v2.7h3.6c2.1-1.9 3.3-4.8 3.3-8.1z" />
			<path fill="#34A853" d="M12 23c3 0 5.5-1 7.3-2.7l-3.6-2.7c-1 .7-2.2 1.1-3.7 1.1-2.9 0-5.3-1.9-6.2-4.5H2.1v2.8A11 11 0 0 0 12 23z" />
			<path fill="#FBBC05" d="M5.8 14.2a6.6 6.6 0 0 1 0-4.3V7.1H2.1a11 11 0 0 0 0 9.9l3.7-2.8z" />
			<path fill="#EA4335" d="M12 5.4c1.6 0 3.1.6 4.2 1.7l3.2-3.2A11 11 0 0 0 2.1 7.1l3.7 2.8C6.7 7.3 9.1 5.4 12 5.4z" />
		</svg>
		Đăng nhập bằng Google
	</a>
</section>
