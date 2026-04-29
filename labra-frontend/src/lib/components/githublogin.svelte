<script lang="ts">
	import githubLogo from '$lib/assets/github-logo.svg';
	import { backendBaseURL, setPostLoginRedirect } from '$lib/api';

	export let variant = 'compact';
	export let fallbackPath = '/login';
	export let postLoginPath = '';

	const loginPath = `${backendBaseURL}/v1/login`;
	const fallbackMessage = 'Sign-in is unavailable right now. Please try again.';
	let checking = false;

	function buildFallbackURL(message: string): string {
		const safeMessage = message.trim().length > 0 ? message.trim() : fallbackMessage;
		return `${fallbackPath}?notice=${encodeURIComponent(safeMessage)}`;
	}

	async function handleClick(event: MouseEvent): Promise<void> {
		if (checking) {
			event.preventDefault();
			return;
		}

		event.preventDefault();
		checking = true;
		setPostLoginRedirect(postLoginPath);

		try {
			const res = await fetch(loginPath, { method: 'GET', redirect: 'manual' });
			if (res.type === 'opaqueredirect') {
				window.location.href = loginPath;
				return;
			}

			if (res.status >= 300 && res.status < 400) {
				const location = res.headers.get('location');
				if (location && location.trim().length > 0) {
					window.location.href = location;
					return;
				}
			}

			if (res.ok) {
				window.location.href = loginPath;
				return;
			}

			let detail = fallbackMessage;
			try {
				const raw = await res.text();
				if (raw.trim().length > 0) {
					try {
						const parsed = JSON.parse(raw);
						detail = parsed?.error?.message ?? parsed?.message ?? parsed?.detail ?? detail;
					} catch {
						detail = raw.trim();
					}
				}
			} catch {
				// keep fallback detail
			}

			window.location.href = buildFallbackURL(detail);
		} catch {
			window.location.href = buildFallbackURL(fallbackMessage);
		} finally {
			checking = false;
		}
	}
</script>

<a
	href={loginPath}
	class={`github-login ${variant === 'large' ? 'large' : ''} ${checking ? 'busy' : ''}`}
	aria-label="Login with GitHub"
	on:click={handleClick}
>
	<img src={githubLogo} alt="GitHub logo" height="22" width="22" />
	<span>{checking ? 'Opening GitHub...' : 'Login with GitHub'}</span>
</a>

<style>
	.github-login {
		display: inline-flex;
		justify-content: center;
		align-items: center;
		gap: 0.4rem;
		padding: 0.42rem 0.66rem;
		border: 1px solid rgba(183, 189, 248, 0.3);
		border-radius: 999px;
		font-size: 0.84rem;
		font-weight: 600;
		text-decoration: none;
		color: var(--text-color);
		background: rgba(24, 25, 38, 0.82);
		transition: border-color 130ms ease, transform 130ms ease;
	}

	.github-login.busy {
		pointer-events: none;
		opacity: 0.72;
	}

	.github-login:hover {
		transform: translateY(-1px);
		border-color: rgba(139, 213, 202, 0.54);
	}

	.github-login.large {
		padding: 0.62rem 1.1rem;
		font-size: 0.92rem;
	}

	.github-login.large img {
		height: 24px;
		width: 24px;
	}

	img {
		border-radius: 999px;
	}
</style>
