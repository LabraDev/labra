export type AppDeleteSessionPhase = 'request' | 'verifying' | 'complete';

export type AppDeleteSession = {
	appID: number;
	appName: string;
	startedAt: number;
	updatedAt: number;
	progressUpdatedAt?: number;
	phase: AppDeleteSessionPhase;
	progress?: number;
	phaseSince?: number;
	completedAt?: number;
};

const activeAppDeleteSessionStorageKey = 'labra_active_app_delete_session_v1';

function canUseStorage(): boolean {
	return typeof window !== 'undefined' && typeof window.localStorage !== 'undefined';
}

function isValidSessionShape(value: unknown): value is AppDeleteSession {
	if (!value || typeof value !== 'object') return false;
	const candidate = value as Partial<AppDeleteSession>;
	if (!Number.isInteger(candidate.appID) || (candidate.appID ?? 0) <= 0) return false;
	if (typeof candidate.appName !== 'string') return false;
	if (!Number.isFinite(candidate.startedAt)) return false;
	if (!Number.isFinite(candidate.updatedAt)) return false;
	if (candidate.progressUpdatedAt !== undefined && !Number.isFinite(candidate.progressUpdatedAt))
		return false;
	if (candidate.phase !== 'request' && candidate.phase !== 'verifying' && candidate.phase !== 'complete')
		return false;
	if (candidate.progress !== undefined && !Number.isFinite(candidate.progress)) return false;
	if (candidate.phaseSince !== undefined && !Number.isFinite(candidate.phaseSince)) return false;
	if (candidate.completedAt !== undefined && !Number.isFinite(candidate.completedAt)) return false;
	return true;
}

export function readActiveAppDeleteSession(): AppDeleteSession | null {
	if (!canUseStorage()) return null;
	try {
		const raw = window.localStorage.getItem(activeAppDeleteSessionStorageKey);
		if (!raw) return null;
		const parsed: unknown = JSON.parse(raw);
		if (!isValidSessionShape(parsed)) return null;
		return parsed;
	} catch {
		return null;
	}
}

export function writeActiveAppDeleteSession(session: AppDeleteSession): void {
	if (!canUseStorage()) return;
	try {
		window.localStorage.setItem(activeAppDeleteSessionStorageKey, JSON.stringify(session));
	} catch {
		// Best-effort persistence only.
	}
}

export function clearActiveAppDeleteSession(): void {
	if (!canUseStorage()) return;
	try {
		window.localStorage.removeItem(activeAppDeleteSessionStorageKey);
	} catch {
		// Ignore storage cleanup issues.
	}
}
