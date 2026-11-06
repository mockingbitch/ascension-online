import { redirect } from '@sveltejs/kit';

// Old URL of the cultivation screen (bookmarks): the game is now the one-page shell at /play.
export const load = () => redirect(308, '/play');
