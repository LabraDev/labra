<script>
	import GithubLoginButton from '$lib/components/githublogin.svelte';

	const appEnv = import.meta.env.PUBLIC_APP_ENV ?? 'local';
</script>

<header class="shell-header">
	<div class="brand-block">
		<a id="logo" href="/">LABRA</a>
		<p class="tagline">Cloud Deploy Control Plane</p>
	</div>

	<nav aria-label="Primary">
		<a href="/dashboard">Dashboard</a>
		<a href="/apps">Apps</a>
		<a href="/deploys">Deploys</a>
		<a href="/settings">Settings</a>
		<a href="/login">Login</a>
	</nav>

	<div class="meta">
		<span class="env">Env: {appEnv}</span>
		<div class="login-wrap">
			<GithubLoginButton />
		</div>
	</div>
</header>

<style>
	.shell-header {
		position: sticky;
		top: 0;
		z-index: 30;
		display: grid;
		grid-template-columns: auto 1fr auto;
		align-items: center;
		gap: 1rem;
		padding: 0.85rem 1rem;
		border-bottom: 1px solid rgba(183, 189, 248, 0.18);
		background: linear-gradient(170deg, rgba(24, 25, 38, 0.95), rgba(30, 32, 48, 0.84));
		backdrop-filter: blur(8px);
	}

	.brand-block {
		display: grid;
		gap: 0.05rem;
	}

	#logo {
		font-size: clamp(1.2rem, 2.1vw, 1.55rem);
		font-weight: 700;
		letter-spacing: 0.08em;
		text-decoration: none;
		color: var(--rosewater);
	}

	.tagline {
		font-size: 0.76rem;
		opacity: 0.72;
	}

	nav {
		display: flex;
		flex-wrap: wrap;
		justify-content: center;
		gap: 0.5rem;
	}

	nav a {
		text-decoration: none;
		padding: 0.45rem 0.68rem;
		border: 1px solid rgba(138, 173, 244, 0.24);
		border-radius: 999px;
		font-size: 0.88rem;
		font-weight: 600;
		color: var(--lavender);
		background: rgba(24, 25, 38, 0.58);
		transition: transform 120ms ease, border-color 120ms ease, background-color 120ms ease;
	}

	nav a:hover {
		transform: translateY(-1px);
		border-color: rgba(139, 213, 202, 0.55);
		background: rgba(36, 39, 58, 0.95);
	}

	.meta {
		display: flex;
		align-items: center;
		gap: 0.55rem;
	}

	.env {
		font-size: 0.82rem;
		font-weight: 600;
		padding: 0.34rem 0.56rem;
		border: 1px solid rgba(244, 219, 214, 0.3);
		border-radius: 999px;
		color: var(--rosewater);
		background: rgba(24, 25, 38, 0.74);
	}

	.login-wrap {
		display: flex;
		align-items: center;
	}

	@media (max-width: 940px) {
		.shell-header {
			grid-template-columns: 1fr;
			gap: 0.7rem;
			padding-bottom: 0.9rem;
		}

		nav,
		.meta {
			justify-content: center;
		}
	}
</style>
