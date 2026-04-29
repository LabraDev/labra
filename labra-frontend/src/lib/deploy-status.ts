export type DeployTone = 'ok' | 'warn' | 'error' | 'busy' | 'neutral';

export function sanitizeSiteURL(value?: string): string {
	const trimmed = value?.trim() ?? '';
	if (trimmed.length === 0) return '';
	return trimmed;
}

export function deriveStatusKey(
	status?: string,
	siteURL?: string,
	siteReachable?: boolean
): string {
	const normalized = (status ?? '').trim().toLowerCase();
	const hasSiteURL = sanitizeSiteURL(siteURL).length > 0;
	const reachable = siteReachable ?? hasSiteURL;

	// Treat "awaiting_site_url" as recovered when the URL is present and reachable.
	if (normalized === 'awaiting_site_url' && hasSiteURL && reachable) {
		return 'succeeded';
	}

	if (normalized === 'succeeded') {
		if (!hasSiteURL || !reachable) return 'awaiting_site_url';
	}
	return normalized;
}

export function statusTone(status?: string, siteURL?: string, siteReachable?: boolean): DeployTone {
	const normalized = deriveStatusKey(status, siteURL, siteReachable);
	if (normalized === 'awaiting_site_url') return 'warn';
	if (normalized === 'succeeded') return 'ok';
	if (normalized === 'failed' || normalized === 'canceled') return 'error';
	if (normalized === 'queued' || normalized === 'running') return 'busy';
	if (normalized === 'retrying' || normalized === 'degraded') return 'warn';
	return 'neutral';
}

export function statusLabel(status?: string, siteURL?: string, siteReachable?: boolean): string {
	const normalized = deriveStatusKey(status, siteURL, siteReachable);
	if (normalized.length === 0) return 'No deployments yet';
	if (normalized === 'awaiting_site_url') return 'Awaiting site URL';
	if (normalized === 'manual_retry') return 'Retry queued';
	return normalized.replaceAll('_', ' ');
}

export function statusProgressValue(
	status?: string,
	siteURL?: string,
	siteReachable?: boolean
): number {
	const normalized = deriveStatusKey(status, siteURL, siteReachable);
	if (normalized === 'queued') return 24;
	if (normalized === 'running') return 72;
	if (normalized === 'awaiting_site_url') return 92;
	if (normalized === 'succeeded' || normalized === 'failed' || normalized === 'canceled')
		return 100;
	return 0;
}

export function prettyTrigger(trigger?: string): string {
	const normalized = (trigger ?? '').trim().toLowerCase();
	if (normalized.length === 0) return 'Not available yet';
	if (normalized === 'manual') return 'Manual';
	if (normalized === 'manual_retry') return 'Manual Retry';
	if (normalized === 'webhook') return 'Webhook';
	return normalized.replaceAll('_', ' ');
}

export function present(value?: string): string {
	const trimmed = value?.trim() ?? '';
	return trimmed.length > 0 ? trimmed : 'Not available yet';
}
