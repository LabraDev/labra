export type App = {
	id: number;
	user_id: number;
	name: string;
	repo_full_name: string;
	branch: string;
	build_type: string;
	output_dir: string;
	root_dir?: string;
	site_url?: string;
	auto_deploy_enabled: boolean;
	created_at: number;
	updated_at: number;
};

export type Deployment = {
	id: number;
	app_id: number;
	user_id: number;
	status: string;
	trigger_type: string;
	commit_sha?: string;
	commit_message?: string;
	commit_author?: string;
	branch?: string;
	site_url?: string;
	failure_reason?: string;
	created_at: number;
	updated_at: number;
	started_at?: number;
	finished_at?: number;
};

export type DeploymentLog = {
	id: number;
	deployment_id: number;
	log_level: string;
	message: string;
	created_at: number;
};

export type AIRequestLog = {
	id: number;
	user_id: number;
	deployment_id: number;
	prompt_version: string;
	provider: string;
	model: string;
	input_redacted: boolean;
	fallback_used: boolean;
	status: string;
	input_excerpt?: string;
	output_excerpt?: string;
	created_at: number;
};

export type AIDeployInsightResponse = {
	deployment_id: number;
	insight: string;
	source: string;
	model: string;
	prompt_version: string;
	fallback_used: boolean;
	confidence: string;
	limitations: string;
	request_log: AIRequestLog;
};

export type ProfileResponse = {
	user: {
		id: number;
		email?: string;
		status: string;
		created_at: number;
		updated_at: number;
	};
	principal: {
		sub: string;
		email?: string;
		roles: string[];
		session_id?: string;
		expires_at?: number;
	};
	aws_connection_count: number;
};

export type ServiceStatus = {
	name: string;
	tier: string;
	mode: string;
	status: string;
	description: string;
};

export type AuthSessionResponse = {
	session: {
		token: string;
		session_id: string;
		expires_at: number;
	};
	user: {
		id: number;
		email?: string;
		status: string;
		is_new: boolean;
	};
	principal: {
		sub: string;
		email?: string;
		roles: string[];
	};
};

export const backendBaseURL = import.meta.env.VITE_BACKEND_BASE_URL ?? '';
const SESSION_TOKEN_KEY = 'labra_session_token';

export function getSessionToken(): string {
	if (typeof window === 'undefined') return '';
	return window.localStorage.getItem(SESSION_TOKEN_KEY) ?? '';
}

export function setSessionToken(token: string): void {
	if (typeof window === 'undefined') return;
	window.localStorage.setItem(SESSION_TOKEN_KEY, token);
}

export function clearSessionToken(): void {
	if (typeof window === 'undefined') return;
	window.localStorage.removeItem(SESSION_TOKEN_KEY);
}

function buildHeaders(extra?: Record<string, string>, userID?: string): HeadersInit {
	const headers: Record<string, string> = {
		'Content-Type': 'application/json',
		...(extra ?? {})
	};
	if (userID && userID.trim().length > 0) {
		headers['X-User-ID'] = userID.trim();
	}
	const token = getSessionToken();
	if (token) {
		headers.Authorization = `Bearer ${token}`;
	}
	return headers;
}

async function parseOrThrow<T>(res: Response): Promise<T> {
	if (!res.ok) {
		let detail = `request failed (${res.status})`;
		try {
			const body = await res.json();
			detail = body?.error?.message ?? detail;
		} catch {
			// keep fallback
		}
		throw new Error(detail);
	}
	return (await res.json()) as T;
}

export async function apiGET<T>(path: string, userID?: string): Promise<T> {
	const res = await fetch(`${backendBaseURL}${path}`, {
		headers: buildHeaders(undefined, userID)
	});
	return parseOrThrow<T>(res);
}

export async function apiPOST<T>(
	path: string,
	body: unknown,
	headers?: Record<string, string>,
	userID?: string
): Promise<T> {
	const res = await fetch(`${backendBaseURL}${path}`, {
		method: 'POST',
		headers: buildHeaders(headers, userID),
		body: JSON.stringify(body)
	});
	return parseOrThrow<T>(res);
}

export async function apiPATCH<T>(
	path: string,
	body: unknown,
	headers?: Record<string, string>,
	userID?: string
): Promise<T> {
	const res = await fetch(`${backendBaseURL}${path}`, {
		method: 'PATCH',
		headers: buildHeaders(headers, userID),
		body: JSON.stringify(body)
	});
	return parseOrThrow<T>(res);
}

export async function createAuthSession(externalJWT: string): Promise<AuthSessionResponse> {
	const res = await fetch(`${backendBaseURL}/v1/auth/session`, {
		method: 'POST',
		headers: {
			'Content-Type': 'application/json',
			Authorization: `Bearer ${externalJWT}`
		}
	});
	const session = await parseOrThrow<AuthSessionResponse>(res);
	setSessionToken(session.session.token);
	return session;
}

export async function logout(): Promise<void> {
	await apiPOST('/v1/auth/logout', {});
	clearSessionToken();
}

export function shortSHA(sha?: string): string {
	if (!sha) return 'n/a';
	return sha.slice(0, 7);
}

export function prettyDate(epochSeconds?: number): string {
	if (!epochSeconds || epochSeconds <= 0) return 'n/a';
	return new Date(epochSeconds * 1000).toLocaleString();
}
