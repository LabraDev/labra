// types that mirror what the backend sends back - keep these in sync with store/models.go

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

// what the backend stores when we call the ai insights endpoint
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
	output_text?: string;
	created_at: number;
};

// what the ai insights endpoint returns
export type AIDeployInsightResponse = {
	deployment_id: number;
	insight: string;
	source: string;
	model: string;
	prompt_version: string;
	fallback_used: boolean;
	confidence: string;
	request_log: AIRequestLog;
};

// profile response from /v1/auth/profile - has both the db user and jwt principal
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

// base url for all api calls - can be overridden via env var for local dev pointing at a remote backend
export const backendBaseURL = import.meta.env.VITE_BACKEND_BASE_URL ?? '';

// localStorage key where we keep the session jwt
const SESSION_TOKEN_KEY = 'labra_session_token';

// localStorage key for where to redirect after login
const POST_LOGIN_REDIRECT_KEY = 'labra_post_login_redirect';

// flag to prevent multiple simultaneous auth redirect attempts
let authRedirectInProgress = false;

// custom error class so callers can check the http status code
export class APIError extends Error {
	status: number;

	constructor(message: string, status: number) {
		super(message);
		this.name = 'APIError';
		this.status = status;
	}
}

// pull the session token from localStorage - returns empty string if not logged in
export function getSessionToken(): string {
	if (typeof window === 'undefined') return '';
	return window.localStorage.getItem(SESSION_TOKEN_KEY) ?? '';
}

// store the session token after login
export function setSessionToken(token: string): void {
	if (typeof window === 'undefined') return;
	window.localStorage.setItem(SESSION_TOKEN_KEY, token);
}

// clear the session token on logout or when the token expires
export function clearSessionToken(): void {
	if (typeof window === 'undefined') return;
	window.localStorage.removeItem(SESSION_TOKEN_KEY);
}

// save where the user wanted to go before we sent them to login
export function setPostLoginRedirect(path: string): void {
	if (typeof window === 'undefined') return;
	const trimmedPath = path.trim();
	// only save internal paths - reject anything that doesn't start with /
	if (!trimmedPath.startsWith('/')) return;
	window.localStorage.setItem(POST_LOGIN_REDIRECT_KEY, trimmedPath);
}

// pop the saved redirect path after login completes - defaults to /dashboard
export function consumePostLoginRedirect(defaultPath = '/dashboard'): string {
	if (typeof window === 'undefined') return defaultPath;
	const savedRedirectPath = window.localStorage.getItem(POST_LOGIN_REDIRECT_KEY) ?? '';
	window.localStorage.removeItem(POST_LOGIN_REDIRECT_KEY);
	const trimmedSavedPath = savedRedirectPath.trim();
	if (trimmedSavedPath.startsWith('/')) {
		return trimmedSavedPath;
	}
	return defaultPath;
}

// build the headers for an api request - adds auth header if we have a token
function buildHeaders(extraHeaders?: Record<string, string>): HeadersInit {
	const requestHeaders: Record<string, string> = {
		'Content-Type': 'application/json',
		...(extraHeaders ?? {})
	};
	const currentToken = getSessionToken();
	if (currentToken) {
		requestHeaders.Authorization = `Bearer ${currentToken}`;
	}
	return requestHeaders;
}

// parses a response and throws a useful APIError if it's not ok
// handles json errors, html error pages, and 401 redirects
async function parseOrThrow<T>(response: Response): Promise<T> {
	if (!response.ok) {
		let errorDetail = `Request failed (${response.status})`;
		if (response.status === 401) {
			errorDetail = 'Your session expired. Redirecting to sign in.';
		} else if (response.status === 403) {
			errorDetail = 'You are logged in, but your account does not have access to this action.';
		}

		if (response.status !== 401) {
			try {
				const rawResponseText = await response.text();
				if (rawResponseText.trim().length > 0) {
					try {
						const parsedErrorBody = JSON.parse(rawResponseText);
						errorDetail = parsedErrorBody?.error?.message ?? parsedErrorBody?.message ?? parsedErrorBody?.detail ?? errorDetail;
					} catch {
						const trimmedResponseText = rawResponseText.trim();
						if (trimmedResponseText.startsWith('<')) {
							// html response means we hit a proxy or cdn error page
							if (response.status >= 500) {
								errorDetail = `Service temporarily unavailable (${response.status}). Please retry in a moment.`;
							} else {
								errorDetail = `Unexpected server response (${response.status}). Please refresh and try again.`;
							}
						} else {
							errorDetail = trimmedResponseText;
						}
					}
				}
			} catch {
				// if we can't read the body at all just keep the fallback message
			}
		}

		if (response.status === 401) {
			// clear the token and bounce to home - session is gone
			clearSessionToken();
			if (typeof window !== 'undefined' && !authRedirectInProgress) {
				authRedirectInProgress = true;
				if (window.location.pathname !== '/') {
					window.location.replace('/');
				}
			}
		}

		throw new APIError(errorDetail, response.status);
	}

	const responseContentType = response.headers.get('content-type')?.toLowerCase() ?? '';
	const rawResponseText = await response.text();
	if (rawResponseText.trim().length === 0) {
		// empty body is fine - return empty object
		return {} as T;
	}

	if (!responseContentType.includes('application/json')) {
		throw new APIError(
			'Server returned an unexpected response format. Refresh and try again.',
			response.status
		);
	}

	try {
		return JSON.parse(rawResponseText) as T;
	} catch {
		throw new APIError('Server returned malformed JSON. Refresh and try again.', response.status);
	}
}

// generic GET request helper - all api reads go through here
export async function apiGET<T>(apiPath: string): Promise<T> {
	const fetchResponse = await fetch(`${backendBaseURL}${apiPath}`, {
		headers: buildHeaders(),
		cache: 'no-store'
	});
	return parseOrThrow<T>(fetchResponse);
}

// generic POST request helper
export async function apiPOST<T>(
	apiPath: string,
	requestBody: unknown,
	extraHeaders?: Record<string, string>
): Promise<T> {
	const fetchResponse = await fetch(`${backendBaseURL}${apiPath}`, {
		method: 'POST',
		headers: buildHeaders(extraHeaders),
		body: JSON.stringify(requestBody)
	});
	return parseOrThrow<T>(fetchResponse);
}

// generic PATCH request helper - used for partial updates
export async function apiPATCH<T>(
	apiPath: string,
	requestBody: unknown,
	extraHeaders?: Record<string, string>
): Promise<T> {
	const fetchResponse = await fetch(`${backendBaseURL}${apiPath}`, {
		method: 'PATCH',
		headers: buildHeaders(extraHeaders),
		body: JSON.stringify(requestBody)
	});
	return parseOrThrow<T>(fetchResponse);
}

// generic DELETE request helper
export async function apiDELETE<T>(apiPath: string, extraHeaders?: Record<string, string>): Promise<T> {
	const fetchResponse = await fetch(`${backendBaseURL}${apiPath}`, {
		method: 'DELETE',
		headers: buildHeaders(extraHeaders)
	});
	return parseOrThrow<T>(fetchResponse);
}

// logs the user out - calls the backend to revoke the session then clears the local token
export async function logout(): Promise<void> {
	await apiPOST('/v1/auth/logout', {});
	clearSessionToken();
}

// formats a unix timestamp (in seconds) as a human readable date string
export function prettyDate(epochSeconds?: number): string {
	if (!epochSeconds || epochSeconds <= 0) return 'n/a';
	return new Date(epochSeconds * 1000).toLocaleString();
}
