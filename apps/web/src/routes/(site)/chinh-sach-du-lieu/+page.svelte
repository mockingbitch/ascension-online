<script lang="ts">
	// Keep in sync with what the server actually stores (migrations 000001/000003/000006/000017, auth service, netsig, nginx logs).
	// Any new personal field → update this page in the same PR.
	const updated = '07/11/2026';
</script>

<svelte:head><title>Chính sách dữ liệu · Tiên Đạo</title></svelte:head>

<article class="panel mx-auto max-w-3xl p-6 sm:p-8">
	<p class="eyebrow">Tiên Đạo Online</p>
	<h1 class="mt-2 text-4xl font-bold text-gold-300">Chính sách dữ liệu</h1>
	<p class="mt-2 text-sm text-mist-500">Cập nhật: {updated}</p>

	<div class="policy mt-6">
		<p>
			Tiên Đạo chỉ thu những dữ liệu cần để bạn đăng nhập, chơi và để giữ cộng đồng an toàn. Chúng tôi không bán dữ liệu,
			không dùng cho quảng cáo, không đọc danh bạ, vị trí hay tệp trên thiết bị của bạn.
		</p>

		<h2>1. Dữ liệu nhận từ Google khi đăng nhập</h2>
		<p>Tiên Đạo chỉ cho đăng nhập bằng Google và không lưu mật khẩu. Khi bạn đăng nhập, Google gửi cho chúng tôi:</p>
		<ul>
			<li><strong>Địa chỉ email</strong> và trạng thái đã xác minh email: để nhận diện tài khoản.</li>
			<li><strong>Mã định danh tài khoản Google</strong>: để bạn vẫn vào đúng tài khoản nếu đổi email.</li>
			<li><strong>Tên hiển thị và đường dẫn ảnh đại diện Google</strong>: chỉ hiện cho chính bạn trên thanh trên cùng, không hiện cho người chơi khác.</li>
		</ul>

		<h2>2. Dữ liệu trong game</h2>
		<ul>
			<li><strong>Đạo hiệu</strong> bạn tự đặt: hiển thị công khai trên Thiên Bảng.</li>
			<li>Tiến độ tu luyện, cảnh giới, lịch sử đột phá, vật phẩm, linh thạch, điểm Tiên Duyên và nhật ký thay đổi của chúng (để chống gian lận và xử lý khiếu nại).</li>
			<li>
				<strong>Tin nhắn kênh Thế Giới</strong>: công khai với mọi người chơi, kèm đạo hiệu và cảnh giới lúc gửi. Từ tục bị che bằng ***;
				bản gốc trước khi che được giữ riêng cho ban quản trị để xử lý vi phạm. Tin nhắn tự xóa sau 90 ngày.
			</li>
			<li>Báo cáo bạn gửi và báo cáo về nhân vật của bạn (kèm nội dung tin nhắn bị báo cáo, nếu có). Người bị báo cáo không được biết ai đã báo cáo.</li>
			<li>
				<strong>Đạo lữ:</strong> lời cầu thân bạn gửi và nhận, mối đạo lữ (ngày kết, ngày và lý do chấm dứt) và thời gian song tu đã nhận. Đạo lữ của
				bạn thấy đạo hiệu, cảnh giới và việc bạn đang trực tuyến hay không.
			</li>
		</ul>

		<h2>3. Dữ liệu kỹ thuật</h2>
		<ul>
			<li>
				<strong>Phiên đăng nhập:</strong> một cookie bảo mật (HttpOnly) giữ phiên tối đa 30 ngày. Máy chủ chỉ lưu bản băm của
				mã phiên, chuỗi nhận diện trình duyệt (user agent) và thời điểm đăng nhập, dùng để duy trì đăng nhập và phát hiện phiên bị đánh cắp.
			</li>
			<li>
				<strong>Dấu vết mạng (chống một người dùng nhiều tài khoản):</strong> mỗi lần bạn đăng nhập hoặc làm mới phiên, máy chủ lưu một mã băm
				được tạo từ địa chỉ IP của bạn cùng một khóa bí mật của máy chủ — <strong>không lưu địa chỉ IP gốc</strong>, và không thể dịch ngược
				mã này thành IP nếu không có khóa bí mật của máy chủ. Với IPv4, mã được tạo từ cả địa chỉ; với IPv6, chỉ từ phần mạng (64 bit đầu,
				thường là cả mạng gia đình bạn). Mã tự xóa sau 30 ngày không dùng tới. Nó chỉ dùng để báo cho ban quản trị khi hai tài khoản có vẻ
				dùng chung một mạng trong các hoạt động giữa người chơi (song tu đạo lữ, tặng/đổi đồ, hộ pháp); các dấu cảnh báo này cũng tự xóa sau
				30 ngày. Hệ thống không tự khóa ai vì mã này, người chơi khác không bao giờ thấy nó, và mỗi lần ban quản trị xem đều được ghi nhật ký.
			</li>
			<li>
				<strong>Địa chỉ IP:</strong> dùng tạm để giới hạn số lần đăng nhập và kết nối (tự xóa sau vài phút). Máy chủ web có thể ghi
				nhật ký truy cập kỹ thuật có IP để vận hành và chống tấn công.
			</li>
			<li>Không có công cụ quảng cáo hay theo dõi của bên thứ ba trên trang.</li>
		</ul>

		<h2>4. Ai xem được dữ liệu</h2>
		<ul>
			<li>Người chơi khác chỉ thấy đạo hiệu, cảnh giới, thứ hạng và tin nhắn bạn gửi ở kênh chung.</li>
			<li>Ban quản trị xem được dữ liệu tài khoản khi hỗ trợ hoặc xử lý báo cáo. Mọi thao tác quản trị đều được ghi nhật ký.</li>
			<li>Chúng tôi chỉ cung cấp dữ liệu cho cơ quan nhà nước khi pháp luật yêu cầu.</li>
		</ul>

		<h2>5. Lưu trữ và quyền của bạn</h2>
		<p>
			Dữ liệu được giữ trong thời gian tài khoản còn hoạt động. Bạn có quyền yêu cầu xem, sửa hoặc xóa tài khoản và dữ liệu của mình
			qua kênh hỗ trợ chính thức của game. Khi xóa tài khoản, nhân vật, tiến độ và vật phẩm sẽ bị xóa vĩnh viễn; một số nhật ký giao
			dịch và quản trị có thể được giữ lại trong thời hạn pháp luật yêu cầu.
		</p>
	</div>

	<p class="mt-8 text-sm"><a href="/login" class="text-gold-300 underline underline-offset-4">Quay lại đăng nhập</a></p>
</article>

<style>
	.policy {
		color: var(--color-mist-300);
		line-height: 1.7;
	}
	.policy :global(h2) {
		margin-top: 1.75rem;
		margin-bottom: 0.5rem;
		font-family: var(--font-display);
		font-size: 1.5rem;
		font-weight: 700;
		color: var(--color-mist-100);
	}
	.policy :global(p + p) {
		margin-top: 0.75rem;
	}
	.policy :global(ul) {
		margin-top: 0.5rem;
		padding-left: 1.25rem;
		list-style: disc;
	}
	.policy :global(li + li) {
		margin-top: 0.4rem;
	}
	.policy :global(strong) {
		color: var(--color-mist-100);
		font-weight: 600;
	}
</style>
