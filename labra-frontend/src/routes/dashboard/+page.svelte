<script lang="ts">
	import { onMount } from 'svelte';
	import { apiGET, type ProfileResponse, type ServiceStatus } from '$lib/api';

	let loading = true;
	let error = '';
	let profile: ProfileResponse | null = null;
	let services: ServiceStatus[] = [];

	const statusTone = (status: string) => {
		const s = status.toLowerCase();
		if (s === 'healthy' || s === 'up' || s === 'ok') return 'tone-ok';
		if (s === 'degraded' || s === 'warning') return 'tone-warn';
		if (s === 'down' || s === 'error') return 'tone-error';
		return 'tone-neutral';
	};

	onMount(async () => {
		try {
			profile = await apiGET<ProfileResponse>('/v1/profile');
			const system = await apiGET<{ services: ServiceStatus[] }>('/v1/system/services');
			services = system.services;
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load dashboard';
		} finally {
			loading = false;
		}
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
			<div class="card metric">
				<h2>Principal</h2>
				<p><strong>User ID:</strong> {profile?.user.id}</p>
				<p><strong>Email:</strong> {profile?.user.email ?? 'n/a'}</p>
			</div>
			<div class="card metric">
				<h2>Access</h2>
				<p><strong>Roles:</strong> {profile?.principal.roles.join(', ') || 'none'}</p>
				<p><strong>AWS Connections:</strong> {profile?.aws_connection_count}</p>
			</div>
			<div class="card metric">
				<h2>Runtime</h2>
				<p><strong>Services:</strong> {services.length}</p>
				<p><strong>Healthy:</strong> {services.filter((s) => statusTone(s.status) === 'tone-ok').length}</p>
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
	{/if}
</section>

<style>
	.metric p {
		margin-top: 0.38rem;
	}

	.service-list {
		list-style: none;
		display: grid;
		gap: 0.55rem;
		padding: 0;
		margin: 0.35rem 0 0;
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
</style>
