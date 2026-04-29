<script lang="ts">
	import AppDangerZone from '$lib/components/app-danger-zone.svelte';
	import DeployHistoryTable from '$lib/components/deploy-history-table.svelte';
	import { goto } from '$app/navigation';
	import DeleteProgressPanel from '$lib/components/delete-progress-panel.svelte';
	import DeploymentStatusPanel from '$lib/components/deployment-status-panel.svelte';
	import { createAppDeleteFlow } from '$lib/app-delete-flow';
	import type { App } from '$lib/api';
	import { APIError, apiDELETE } from '$lib/api';
	import {
		deriveStatusKey,
		present,
		sanitizeSiteURL,
		statusLabel,
		statusProgressValue,
		statusTone
	} from '$lib/deploy-status';
	import { probeSiteReachability } from '$lib/site-reachability';
	import {
		appNoLongerExists as checkIfAppDeleted,
		createAppDetailsPoller,
		fetchAppDetailsBundle,
		queueDeployment,
		type HistoryResponse,
		type InfraOutputResponse,
		updateAutoDeploy
	} from './app-details-runtime';
	import { onDestroy, onMount } from 'svelte';

	export let data: { appID: string };

	let loading = false;
	let error = '';
	let actionError = '';
	let actionMessage = '';
	let actionBusy = false;
	let app: App | null = null;
	let history: HistoryResponse | null = null;
	let infraOutputs: InfraOutputResponse | null = null;
	let deployAccountID = '';
	let showDeleteConfirm = false;
	let deleteConfirmInput = '';
	let deployCompletionFXTimer: number | null = null;
	let deployCompletionFXActive = false;
	let hasSeenLatestProgress = false;
	let previousLatestProgress = 0;
	let latestSiteReachable = false;
	let latestReachabilityTarget = '';
	let autoPollingActive = false;
	const pendingPollIntervalMS = 3000;
	const completionFXDurationMS = 5600;

	// this helper owns delete progress and retry behavior so the page stays readable
	const deleteFlow = createAppDeleteFlow({
		appID: Number(data.appID),
		getAppName: () => app?.name ?? '',
		checkDeleted: checkIfAppDeleted,
		onRedirectToApps: async () => goto('/apps')
	});
	const deleteFlowState = deleteFlow.state;

	$: latest = history?.deployments?.[0] ?? null;
	$: latestSiteURL = sanitizeSiteURL(latest?.site_url || app?.site_url);
	$: cloudfrontURL = sanitizeSiteURL(
		infraOutputs?.outputs.site_url || latest?.site_url || app?.site_url
	);
	$: latestStatusSiteURL = cloudfrontURL || latestSiteURL;
	$: latestStatusTone = statusTone(latest?.status, latestStatusSiteURL, latestSiteReachable);
	$: latestStatusLabel = statusLabel(latest?.status, latestStatusSiteURL, latestSiteReachable);
	$: latestStatusProgress = statusProgressValue(
		latest?.status,
		latestStatusSiteURL,
		latestSiteReachable
	);
	$: latestStatusAwaitingURL =
		deriveStatusKey(latest?.status, latestStatusSiteURL, latestSiteReachable) === 'awaiting_site_url';
	$: deleteInProgress = $deleteFlowState.inProgress;
	$: deleteProgress = $deleteFlowState.progress;
	$: deleteProgressLabel = $deleteFlowState.label;
	$: deleteSuccessNotice = $deleteFlowState.successNotice;
	$: deleteRedirectCountdown = $deleteFlowState.redirectCountdown;
	$: deleteCompletionFXActive = $deleteFlowState.completionFXActive;
	$: activeDeleteSession = $deleteFlowState.activeSession;
	$: deletePhrase = app ? buildDeletePhrase(app.name) : '';
	$: deletePhraseMatches = deleteConfirmInput.trim().toLowerCase() === deletePhrase;
	$: backToAppsHref = '/apps';

	// separate poller keeps interval logic out of the main route file
	const poller = createAppDetailsPoller({
		intervalMS: pendingPollIntervalMS,
		shouldPoll: shouldAutoPoll,
		onTick: pollRefresh,
		onActiveChange: (active) => {
			autoPollingActive = active;
		}
	});

	$: {
		if (!loading && latestStatusSiteURL !== latestReachabilityTarget) {
			// reprobe only when url target changes so we dont spam checks
			void checkLatestSiteReachability(latestStatusSiteURL);
		}
	}

	$: {
		// this drives the one time completion glow when progress crosses 100
		if (!hasSeenLatestProgress) {
			hasSeenLatestProgress = true;
			previousLatestProgress = latestStatusProgress;
		} else {
			const crossedToComplete =
				previousLatestProgress < 100 && latestStatusProgress >= 100 && latestStatusTone === 'ok';
			if (crossedToComplete) {
				triggerDeployCompletionFX();
			}
			previousLatestProgress = latestStatusProgress;
		}
	}

	async function checkLatestSiteReachability(url: string) {
		latestReachabilityTarget = url;
		if (!url) {
			latestSiteReachable = false;
			return;
		}
		latestSiteReachable = await probeSiteReachability(url);
	}

	function buildDeletePhrase(name: string): string {
		// normalize to avoid weird spacing or punctuation mismatch while typing confirm text
		const normalized = name
			.trim()
			.toLowerCase()
			.replace(/[^a-z0-9]+/g, '-')
			.replace(/^-+|-+$/g, '');
		return `delete-${normalized || 'app'}`;
	}

	function triggerDeployCompletionFX() {
		if (typeof window === 'undefined') return;
		deployCompletionFXActive = true;
		if (deployCompletionFXTimer !== null) {
			window.clearTimeout(deployCompletionFXTimer);
		}
		deployCompletionFXTimer = window.setTimeout(() => {
			deployCompletionFXActive = false;
			deployCompletionFXTimer = null;
		}, completionFXDurationMS);
	}

	function consumeInitialDeployFeedback() {
		if (typeof window === 'undefined') return;
		const current = new URL(window.location.href);
		const initialDeployError = current.searchParams.get('initial_deploy_error')?.trim() ?? '';
		if (initialDeployError.length > 0) {
			actionError = `App created, but initial deploy could not be queued: ${initialDeployError}`;
		}

		current.searchParams.delete('queued_deploy_id');
		current.searchParams.delete('initial_deploy_error');
		// clean the url so refreshes dont keep showing old one time messages
		const nextPath =
			current.pathname +
			(current.searchParams.toString() ? `?${current.searchParams.toString()}` : '');
		window.history.replaceState({}, '', nextPath);
	}

	type LoadPageOptions = {
		background?: boolean;
	};

	async function loadPage(options: LoadPageOptions = {}) {
		const background = options.background === true;
		if (!background) {
			// full load resets top level error and live url probe state
			loading = true;
			error = '';
			latestReachabilityTarget = '';
			latestSiteReachable = false;
		}
		try {
			const details = await fetchAppDetailsBundle(Number(data.appID));
			app = details.app;
			history = details.history;
			infraOutputs = details.infraOutputs;
			deployAccountID = details.deployAccountID;
		} catch (err) {
			if (err instanceof APIError && err.status === 404) {
				// if app is gone bounce back to list and clear any delete flow residue
				deleteFlow.clear();
				if (!background && typeof window !== 'undefined') {
					await goto('/apps');
					return;
				}
			}
			if (!background) {
				error = err instanceof Error ? err.message : 'failed to load app details';
				app = null;
				history = null;
				infraOutputs = null;
				deployAccountID = '';
			}
		} finally {
			if (!background) {
				loading = false;
			}
			poller.sync();
		}
	}

	function shouldAutoPoll(): boolean {
		if (loading || actionBusy) return false;
		if (!app || !history) return false;
		// only poll when deploy is still moving or waiting on site url health
		const status = deriveStatusKey(latest?.status, latestStatusSiteURL, latestSiteReachable);
		if (status === 'queued' || status === 'running' || status === 'awaiting_site_url') {
			return true;
		}
		return false;
	}

	async function pollRefresh() {
		if (actionBusy) return;
		await loadPage({ background: true });
		// keep probing cloudfront url while backend is still waiting to mark it ready
		if (latestStatusSiteURL && latestStatusAwaitingURL) {
			await checkLatestSiteReachability(latestStatusSiteURL);
		}
	}

	async function deployNow() {
		if (!app) return;
		actionBusy = true;
		actionError = '';
		actionMessage = '';
		try {
			const deploymentID = await queueDeployment(app.id);
			actionMessage = `Deployment #${deploymentID || '?'} queued`;
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
			const updated = await updateAutoDeploy(app, !app.auto_deploy_enabled);
			app = updated;
			await loadPage();
		} catch (err) {
			actionError = err instanceof Error ? err.message : 'failed to update app';
		} finally {
			actionBusy = false;
		}
	}

	async function deleteApp() {
		if (!app) return;
		if (!deletePhraseMatches) {
			actionError = `Type "${deletePhrase}" to confirm deletion.`;
			return;
		}

		// start optimistic progress right away so user sees action feedback fast
		actionBusy = true;
		actionError = '';
		actionMessage = '';
		deleteFlow.startDeleteProgress('request', Date.now());
		let deleted = false;
		try {
			await apiDELETE(`/v1/apps/${app.id}`);
			deleted = true;
		} catch (err) {
			const message = err instanceof Error ? err.message : 'failed to delete app';
			if (message.includes('(504)') || message.toLowerCase().includes('timed out')) {
				// timeout does not always mean failure so we switch into verify mode
				deleteFlow.markVerifyingFromTimeout();
				deleted = await deleteFlow.waitForDeletion(app.id);
				if (!deleted) {
					actionError = 'Deletion is still in progress. Please wait about a minute, then refresh and retry if needed.';
				}
			} else {
				actionError = message;
			}
		} finally {
			if (!deleted) {
				actionBusy = false;
				deleteFlow.handleFailureAfterDeleteRequest();
				return;
			}
		}
		await deleteFlow.completeAndRedirect();
	}

	function toggleDeleteConfirm() {
		showDeleteConfirm = !showDeleteConfirm;
		if (!showDeleteConfirm) {
			// when closing prompt we also clear transient state so next open is clean
			deleteConfirmInput = '';
			if (!deleteInProgress) {
				deleteFlow.clear();
			}
			actionBusy = false;
		}
	}

	onMount(async () => {
		consumeInitialDeployFeedback();
		await deleteFlow.restoreIfNeeded();
		if (deleteFlow.getSnapshot().inProgress) {
			showDeleteConfirm = true;
			actionBusy = true;
			// do background load only so we dont break ongoing delete progress ui
			await loadPage({ background: true });
			return;
		}
		await loadPage();
	});

	onDestroy(() => {
		poller.destroy();
		deleteFlow.destroy();
		if (typeof window !== 'undefined') {
			if (deployCompletionFXTimer !== null) {
				window.clearTimeout(deployCompletionFXTimer);
			}
		}
	});

	$: poller.sync();
