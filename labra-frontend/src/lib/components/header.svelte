<script>
	import { goto } from '$app/navigation';
	import { onMount } from 'svelte';
	import { clearSessionToken, getSessionToken, logout } from '$lib/api';

	let isAuthenticated = false;
	let logoutBusy = false;
	let logoGradientDelay = '0s';

	function syncAuthState() {
		isAuthenticated = getSessionToken().trim().length > 0;
	}

	async function handleLogout() {
		if (logoutBusy) return;
		logoutBusy = true;
		try {
			await logout();
		} catch {
			// Always clear local state; server session may already be expired/revoked.
			clearSessionToken();
		} finally {
			syncAuthState();
			logoutBusy = false;
			void goto('/');
		}
	}

	onMount(() => {
		syncAuthState();
		const gradientCycleMs = 18000;
		logoGradientDelay = `${-((Date.now() % gradientCycleMs) / 1000)}s`;
		window.addEventListener('storage', syncAuthState);
		return () => window.removeEventListener('storage', syncAuthState);
	});
</script>

<header class="shell-header">
	<div class="brand-block">
		<a id="logo" href="/" style={`--logo-gradient-delay: ${logoGradientDelay};`}>LABRA</a>
	</div>

	{#if isAuthenticated}
		<nav aria-label="Primary">
			<a href="/dashboard">Dashboard</a>
			<a href="/apps">Apps</a>
			<a href="/settings">AWS Access</a>
			<button type="button" class="logout" on:click={handleLogout} disabled={logoutBusy}>
				{logoutBusy ? 'Signing out...' : 'Logout'}
			</button>
		</nav>
	{/if}
</header>

<style>
	.shell-header {
		position: sticky;
		top: 0;
		z-index: 30;
		display: grid;
		grid-template-columns: auto 1fr;
		align-items: center;
		gap: 1rem;
		padding: 0.82rem 1rem;
		border-bottom: 1px solid rgba(183, 189, 248, 0.18);
		background: linear-gradient(170deg, rgba(24, 25, 38, 0.95), rgba(30, 32, 48, 0.84));
		backdrop-filter: blur(8px);
	}

	.brand-block {
		display: inline-flex;
		align-items: center;
	}

	#logo {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		padding: 0.36rem 0.78rem;
		font-size: clamp(1.62rem, 2.95vw, 2.2rem);
		font-weight: 800;
		letter-spacing: 0.1em;
		line-height: 1;
		text-decoration: none;
		color: transparent;
		background: linear-gradient(135deg, var(--blue) 8%, var(--lavender) 48%, var(--sky) 88%);
		background-size: 230% 230%;
		background-position: 0% 50%;
		-webkit-background-clip: text;
		background-clip: text;
		text-shadow: 0 0 14px rgba(138, 173, 244, 0.22);
		animation: logo-gradient-flow 18s linear infinite;
		animation-delay: var(--logo-gradient-delay, 0s);
		transition:
			transform 120ms ease,
			filter 120ms ease;
	}

	#logo:hover {
		transform: translateY(-1px);
		filter: saturate(1.12);
	}

	nav {
		display: flex;
		flex-wrap: wrap;
		justify-content: flex-end;
		gap: 0.5rem;
	}

	nav a,
	nav .logout {
		text-decoration: none;
		padding: 0.45rem 0.68rem;
		border: 1px solid rgba(138, 173, 244, 0.24);
		border-radius: 999px;
		font-size: 0.88rem;
		font-weight: 600;
		color: var(--lavender);
		background: rgba(24, 25, 38, 0.58);
		transition:
			transform 120ms ease,
			border-color 120ms ease,
			background-color 120ms ease;
	}

	nav a:hover,
	nav .logout:hover:enabled {
		transform: translateY(-1px);
		border-color: rgba(139, 213, 202, 0.55);
		background: rgba(36, 39, 58, 0.95);
	}

	nav .logout {
		cursor: pointer;
		font-family: inherit;
		line-height: 1.2;
		box-shadow: none;
		appearance: none;
	}

	nav .logout:disabled {
		opacity: 0.72;
		cursor: wait;
	}

	@media (max-width: 940px) {
		.shell-header {
			grid-template-columns: 1fr;
			gap: 0.7rem;
			padding-bottom: 0.9rem;
		}

		nav {
			justify-content: center;
		}
	}

	@keyframes logo-gradient-flow {
		0% {
			background-position: 0% 50%;
		}
		50% {
			background-position: 100% 50%;
		}
		100% {
			background-position: 0% 50%;
		}
	}
</style>
