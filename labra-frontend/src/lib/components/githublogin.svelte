<script>
	import githubLogo from '$lib/assets/github-logo.svg';
	import { backendBaseURL } from '$lib/api';

	const loginPath = `${backendBaseURL}/v1/login`;
</script>

<a href={loginPath} id="logintxt" aria-label="Login with GitHub">
	<img src={githubLogo} alt="GitHub logo" height="22" width="22" />
	<span>Login with GitHub</span>
</a>

<style>
	#logintxt {
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

	#logintxt:hover {
		transform: translateY(-1px);
		border-color: rgba(139, 213, 202, 0.54);
	}

	img {
		border-radius: 999px;
	}
</style>
