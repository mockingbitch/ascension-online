// Đạo ngữ for the boot screen (original lines, display only). One is picked at random per load.
export const DAO_NGU = [
	'Đạo khả đạo, phi thường đạo — đường đi rồi mới thành đường.',
	'Một hơi linh khí, mười năm tĩnh tọa.',
	'Tâm tĩnh như nước, kiếp đến như mây.',
	'Bình cảnh không phải tường, là cửa chưa mở.',
	'Người tu hành không tranh ngày, chỉ tranh một niệm.',
	'Hương tàn lại thắp, đạo tâm chưa tàn.',
	'Thiên địa bất nhân, chỉ người có tâm mới thấy đường.',
	'Gió qua núi không để lại dấu, đạo qua người để lại tâm.',
	'Chậm một bước, vững mười năm.',
	'Muôn pháp quy tông, khởi từ hơi thở đầu tiên.'
] as const;

export const randomDaoNgu = (r: () => number = Math.random) => DAO_NGU[Math.floor(r() * DAO_NGU.length)] ?? DAO_NGU[0];
