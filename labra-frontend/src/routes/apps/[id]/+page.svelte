<script lang="ts">
	import type { App, Deployment } from '$lib/api';
	import { apiGET, apiPATCH, apiPOST, prettyDate, shortSHA } from '$lib/api';
	import { onMount } from 'svelte';

	type HistoryResponse = {
		app_id: number;
		app_name: string;
		repo: string;
		branch: string;
		deployments: Deployment[];
	};
	type ConfigHistoryResponse = {
		app_id: number;
		config_versions: {
			id: number;
			source: string;
			config_json: string;
			created_at: number;
		}[];
	};
	type InfraOutputResponse = {
		app_id: number;
		outputs: {
			bucket_name: string;
			distribution_id: string;
			site_url: string;
			updated_at: number;
		};
	};

	export let data: { appID: string };

	let userID = '1';
	let loading = false;
	let error = '';
	let actionError = '';
	let actionMessage = '';
	let actionBusy = false;
	let app: App | null = null;
	let history: HistoryResponse | null = null;
	let configHistory: ConfigHistoryResponse | null = null;
	let infraOutputs: InfraOutputResponse | null = null;

	$: latest = history?.deployments?.[0] ?? null;

	async function loadPage() {
		loading = true;
		error = '';
		try {
			app = await apiGET<App>(`/v1/apps/${data.appID}`, userID);
			history = await apiGET<HistoryResponse>(`/v1/apps/${data.appID}/deploys`, userID);
			configHistory = await apiGET<ConfigHistoryResponse>(`/v1/apps/${data.appID}/config-history`, userID);
			infraOutputs = await apiGET<InfraOutputResponse>(`/v1/apps/${data.appID}/infra-outputs`, userID);
		} catch (err) {
			error = err instanceof Error ? err.message : 'failed to load app details';
			app = null;
			history = null;
			configHistory = null;
			infraOutputs = null;
		} finally {
			loading = false;
		}
	}

	async function deployNow() {
		if (!app) return;
		actionBusy = true;
		actionError = '';
		actionMessage = '';
		try {
			const res = await apiPOST<{ deployment: { id: number } }>(`/v1/apps/${app.id}/deploy`, {}, undefined, userID);
			actionMessage = `Deployment #${res.deployment?.id ?? '?'} queued`;
			await loadPage();
		} catch (err) {
			actionError = err instanceof Error ? err.message : 'failed to trigger deployment';
		} finally {
			actionBusy = false;
		}
	}

	async function toggleAutoDeploy() {
		if (!app) return;
		actionBusy = true;
		actionError = '';
		actionMessage = '';
		try {
			const updated = await apiPATCH<App>(`/v1/apps/${app.id}`, { auto_deploy_enabled: !app.auto_deploy_enabled }, undefined, userID);
			app = updated;
			actionMessage = `Auto-deploy ${updated.auto_deploy_enabled ? 'enabled' : 'disabled'}`;
			await loadPage();
		} catch (err) {
			actionError = err instanceof Error ? err.message : 'failed to update app';
		} finally {
			actionBusy = false;
		}
	}

	onMount(loadPage);
</script>

<section class="page">
	<div class="toolbar">
		<div>
			<a href="/apps" class="back">← Back to apps</a>
			<h1>App Deploy History</h1>
		</div>
		<div class="controls">
			<label>
				User ID
				<input bind:value={userID} />
			</label>
			<button on:click={deployNow} disabled={actionBusy || loading}>Deploy Now</button>
			<button on:click={toggleAutoDeploy} disabled={actionBusy || loading}>
				{app?.auto_deploy_enabled ? 'Disable Auto-Deploy' : 'Enable Auto-Deploy'}
			</button>
			<button on:click={loadPage}>Refresh</button>
		</div>
	</div>

	{#if loading}
		<p class="muted">Loading app history...</p>
	{:else if error}
		<p class="error">{error}</p>
	{:else if !app || !history}
		<p class="muted">No app data.</p>
	{:else}
		{#if actionError}
			<p class="error">{actionError}</p>
		{:else if actionMessage}
			<p class="ok">{actionMessage}</p>
		{/if}

		<div class="summary-grid">
			<article>
				<h2>{app.name}</h2>
				<p><strong>Repo:</strong> {app.repo_full_name}</p>
				<p><strong>Branch:</strong> {app.branch}</p>
				<p><strong>Auto-Deploy:</strong> {app.auto_deploy_enabled ? 'enabled' : 'disabled'}</p>
				<p><strong>Current Site URL:</strong> {latest?.site_url || app.site_url || 'n/a'}</p>
			</article>
			<article>
				<h2>Latest Deploy</h2>
				<p><strong>Status:</strong> {latest?.status || 'n/a'}</p>
				<p><strong>Trigger:</strong> {latest?.trigger_type || 'n/a'}</p>
				<p><strong>Commit:</strong> {shortSHA(latest?.commit_sha)} {latest?.commit_author ? `by ${latest.commit_author}` : ''}</p>
				<p><strong>Updated:</strong> {prettyDate(latest?.updated_at)}</p>
			</article>
			<article>
				<h2>Infra Outputs</h2>
				<p><strong>Bucket:</strong> {infraOutputs?.outputs.bucket_name || 'n/a'}</p>
				<p><strong>Distribution:</strong> {infraOutputs?.outputs.distribution_id || 'n/a'}</p>
				<p><strong>Site:</strong> {infraOutputs?.outputs.site_url || 'n/a'}</p>
				<p><strong>Updated:</strong> {prettyDate(infraOutputs?.outputs.updated_at)}</p>
			</article>
		</div>

		<h2>Config History</h2>
		{#if !configHistory || configHistory.config_versions.length === 0}
			<p class="muted">No config history recorded yet.</p>
		{:else}
			<ul class="config-history">
				{#each configHistory.config_versions as cfg}
					<li>
						<strong>{cfg.source}</strong> at {prettyDate(cfg.created_at)}
					</li>
				{/each}
			</ul>
		{/if}

		{#if history.deployments.length === 0}
			<p class="muted">No deployments yet.</p>
		{:else}
			<table>
				<thead>
					<tr>
						<th>ID</th>
						<th>Status</th>
						<th>Trigger</th>
						<th>Commit</th>
						<th>Updated</th>
						<th>Details</th>
					</tr>
				</thead>
				<tbody>
					{#each history.deployments as dep}
						<tr>
							<td>{dep.id}</td>
							<td>{dep.status}</td>
							<td>{dep.trigger_type}</td>
							<td title={dep.commit_message || ''}>{shortSHA(dep.commit_sha)}</td>
							<td>{prettyDate(dep.updated_at)}</td>
							<td><a href={`/deploys/${dep.id}`}>open</a></td>
						</tr>
					{/each}
				</tbody>
			</table>
		{/if}
	{/if}
</section>
