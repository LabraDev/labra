<script lang="ts">
	import type { Deployment, DeploymentLog } from '$lib/api';
	import { apiGET, apiPOST, prettyDate } from '$lib/api';
	import AIInsightPanel from '$lib/components/ai-insight-panel.svelte';
	import DeploymentStatusPanel from '$lib/components/deployment-status-panel.svelte';
	import {
		deriveStatusKey,
		present,
		prettyTrigger,
		sanitizeSiteURL,
		statusLabel,
		statusProgressValue,
		statusTone
	} from '$lib/deploy-status';
	import { probeSiteReachability } from '$lib/site-reachability';
	import { onMount } from 'svelte';

	export let data: { deployID: string };

	let loading = false;
	let error = '';
	let actionError = '';
	let actionMessage = '';
	let actionBusy = false;
	let deploy: Deployment | null = null;
	let logs: DeploymentLog[] = [];
	$: deploySiteURL = sanitizeSiteURL(deploy?.site_url);
	let deploySiteReachable = false;
	let deployReachabilityTarget = '';
	$: deployStatusTone = statusTone(deploy?.status, deploySiteURL, deploySiteReachable);
	$: deployStatusLabel = statusLabel(deploy?.status, deploySiteURL, deploySiteReachable);
	$: deployStatusProgress = statusProgressValue(deploy?.status, deploySiteURL, deploySiteReachable);
	$: deployStatusComplete = deployStatusTone === 'ok' && deployStatusProgress >= 100;
	$: deployStatusAwaitingURL =
		deriveStatusKey(deploy?.status, deploySiteURL, deploySiteReachable) === 'awaiting_site_url';

	$: {
		if (!loading && deploySiteURL !== deployReachabilityTarget) {
			void checkDeploySiteReachability(deploySiteURL);
		}
	}

	async function loadPage() {
		loading = true;
		error = '';
		deployReachabilityTarget = '';
		deploySiteReachable = false;
		try {
			deploy = await apiGET<Deployment>(`/v1/deploys/${data.deployID}`);
			const logRes = await apiGET<{ logs: DeploymentLog[] }>(`/v1/deploys/${data.deployID}/logs`);
			logs = logRes.logs ?? [];
		} catch (err) {
			error = err instanceof Error ? err.message : 'failed to load deploy details';
			deploy = null;
			logs = [];
		} finally {
			loading = false;
		}
	}

	onMount(loadPage);

	async function cancelDeploy() {
		if (!deploy) return;
		actionBusy = true;
		actionError = '';
		actionMessage = '';
		try {
			await apiPOST<{ deployment: Deployment }>(`/v1/deploys/${deploy.id}/cancel`, {});
			actionMessage = 'Deployment canceled';
			await loadPage();
		} catch (err) {
			actionError = err instanceof Error ? err.message : 'failed to cancel deployment';
		} finally {
			actionBusy = false;
		}
	}

	async function retryDeploy() {
		if (!deploy) return;
		actionBusy = true;
		actionError = '';
		actionMessage = '';
		try {
			const res = await apiPOST<{ deployment: { id: number } }>(
				`/v1/deploys/${deploy.id}/retry`,
				{}
			);
			const nextID = res?.deployment?.id;
			if (nextID) {
				window.location.href = `/deploys/${nextID}`;
				return;
			}
			actionMessage = 'Retry requested';
			await loadPage();
		} catch (err) {
			actionError = err instanceof Error ? err.message : 'failed to retry deployment';
		} finally {
			actionBusy = false;
		}
	}

	async function checkDeploySiteReachability(url: string) {
		deployReachabilityTarget = url;
		if (!url) {
			deploySiteReachable = false;
			return;
		}
		deploySiteReachable = await probeSiteReachability(url);
	}
</script>

<section class="page">
	<div class="toolbar">
		<div>
			<h1>Deployment #{data.deployID}</h1>
			{#if deploy}
				<a class="back" href={`/apps/${deploy.app_id}`}>← Back to app history</a>
			{/if}
		</div>
		<div class="controls">
			<button
				on:click={cancelDeploy}
				disabled={actionBusy ||
					loading ||
					!deploy ||
					(deploy.status !== 'queued' && deploy.status !== 'running')}>Cancel</button
			>
			<button
				on:click={retryDeploy}
				disabled={actionBusy ||
					loading ||
					!deploy ||
					(deploy.status !== 'failed' && deploy.status !== 'canceled')}>Retry</button
			>
			<button on:click={loadPage}>Refresh</button>
		</div>
	</div>

	{#if loading}
		<p class="muted">Loading deployment...</p>
	{:else if error}
		<p class="error">{error}</p>
	{:else if !deploy}
		<p class="muted">No deployment found.</p>
	{:else}
		{#if actionError}
			<p class="error">{actionError}</p>
		{:else if actionMessage}
			<p class="ok">{actionMessage}</p>
		{/if}

		<div class="summary-grid">
			<article>
				<h2>Status</h2>
				<DeploymentStatusPanel
					tone={deployStatusTone}
					label={deployStatusLabel}
					progress={deployStatusProgress}
					awaitingURL={deployStatusAwaitingURL}
					pulseLoop={deployStatusComplete}
					ariaPrefix="Deployment progress"
				/>
				<p><strong>Trigger:</strong> {prettyTrigger(deploy.trigger_type)}</p>
				<p><strong>Updated:</strong> {prettyDate(deploy.updated_at)}</p>
				{#if deploy.failure_reason?.trim()}
					<p><strong>Failure Reason:</strong> {deploy.failure_reason}</p>
				{/if}
			</article>
		</div>

		<h2>Logs</h2>
		{#if logs.length === 0}
			<p class="muted">No logs for this deployment yet.</p>
		{:else}
			<ul class="logs">
				{#each logs as log}
					<li>
						<span class="stamp">[{prettyDate(log.created_at)}]</span>
						<span class="level">{log.log_level.toUpperCase()}</span>
						<span>{log.message}</span>
					</li>
				{/each}
			</ul>
		{/if}

		<AIInsightPanel
			deployID={deploy.id}
			disabled={loading || actionBusy}
			title="AI Insight"
			generateLabel="Generate AI Insight"
			historyLabel="Recent AI Requests"
		/>
	{/if}
</section>
