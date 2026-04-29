import {
	clearActiveAppDeleteSession,
	readActiveAppDeleteSession,
	writeActiveAppDeleteSession,
	type AppDeleteSession
} from '$lib/app-delete-session';

export function readDeleteFlowSessionForApp(appID: number): AppDeleteSession | null {
	if (typeof window === 'undefined') return null;
	const saved = readActiveAppDeleteSession();
	if (!saved) return null;
	if (saved.appID !== appID) return null;
	return saved;
}

export function persistDeleteFlowSession(next: AppDeleteSession | null): void {
	if (next) {
		writeActiveAppDeleteSession(next);
		return;
	}
	clearActiveAppDeleteSession();
}
