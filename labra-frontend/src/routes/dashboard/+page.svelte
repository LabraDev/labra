<script lang="ts">
	import { onMount } from 'svelte';
	import AppListCard from '$lib/components/app-list-card.svelte';
	import { apiGET, type App, type ProfileResponse, type ServiceStatus } from '$lib/api';

	const HEALTH_POLL_INTERVAL_MS = 15_000;

	let loading = true;
	let error = '';
	let profile: ProfileResponse | null = null;
	let services: ServiceStatus[] = [];
	let apps: App[] = [];
	let appsError = '';
	let servicesRefreshing = false;
	let healthRefreshError = '';
	let pollTimer: ReturnType<typeof setInterval> | null = null;
	let healthyServiceCount = 0;
	let unhealthyServiceCount = 0;
	let healthPercent = 0;
	let healthToneClass = 'health-neutral';
	let healthIsFullyHealthy = false;
	let serviceHealthSummary = 'No service data yet.';
	let healthHint = 'No health checks reported yet.';
	let recentApps: App[] = [];

	const statusTone = (status: string) => {
		// keep this mapper tiny so badge colors stay predictable everywhere
		const s = status.toLowerCase();
		if (s === 'healthy' || s === 'up' || s === 'ok') return 'tone-ok';
		if (s === 'degraded' || s === 'warning') return 'tone-warn';
		if (s === 'down' || s === 'error') return 'tone-error';
		return 'tone-neutral';
	};

	$: healthyServiceCount = services.filter((s) => statusTone(s.status) === 'tone-ok').length;
	$: unhealthyServiceCount = Math.max(services.length - healthyServiceCount, 0);
	$: healthPercent =
		services.length === 0 ? 0 : Math.round((healthyServiceCount / services.length) * 100);
	$: healthToneClass =
		services.length === 0
			? 'health-neutral'
			: unhealthyServiceCount === 0
					? 'health-ok'
					: healthyServiceCount === 0
						? 'health-error'
						: 'health-warn';
	$: healthIsFullyHealthy =
		services.length > 0 && unhealthyServiceCount === 0 && healthPercent >= 100;
	$: serviceHealthSummary =
		services.length === 0
			? 'No service data yet.'
			: unhealthyServiceCount === 0
				? ''
				: `${healthyServiceCount} of ${services.length} services are healthy.`;
	$: healthHint =
		services.length === 0
			? 'No health checks reported yet.'
			: unhealthyServiceCount === 0
				? ''
				: unhealthyServiceCount === 1
					? '1 service needs attention.'
					: `${unhealthyServiceCount} services need attention.`;
	$: recentApps = apps.slice(0, 4);

	$: {
		// poll only when something is unhealthy so we keep noise and api load down
		const shouldPoll = !loading && !error && unhealthyServiceCount > 0;
		if (shouldPoll) {
			startHealthPolling();
		} else {
			stopHealthPolling();
		}
	}

	const toErrorMessage = (err: unknown) =>
		err instanceof Error ? err.message : 'Failed to refresh service health';

	async function loadServices(options: { nonBlocking?: boolean } = {}) {
		if (servicesRefreshing) return;
		servicesRefreshing = true;
		if (!options.nonBlocking) {
			healthRefreshError = '';
		}
		try {
			const system = await apiGET<{ services: ServiceStatus[] }>('/v1/system/services');
			services = system.services;
			healthRefreshError = '';
		} catch (err) {
			const message = toErrorMessage(err);
			if (options.nonBlocking && services.length > 0) {
				// keep old health data on screen if refresh fails in background
				healthRefreshError = message;
				return;
			}
			throw err;
		} finally {
			servicesRefreshing = false;
		}
	}

	async function refreshHealth() {
		await loadServices({ nonBlocking: true });
	}

	function startHealthPolling() {
		if (pollTimer !== null) return;
		// slow interval is enough here because this is not live per second telemetry
		pollTimer = setInterval(() => {
			void loadServices({ nonBlocking: true });
		}, HEALTH_POLL_INTERVAL_MS);
	}

	function stopHealthPolling() {
		if (pollTimer === null) return;
		clearInterval(pollTimer);
		pollTimer = null;
	}

	onMount(() => {
		let mounted = true;
		const load = async () => {
			try {
				// load profile first since this page is account scoped
				profile = await apiGET<ProfileResponse>('/v1/profile');
				await loadServices();
				try {
					// best effort apps preview so dashboard still works if this call fails
					const appResponse = await apiGET<{ apps: App[] }>('/v1/apps');
					apps = appResponse.apps ?? [];
					appsError = '';
				} catch (appsErr) {
					apps = [];
					appsError = appsErr instanceof Error ? appsErr.message : 'Failed to load apps';
				}
			} catch (err) {
				if (!mounted) return;
				error = err instanceof Error ? err.message : 'Failed to load dashboard';
			} finally {
				if (mounted) {
					loading = false;
				}
			}
		};
		void load();

		return () => {
			mounted = false;
			stopHealthPolling();
		};
	});
