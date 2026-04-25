<script lang="ts">
	import type { AIDeployInsightResponse, AIRequestLog, Deployment, DeploymentLog } from '$lib/api';
	import { apiGET, apiPOST, prettyDate, shortSHA } from '$lib/api';
	import { onMount } from 'svelte';

export let data: { deployID: string };

	let loading = false;
	let error = '';
	let actionError = '';
	let actionMessage = '';
	let actionBusy = false;
	let deploy: Deployment | null = null;
	let logs: DeploymentLog[] = [];
	let aiPrompt = '';
	let aiBusy = false;
	let aiError = '';
	let aiResult: AIDeployInsightResponse | null = null;
	let aiHistory: AIRequestLog[] = [];

	async function loadPage() {
		loading = true;
		error = '';
		try {
			deploy = await apiGET<Deployment>(`/v1/deploys/${data.deployID}`);
			const logRes = await apiGET<{ logs: DeploymentLog[] }>(`/v1/deploys/${data.deployID}/logs`);
			logs = logRes.logs ?? [];
			await loadAIHistory();
		} catch (err) {
			error = err instanceof Error ? err.message : 'failed to load deploy details';
			deploy = null;
			logs = [];
			aiHistory = [];
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
			const res = await apiPOST<{ deployment: { id: number } }>(`/v1/deploys/${deploy.id}/retry`, {});
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

	async function loadAIHistory() {
		if (!deploy) return;
		try {
			const res = await apiGET<{ logs: AIRequestLog[] }>(`/v1/ai/requests?limit=25`);
			aiHistory = (res.logs ?? []).filter((x) => x.deployment_id === deploy?.id).slice(0, 5);
		} catch {
			aiHistory = [];
		}
	}

	async function generateAIInsight(bypassAI = false) {
		if (!deploy) return;
		aiBusy = true;
		aiError = '';
		try {
			aiResult = await apiPOST<AIDeployInsightResponse>(
				'/v1/ai/deploy-insights',
				{
					deployment_id: deploy.id,
					prompt: aiPrompt,
					bypass_ai: bypassAI
				}
			);
			await loadAIHistory();
		} catch (err) {
			aiError = err instanceof Error ? err.message : 'failed to generate AI insight';
		} finally {
			aiBusy = false;
		}
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
			<button on:click={cancelDeploy} disabled={actionBusy || loading || !deploy || (deploy.status !== 'queued' && deploy.status !== 'running')}>Cancel</button>
			<button on:click={retryDeploy} disabled={actionBusy || loading || !deploy || (deploy.status !== 'failed' && deploy.status !== 'canceled')}>Retry</button>
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
				<p><strong>{deploy.status}</strong></p>
				<p><strong>Trigger:</strong> {deploy.trigger_type}</p>
				<p><strong>Updated:</strong> {prettyDate(deploy.updated_at)}</p>
				<p><strong>Site URL:</strong> {deploy.site_url || 'n/a'}</p>
			</article>
			<article>
				<h2>Commit</h2>
				<p><strong>SHA:</strong> {shortSHA(deploy.commit_sha)}</p>
				<p><strong>Author:</strong> {deploy.commit_author || 'n/a'}</p>
				<p><strong>Message:</strong> {deploy.commit_message || 'n/a'}</p>
				<p><strong>Branch:</strong> {deploy.branch || 'n/a'}</p>
				<p><strong>Failure Reason:</strong> {deploy.failure_reason || 'n/a'}</p>
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

		<h2>AI Insight</h2>
		<p class="muted">AI-generated output may be incorrect. Verify suggestions against deployment logs before acting.</p>
		<div class="ai-controls">
			<textarea bind:value={aiPrompt} rows="3" placeholder="Ask AI for a focused deployment analysis (optional)"></textarea>
			<div class="ai-actions">
				<button on:click={() => generateAIInsight(false)} disabled={aiBusy || loading || !deploy}>
					{aiBusy ? 'Generating...' : 'Generate AI Insight'}
				</button>
				<button class="secondary" on:click={() => generateAIInsight(true)} disabled={aiBusy || loading || !deploy}>
					Bypass AI (Fallback)
				</button>
			</div>
		</div>

		{#if aiError}
			<p class="error">{aiError}</p>
		{/if}

		{#if aiResult}
			<article class="ai-result">
				<h3>Latest Insight</h3>
				<p>{aiResult.insight}</p>
				<p class="muted">Source: {aiResult.source} | Model: {aiResult.model} | Prompt Version: {aiResult.prompt_version}</p>
				<p class="muted">Confidence: {aiResult.confidence} | Fallback Used: {aiResult.fallback_used ? 'yes' : 'no'}</p>
				<p class="muted">{aiResult.limitations}</p>
			</article>
		{/if}

		{#if aiHistory.length > 0}
			<h3>Recent AI Requests</h3>
			<ul class="logs">
				{#each aiHistory as entry}
					<li>
						<span class="stamp">[{prettyDate(entry.created_at)}]</span>
						<span class="level">{entry.status.toUpperCase()}</span>
						<span>{entry.provider} / {entry.model} / prompt {entry.prompt_version}</span>
					</li>
				{/each}
			</ul>
		{/if}
	{/if}
</section>

<style>
	.ai-controls {
		display: grid;
		gap: 0.6rem;
		margin-bottom: 0.8rem;
	}

	.ai-actions {
		display: flex;
		gap: 0.6rem;
		flex-wrap: wrap;
	}

	.ai-result {
		margin-top: 0.5rem;
		margin-bottom: 0.8rem;
	}
</style>
