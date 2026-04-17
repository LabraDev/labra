<script lang="ts">
	import type { App } from '$lib/api';
	import { apiGET, prettyDate } from '$lib/api';
	import { onMount } from 'svelte';

	let userID = '1';
	let loading = false;
	let error = '';
	let apps: App[] = [];

	async function loadApps() {
		loading = true;
		error = '';
		try {
			const data = await apiGET<{ apps: App[] }>('/v1/apps', userID);
			apps = data.apps ?? [];
		} catch (err) {
			error = err instanceof Error ? err.message : 'failed to load apps';
			apps = [];
		} finally {
			loading = false;
		}
	}

	onMount(loadApps);
</script>

<section class="page">
	<div class="toolbar">
		<div>
			<h1>Apps</h1>
			<p class="muted">Select an app to view config history, infra outputs, and deploy timeline.</p>
		</div>
		<div class="controls">
			<label>
				User ID
				<input bind:value={userID} />
			</label>
			<button on:click={loadApps}>Refresh</button>
		</div>
	</div>

	{#if loading}
		<p class="muted">Loading apps...</p>
	{:else if error}
		<p class="error">{error}</p>
	{:else if apps.length === 0}
		<p class="muted">No apps yet.</p>
	{:else}
		<div class="cards">
			{#each apps as app}
				<a class="card" href={`/apps/${app.id}`}>
					<h2>{app.name}</h2>
					<p><strong>Repo:</strong> {app.repo_full_name}</p>
					<p><strong>Branch:</strong> {app.branch}</p>
					<p><strong>Build:</strong> {app.build_type}</p>
					<p><strong>Updated:</strong> {prettyDate(app.updated_at)}</p>
				</a>
			{/each}
		</div>
	{/if}
</section>
