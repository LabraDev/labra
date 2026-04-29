export async function probeSiteReachability(url: string): Promise<boolean> {
	if (!url) return false;

	const controller = new AbortController();
	const timeout = window.setTimeout(() => controller.abort(), 5500);
	try {
		await fetch(url, {
			method: 'GET',
			mode: 'no-cors',
			cache: 'no-store',
			signal: controller.signal
		});
		return true;
	} catch {
		return false;
	} finally {
		window.clearTimeout(timeout);
	}
}