</script>

	<section class="page">
			<div class="toolbar">
				<div>
					<a href={backToAppsHref} class="back">← Back to apps</a>
					<h1>App Deploy History</h1>
				</div>
			<div class="controls">
				<div class="action-row">
					<div class="run-controls">
						<button class="action-btn action-btn-primary" on:click={deployNow} disabled={actionBusy || loading}>
							Deploy Now
						</button>
						<button class="action-btn action-btn-secondary" on:click={() => loadPage()} disabled={loading}>
							Refresh
						</button>
					</div>
					<div class="auto-toggle-wrap">
						<div class="auto-poll-slot">
							{#if autoPollingActive}
								<span class="muted poll-indicator">Auto-refreshing…</span>
							{/if}
						</div>
						<button
							class={`action-btn auto-toggle ${app?.auto_deploy_enabled ? 'enabled' : 'disabled'}`}
							on:click={toggleAutoDeploy}
							disabled={actionBusy || loading}
						>
							{app?.auto_deploy_enabled ? 'Disable Auto-Deploy' : 'Enable Auto-Deploy'}
						</button>
					</div>
				</div>
			</div>
		</div>

	{#if loading}
		<p class="muted">Loading app history...</p>
	{:else if error}
		<p class="error">{error}</p>
	{:else if deleteInProgress}
		<article class="card deleting-standalone">
			<h2>Deleting App</h2>
			{#if activeDeleteSession?.appName}
				<p>
					<strong>App:</strong>
					{activeDeleteSession.appName}
				</p>
			{/if}
			<p class="muted">
				App record removal can finish before AWS infrastructure teardown is fully complete.
			</p>
			<DeleteProgressPanel
				label={deleteProgressLabel}
				progress={deleteProgress}
				successNote={deleteSuccessNotice}
				countdown={deleteRedirectCountdown}
				completionFXActive={deleteCompletionFXActive}
			/>
		</article>
	{:else if !app || !history}
		<p class="muted">No app data.</p>
	{:else}
		{#if actionError}
			<p class="error">{actionError}</p>
		{:else if actionMessage}
			<p class="ok">{actionMessage}</p>
		{/if}

		<div class="summary-grid">
			<article class="app-overview-card">
				<div class="overview-top">
					<div class="overview-meta">
						<h2>{app.name}</h2>
						<p><strong>Repo:</strong> {app.repo_full_name}</p>
						<p><strong>Branch:</strong> {app.branch}</p>
					</div>
				<div class="overview-status">
					<DeploymentStatusPanel
						tone={latestStatusTone}
						label={latestStatusLabel}
						progress={latestStatusProgress}
						awaitingURL={latestStatusAwaitingURL}
						pulseOnce={deployCompletionFXActive}
						ariaPrefix="Latest deploy progress"
					/>
				</div>
			</div>
				<div class="infra-meta">
					<p><strong>AWS Account:</strong> {present(deployAccountID)}</p>
					<p><strong>Bucket:</strong> {present(infraOutputs?.outputs.bucket_name)}</p>
				</div>
			</article>
		</div>

		<section class="site-url-focus">
			<div class="site-url-row">
				<p class="site-url-kicker">Current Site URL</p>
				{#if !latestStatusSiteURL}
					<p class="muted">Pending first successful cloud deployment</p>
				{/if}
			</div>
			{#if latestStatusSiteURL}
				<a class="site-url-value" href={latestStatusSiteURL} target="_blank" rel="noreferrer"
					>{latestStatusSiteURL}</a
				>
			{/if}
		</section>

		<DeployHistoryTable
			deployments={history.deployments}
			latestStatusSiteURL={latestStatusSiteURL}
			latestSiteReachable={latestSiteReachable}
		/>

		<AppDangerZone
			showDeleteConfirm={showDeleteConfirm}
			bind:deleteConfirmInput
			deletePhrase={deletePhrase}
			actionBusy={actionBusy}
			loading={loading}
			deletePhraseMatches={deletePhraseMatches}
			deleteInProgress={deleteInProgress}
			deleteProgressLabel={deleteProgressLabel}
			deleteProgress={deleteProgress}
			deleteSuccessNotice={deleteSuccessNotice}
			deleteRedirectCountdown={deleteRedirectCountdown}
			deleteCompletionFXActive={deleteCompletionFXActive}
			onToggle={toggleDeleteConfirm}
			onDelete={deleteApp}
		/>
	{/if}
</section>

<style>
	.page {
		display: grid;
		gap: 1.45rem;
	}

	.toolbar {
		display: flex;
		align-items: flex-end;
		justify-content: space-between;
		gap: 1rem;
		padding-bottom: 0;
	}

	.toolbar h1 {
		margin: 0.3rem 0 0;
	}

	.controls {
		display: grid;
		justify-items: end;
		gap: 0.42rem;
	}

	.action-row {
		display: flex;
		align-items: flex-end;
		justify-content: flex-end;
		flex-wrap: wrap;
		gap: 0.9rem;
	}

	.run-controls {
		display: flex;
		align-items: center;
		gap: 0.65rem;
	}

	.action-btn {
		width: fit-content;
		min-height: 2.3rem;
		min-width: 7.9rem;
		padding: 0.56rem 1.05rem;
		border-radius: 0.72rem;
		font-weight: 700;
		letter-spacing: 0.01em;
		box-shadow: none;
	}

	.action-btn:hover {
		box-shadow: none;
		transform: none;
		filter: none;
	}

	.action-btn-primary {
		background: linear-gradient(138deg, rgba(125, 196, 228, 0.45), rgba(138, 173, 244, 0.38));
		border-color: rgba(125, 196, 228, 0.76);
		color: #e8ecff;
	}

	.action-btn-secondary {
		color: #d8f7f4;
		background: linear-gradient(138deg, rgba(79, 171, 165, 0.36), rgba(93, 190, 182, 0.28));
		border-color: rgba(114, 214, 204, 0.5);
		box-shadow: none;
	}

	.action-btn-secondary:hover {
		background: linear-gradient(138deg, rgba(93, 190, 182, 0.44), rgba(110, 212, 202, 0.34));
		border-color: rgba(130, 227, 218, 0.62);
		box-shadow: none;
	}

	.auto-toggle {
		min-width: 11.25rem;
	}

	.auto-toggle-wrap {
		display: grid;
		justify-items: end;
		gap: 0.2rem;
	}

	.auto-poll-slot {
		min-height: 1rem;
		display: flex;
		align-items: end;
		justify-content: flex-end;
	}

	.auto-toggle.enabled {
		background: linear-gradient(135deg, rgba(238, 212, 159, 0.28), rgba(245, 169, 127, 0.24));
		border-color: rgba(245, 169, 127, 0.62);
		color: #ffe9cd;
		box-shadow: none;
	}

	.auto-toggle.enabled:hover {
		background: linear-gradient(135deg, rgba(238, 212, 159, 0.38), rgba(245, 169, 127, 0.34));
		border-color: rgba(245, 169, 127, 0.76);
	}

	.auto-toggle.disabled {
		background: linear-gradient(135deg, rgba(166, 218, 149, 0.24), rgba(139, 213, 202, 0.22));
		border-color: rgba(139, 213, 202, 0.58);
		color: #e8ffe0;
		box-shadow: none;
	}

	.auto-toggle.disabled:hover {
		background: linear-gradient(135deg, rgba(166, 218, 149, 0.34), rgba(139, 213, 202, 0.32));
		border-color: rgba(139, 213, 202, 0.74);
	}

	.poll-indicator {
		font-size: 0.82rem;
		margin-right: 0.1rem;
		white-space: nowrap;
	}

	.app-overview-card {
		display: grid;
		gap: 0.36rem;
	}

	.app-overview-card h2 {
		margin: 0 0 0.2rem;
	}

	.app-overview-card p {
		margin: 0.16rem 0;
	}

	.overview-meta p {
		margin: 0.16rem 0;
	}

	.overview-top {
		display: flex;
		gap: 0.68rem;
		align-items: flex-start;
		justify-content: space-between;
	}

	.overview-meta {
		min-width: 0;
	}

	.overview-status {
		min-width: 394px;
		max-width: 488px;
		flex: 0 0 auto;
		padding: 0.56rem 0.62rem 0.42rem;
		border-radius: 0.68rem;
		border: 1px solid rgba(183, 189, 248, 0.26);
		background: rgba(30, 33, 50, 0.62);
	}

	.summary-grid {
		margin: 0 0 0.6rem;
	}

	.site-url-focus {
		margin: 0 0 0.6rem;
		padding: 0.72rem 0.86rem 0.78rem;
		border-radius: 0.72rem;
		border: 1px solid rgba(154, 217, 255, 0.42);
		background: linear-gradient(145deg, rgba(154, 217, 255, 0.16), rgba(128, 181, 255, 0.08));
		box-shadow: inset 0 0 0 1px rgba(255, 255, 255, 0.02);
	}

	.site-url-row {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		justify-content: space-between;
		gap: 0.45rem 0.8rem;
	}

	.site-url-kicker {
		margin: 0;
		font-size: 1.04rem;
		font-weight: 700;
		letter-spacing: 0.06em;
		text-transform: uppercase;
		color: #9ad9ff;
	}

	.site-url-value {
		display: block;
		margin-top: 0.38rem;
		font-weight: 700;
		color: #dceeff;
		font-size: 0.98rem;
		line-height: 1.36;
		word-break: break-word;
		text-decoration: none;
	}

	.site-url-value:hover {
		text-decoration: underline;
	}

	.infra-meta {
		display: grid;
		gap: 0.24rem;
		margin-top: 0;
	}

	.infra-meta p {
		margin: 0;
		line-height: 1.34;
	}

	.deleting-standalone {
		display: grid;
		gap: 0.45rem;
	}

	.deleting-standalone h2 {
		margin: 0 0 0.1rem;
	}

	.deleting-standalone p {
		margin: 0.15rem 0;
	}

	@media (max-width: 760px) {
		.toolbar {
			flex-direction: column;
			align-items: stretch;
			gap: 0.75rem;
		}

		.controls {
			width: 100%;
			justify-items: stretch;
		}

		.action-row {
			width: 100%;
			flex-direction: column;
			align-items: stretch;
		}

		.run-controls {
			width: 100%;
			display: grid;
			grid-template-columns: 1fr 1fr;
		}

		.auto-toggle {
			width: 100%;
		}

		.overview-top {
			flex-direction: column;
			gap: 0.6rem;
		}

		.overview-status {
			min-width: 0;
			max-width: none;
			width: 100%;
		}

		.poll-indicator {
			align-self: start;
			margin-right: 0;
		}
	}

</style>
