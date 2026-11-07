import { redirect } from '@sveltejs/kit';

// Old URL (bookmarks): the VIP page is now Tiên Duyên.
export const load = () => redirect(308, '/tien-duyen');