</script>

<section class="page">
	<div class="toolbar">
		<div>
			<h1>Dashboard</h1>
		</div>
	</div>

	{#if loading}
		<p class="muted">Loading dashboard...</p>
	{:else if error}
		<p class="error">{error}</p>
	{:else}
		<div class="summary-grid">
			<div class={`card metric health-card ${healthToneClass}`}>
				<div class="health-head">
					<h2>Service Health</h2>
					<div class="health-actions">
						{#if unhealthyServiceCount > 0}
							<button
								class="health-refresh-btn"
								on:click={refreshHealth}
								disabled={servicesRefreshing}
							>
								{servicesRefreshing ? 'Refreshing...' : 'Refresh'}
							</button>
						{/if}
						<span class={`health-pill ${healthToneClass}`}>
							{#if services.length === 0}
								No data
							{:else}
								{healthPercent}% healthy
							{/if}
						</span>
					</div>
				</div>
				<div class="health-body">
					<div class="health-orb" aria-hidden="true">
						<span>{healthyServiceCount}/{services.length}</span>
					</div>
					<div class="health-meta">
						{#if serviceHealthSummary}
							<p class="health-copy">{serviceHealthSummary}</p>
						{/if}
							<div
								class="health-bar-track"
								role="img"
								aria-label={`Service health ${healthPercent}%`}
							>
								<div
									class={`health-bar ${healthToneClass} ${healthIsFullyHealthy ? 'health-complete' : ''}`}
									style={`width: ${healthPercent}%;`}
								></div>
							</div>
						{#if healthHint}
							<p class="health-hint">{healthHint}</p>
						{/if}
						{#if healthRefreshError}
							<p class="health-refresh-error">{healthRefreshError}</p>
						{/if}
					</div>
				</div>
			</div>

			<div class="card metric account-card">
				<h2>Account</h2>
				<div class="account-grid">
					<p><strong>User ID:</strong> {profile?.user.id}</p>
					<p><strong>Email:</strong> {profile?.user.email ?? 'Not provided'}</p>
					<p><strong>AWS Connections:</strong> {profile?.aws_connection_count ?? 0}</p>
				</div>
			</div>
		</div>

		<div class="card">
			<h2>Control-Plane Services</h2>
			<ul class="service-list">
				{#each services as service}
					<li>
						<div>
							<strong>{service.name}</strong>
							<span class="muted"> · {service.tier}</span>
						</div>
						<span class={`status ${statusTone(service.status)}`}>{service.status}</span>
					</li>
				{/each}
			</ul>
		</div>

		<div class="card">
			<h2>Recent Apps</h2>
			{#if appsError}
				<p class="muted">{appsError}</p>
			{:else if recentApps.length === 0}
				<p class="muted">No apps yet. Create one from the Apps page.</p>
			{:else}
				<div class="cards">
					{#each recentApps as app}
						<AppListCard app={app} showBuild={true} />
					{/each}
				</div>
			{/if}
		</div>
	{/if}
</section>

<style>
	.metric p {
		margin: 0.34rem 0 0;
	}

	.health-card {
		display: grid;
		align-content: start;
		gap: 0.82rem;
		border-color: rgba(183, 189, 248, 0.28);
		animation: health-card-enter 320ms ease;
	}

	.health-head {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 0.7rem;
		flex-wrap: wrap;
	}

	.health-actions {
		display: inline-flex;
		align-items: center;
		gap: 0.5rem;
	}

	.health-refresh-btn {
		border: 1px solid rgba(145, 215, 227, 0.34);
		background: rgba(145, 215, 227, 0.12);
		color: var(--text-color);
		font-size: 0.74rem;
		font-weight: 600;
		line-height: 1;
		padding: 0.34rem 0.58rem;
		border-radius: 0.55rem;
		cursor: pointer;
		transition:
			border-color 140ms ease,
			background-color 140ms ease,
			transform 140ms ease;
	}

	.health-refresh-btn:hover:not(:disabled) {
		border-color: rgba(145, 215, 227, 0.55);
		background: rgba(145, 215, 227, 0.2);
		transform: translateY(-1px);
	}

	.health-refresh-btn:disabled {
		opacity: 0.66;
		cursor: not-allowed;
		transform: none;
	}

	.health-pill {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		padding: 0.2rem 0.56rem;
		border-radius: 999px;
		font-size: 0.78rem;
		font-weight: 700;
		letter-spacing: 0.06em;
		text-transform: uppercase;
		border: 1px solid transparent;
	}

	.health-body {
		display: grid;
		grid-template-columns: auto 1fr;
		align-items: center;
		gap: 0.82rem;
	}

	.health-orb {
		width: 4.85rem;
		height: 4.85rem;
		border-radius: 999px;
		display: grid;
		place-items: center;
		background: rgba(183, 189, 248, 0.11);
		border: 1px solid rgba(183, 189, 248, 0.3);
		box-shadow: inset 0 0 0 1px rgba(183, 189, 248, 0.08);
		font-size: 1.06rem;
		font-weight: 700;
		letter-spacing: 0.02em;
	}

	.health-meta {
		display: grid;
		gap: 0.4rem;
	}

	.health-copy {
		margin: 0;
		opacity: 0.86;
	}

	.health-hint {
		margin: 0;
		font-size: 0.84rem;
		opacity: 0.76;
	}

	.health-refresh-error {
		margin: 0.08rem 0 0;
		font-size: 0.79rem;
		color: var(--red);
	}

	.health-bar-track {
		width: 100%;
		height: 0.85rem;
		border-radius: 999px;
		background: rgba(183, 189, 248, 0.15);
		overflow: hidden;
	}

	.health-bar {
		height: 100%;
		border-radius: inherit;
		position: relative;
		overflow: hidden;
		animation: health-bar-fill 620ms ease;
	}

	.health-bar.health-complete {
		animation: health-bar-fill 620ms ease, health-complete-pulse 2.8s ease-in-out infinite;
	}

	.health-bar.health-complete::after {
		content: '';
		position: absolute;
		inset: 0;
		background: linear-gradient(
			115deg,
			rgba(255, 255, 255, 0) 20%,
			rgba(255, 255, 255, 0.48) 50%,
			rgba(255, 255, 255, 0) 80%
		);
		transform: translateX(-140%);
		animation: health-complete-shine 2.5s ease-in-out infinite;
		pointer-events: none;
	}

	.health-bar.health-ok {
		background: linear-gradient(120deg, rgba(166, 218, 149, 0.86), rgba(139, 213, 202, 0.88));
	}

	.health-bar.health-warn {
		background: linear-gradient(120deg, rgba(238, 212, 159, 0.86), rgba(245, 169, 127, 0.86));
	}

	.health-bar.health-error {
		background: linear-gradient(120deg, rgba(237, 135, 150, 0.86), rgba(245, 189, 230, 0.82));
	}

	.health-bar.health-neutral {
		background: linear-gradient(120deg, rgba(145, 215, 227, 0.68), rgba(183, 189, 248, 0.7));
	}

	.health-pill.health-ok {
		color: var(--green);
		border-color: rgba(166, 218, 149, 0.42);
		background: rgba(166, 218, 149, 0.14);
	}

	.health-pill.health-warn {
		color: var(--yellow);
		border-color: rgba(238, 212, 159, 0.4);
		background: rgba(238, 212, 159, 0.14);
	}

	.health-pill.health-error {
		color: var(--red);
		border-color: rgba(237, 135, 150, 0.42);
		background: rgba(237, 135, 150, 0.14);
	}

	.health-pill.health-neutral {
		color: var(--sky);
		border-color: rgba(145, 215, 227, 0.38);
		background: rgba(145, 215, 227, 0.14);
	}

	.health-card.health-ok .health-orb {
		border-color: rgba(166, 218, 149, 0.5);
		color: var(--green);
		background: rgba(166, 218, 149, 0.15);
		animation: health-pulse-ok 2.5s ease-in-out infinite;
	}

	.health-card.health-warn .health-orb {
		border-color: rgba(238, 212, 159, 0.5);
		color: var(--yellow);
		background: rgba(238, 212, 159, 0.15);
	}

	.health-card.health-error .health-orb {
		border-color: rgba(237, 135, 150, 0.52);
		color: var(--red);
		background: rgba(237, 135, 150, 0.15);
	}

	.health-card.health-neutral .health-orb {
		border-color: rgba(145, 215, 227, 0.44);
		color: var(--sky);
		background: rgba(145, 215, 227, 0.14);
	}

	.account-card {
		display: grid;
		align-content: start;
		gap: 0.68rem;
	}

	.account-grid {
		display: grid;
		gap: 0.56rem;
	}

	.account-grid p {
		margin: 0;
		line-height: 1.34;
	}

	@keyframes health-bar-fill {
		from {
			width: 0;
		}
	}

	@keyframes health-pulse-ok {
		0%,
		100% {
			box-shadow:
				inset 0 0 0 1px rgba(166, 218, 149, 0.12),
				0 0 0 0 rgba(166, 218, 149, 0.04);
		}
		50% {
			box-shadow:
				inset 0 0 0 1px rgba(166, 218, 149, 0.2),
				0 0 0 8px rgba(166, 218, 149, 0.08);
		}
	}

	@keyframes health-complete-pulse {
		0%,
		100% {
			filter: drop-shadow(0 0 0 rgba(166, 218, 149, 0));
		}
		50% {
			filter: drop-shadow(0 0 9px rgba(166, 218, 149, 0.28));
		}
	}

	@keyframes health-complete-shine {
		0% {
			transform: translateX(-140%);
			opacity: 0;
		}
		18% {
			opacity: 1;
		}
		58% {
			transform: translateX(140%);
			opacity: 0;
		}
		100% {
			transform: translateX(140%);
			opacity: 0;
		}
	}

	@keyframes health-card-enter {
		from {
			opacity: 0;
			transform: translateY(5px);
		}
		to {
			opacity: 1;
			transform: translateY(0);
		}
	}

	.service-list {
		list-style: none;
		display: grid;
		gap: 0.62rem;
		padding: 0;
		margin: 0.45rem 0 0;
	}

	.service-list li {
		display: flex;
		justify-content: space-between;
		align-items: center;
		gap: 0.8rem;
		padding: 0.65rem 0.75rem;
		border-radius: var(--radius-sm);
		border: 1px solid rgba(183, 189, 248, 0.2);
		background: rgba(24, 25, 38, 0.52);
	}

	.cards {
		margin-top: 0.72rem;
	}

	.status {
		font-size: 0.79rem;
		font-weight: 700;
		text-transform: uppercase;
		letter-spacing: 0.08em;
		padding: 0.22rem 0.55rem;
		border-radius: 999px;
		border: 1px solid transparent;
	}

	.tone-ok {
		color: var(--green);
		border-color: rgba(166, 218, 149, 0.36);
		background: rgba(166, 218, 149, 0.14);
	}

	.tone-warn {
		color: var(--yellow);
		border-color: rgba(238, 212, 159, 0.36);
		background: rgba(238, 212, 159, 0.14);
	}

	.tone-neutral {
		color: var(--sky);
		border-color: rgba(145, 215, 227, 0.34);
		background: rgba(145, 215, 227, 0.14);
	}

	.tone-error {
		color: var(--red);
		border-color: rgba(237, 135, 150, 0.34);
		background: rgba(237, 135, 150, 0.14);
	}

	@media (max-width: 700px) {
		.health-body {
			grid-template-columns: 1fr;
		}

		.health-orb {
			width: 4.25rem;
			height: 4.25rem;
		}
	}

	@media (prefers-reduced-motion: reduce) {
		.health-bar,
		.health-bar.health-complete,
		.health-bar.health-complete::after {
			animation: none !important;
		}
	}
</style>
