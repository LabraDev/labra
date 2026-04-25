<script lang="ts">
	import type { App } from '$lib/api';
	import { APIError, apiGET, apiPOST, prettyDate } from '$lib/api';
	import { onMount } from 'svelte';

	type GitHubRepository = {
		full_name: string;
		name: string;
		default_branch: string;
		private: boolean;
		html_url: string;
	};
	type GitHubBranch = {
		name: string;
		protected: boolean;
	};
	type AppsTab = 'create' | 'apps';

	let loading = false;
	let error = '';
	let apps: App[] = [];

	let reposLoading = false;
	let reposError = '';
	let repositories: GitHubRepository[] = [];
	let branchesLoading = false;
	let branchesError = '';
	let branches: GitHubBranch[] = [];
	let installURL = '';
	let installURLLoading = false;
	let installURLError = '';
	let installStatus = '';
	let needsGitHubInstall = false;
	let needsGitHubConfig = false;
	let refreshAppsBusy = false;
	let refreshReposBusy = false;
	let activeTab: AppsTab = 'create';

	let createBusy = false;
	let createError = '';
	let createSuccess = '';
	let appName = '';
	let repoFullName = '';
	let branch = 'main';
	let showAdvanced = false;
	let outputDirPreset: 'dist' | 'build' | 'out' | 'custom' = 'dist';
	let customOutputDir = '';
	let autoDeployEnabled = true;
	let resolvedOutputDir = 'dist';

	$: selectedRepo = repositories.find((repo) => repo.full_name === repoFullName) ?? null;
	$: installActionLabel = needsGitHubInstall ? 'Install GitHub App' : 'Update Repository Configuration';
	$: reposErrorDisplay = normalizeReposError(reposError);
	$: needsGitHubInstall = requiresGitHubInstall(reposError);
	$: needsGitHubConfig = requiresGitHubConfig(reposError);
	$: resolvedOutputDir = outputDirPreset === 'custom' ? customOutputDir.trim() : outputDirPreset;
	$: hasValidOutputDir = outputDirPreset !== 'custom' || customOutputDir.trim().length > 0;

	function normalizeReposError(message: string): string {
		const trimmed = message.trim();
		if (trimmed.length === 0) return '';

		const value = trimmed.toLowerCase();
		if (value.includes('github oauth token missing') || value.includes('sign in with github again')) {
			return 'GitHub App installation missing. Install GitHub App and select repositories.';
		}
		return trimmed;
	}

	function requiresGitHubInstall(message: string): boolean {
		const value = message.trim().toLowerCase();
		if (value.length === 0) return false;
		return (
			value.includes('github app installation missing') ||
			value.includes('install github app') ||
			value.includes('github app installation not found') ||
			value.includes('github oauth token missing') ||
			value.includes('sign in with github again')
		);
	}

	function requiresGitHubConfig(message: string): boolean {
		const value = message.trim().toLowerCase();
		if (value.length === 0) return false;
		return (
			value.includes('github app credentials are not configured') ||
			value.includes('github app slug is not configured')
		);
	}

	function openGitHubInstall() {
		if (typeof window === 'undefined') return;
		if (installURL.trim().length === 0) return;
		window.location.href = installURL;
	}

	function redirectHomeForAuthError(err: unknown): boolean {
		if (typeof window === 'undefined') return false;
		if (err instanceof APIError && err.status === 401) {
			window.location.href = '/';
			return true;
		}
		if (err instanceof Error) {
			const message = err.message.toLowerCase();
			if (message.includes('not logged in') || message.includes('missing auth principal')) {
				window.location.href = '/';
				return true;
			}
		}
		return false;
	}

	async function loadInstallURL() {
		installURLLoading = true;
		installURLError = '';
		try {
			const data = await apiGET<{ install_url: string }>('/v1/github/install-url');
			installURL = data.install_url?.trim() ?? '';
		} catch (err) {
			if (redirectHomeForAuthError(err)) return;
			installURL = '';
			installURLError = err instanceof Error ? err.message : 'failed to load install URL';
		} finally {
			installURLLoading = false;
		}
	}

	async function syncGitHubInstallationFromURL() {
		if (typeof window === 'undefined') return;
		const current = new URL(window.location.href);
		const rawInstallationID = current.searchParams.get('installation_id')?.trim() ?? '';
		if (rawInstallationID.length === 0) return;

		const installationID = Number(rawInstallationID);
		if (!Number.isInteger(installationID) || installationID <= 0) {
			reposError = 'GitHub App installation callback is invalid. Reinstall GitHub App and retry.';
		} else {
			try {
				await apiPOST('/v1/github/installation', {
					installation_id: installationID,
					account_login: current.searchParams.get('account')?.trim() ?? '',
					target_type: current.searchParams.get('target_type')?.trim() ?? ''
				});
				installStatus = 'GitHub App connected. Loading allowed repositories...';
				reposError = '';
			} catch (err) {
				reposError = err instanceof Error ? err.message : 'failed to connect GitHub App installation';
			}
		}

		current.searchParams.delete('installation_id');
		current.searchParams.delete('setup_action');
		current.searchParams.delete('state');
		current.searchParams.delete('account');
		current.searchParams.delete('target_type');
		const nextPath = current.pathname + (current.searchParams.toString() ? `?${current.searchParams.toString()}` : '');
		window.history.replaceState({}, '', nextPath);
	}

	function applyRepoDefaults() {
		if (!selectedRepo) return;
		if (appName.trim().length === 0) {
			appName = selectedRepo.name;
		}
		if (branch.trim().length === 0 || branch === 'main') {
			branch = selectedRepo.default_branch || 'main';
		}
	}

	async function loadBranchesForRepo(nextRepoFullName: string, preferDefault: boolean) {
		const repo = nextRepoFullName.trim();
		branchesError = '';
		if (repo.length === 0) {
			branches = [];
			return;
		}

		branchesLoading = true;
		try {
			const data = await apiGET<{ branches: GitHubBranch[] }>(
				`/v1/github/branches?repo_full_name=${encodeURIComponent(repo)}`
			);
			branches = (data.branches ?? [])
				.filter((b) => b.name?.trim().length > 0)
				.map((b) => ({ ...b, name: b.name.trim() }))
				.sort((a, b) => a.name.localeCompare(b.name));

			if (branches.length === 0) {
				const fallbackBranch = selectedRepo?.default_branch?.trim() || branch.trim() || 'main';
				branches = [{ name: fallbackBranch, protected: false }];
				branch = fallbackBranch;
				return;
			}

			const currentBranch = branch.trim();
			const currentExists = branches.some((b) => b.name === currentBranch);
			if (!currentExists || preferDefault) {
				const preferred = selectedRepo?.default_branch?.trim() ?? '';
				branch = branches.find((b) => b.name === preferred)?.name || branches[0].name;
			}
		} catch (err) {
			const fallbackBranch = selectedRepo?.default_branch?.trim() || branch.trim() || 'main';
			branches = [{ name: fallbackBranch, protected: false }];
			branch = fallbackBranch;
			branchesError = '';
		} finally {
			branchesLoading = false;
		}
	}

	async function handleRepoChange() {
		applyRepoDefaults();
		await loadBranchesForRepo(repoFullName, true);
	}

	async function loadApps() {
		loading = true;
		error = '';
		try {
			const data = await apiGET<{ apps: App[] }>('/v1/apps');
			apps = data.apps ?? [];
		} catch (err) {
			if (redirectHomeForAuthError(err)) return;
			error = err instanceof Error ? err.message : 'failed to load apps';
			apps = [];
		} finally {
			loading = false;
		}
	}

	async function loadRepositories() {
		reposLoading = true;
		reposError = '';
		try {
			const data = await apiGET<{ repositories: GitHubRepository[] }>('/v1/github/repositories');
			repositories = (data.repositories ?? []).sort((a, b) => a.full_name.localeCompare(b.full_name));
			let selectedRepoChanged = false;
			if (repositories.length > 0 && repoFullName.trim().length === 0) {
				repoFullName = repositories[0].full_name;
				branch = repositories[0].default_branch || 'main';
				if (appName.trim().length === 0) {
					appName = repositories[0].name;
				}
				selectedRepoChanged = true;
			}
			if (repoFullName.trim().length > 0) {
				await loadBranchesForRepo(repoFullName, selectedRepoChanged);
			}
		} catch (err) {
			if (redirectHomeForAuthError(err)) return;
			repositories = [];
			branches = [];
			branchesError = '';
			reposError =
				err instanceof Error
					? err.message
					: 'failed to load repositories';
		} finally {
			reposLoading = false;
		}
	}

	async function createApp() {
		if (needsGitHubInstall || needsGitHubConfig) {
			createError = 'Connect GitHub first to select a repository.';
			return;
		}
		if (!hasValidOutputDir) {
			createError = 'Provide a custom output directory or pick a preset.';
			return;
		}

		createBusy = true;
		createError = '';
		createSuccess = '';
		try {
			const created = await apiPOST<App>('/v1/apps', {
				name: appName.trim(),
					repo_full_name: repoFullName.trim(),
					branch: branch.trim(),
					build_type: 'static',
					output_dir: resolvedOutputDir || 'dist',
					root_dir: '',
					site_url: '',
					auto_deploy_enabled: autoDeployEnabled
				});
			createSuccess = `Created ${created.name}. Opening app details...`;
			await loadApps();
			window.location.href = `/apps/${created.id}`;
		} catch (err) {
			if (redirectHomeForAuthError(err)) return;
			createError = err instanceof Error ? err.message : 'failed to create app';
		} finally {
			createBusy = false;
		}
	}

	async function refreshApps() {
		refreshAppsBusy = true;
		try {
			await loadApps();
		} finally {
			refreshAppsBusy = false;
		}
	}

	async function refreshRepos() {
		refreshReposBusy = true;
		try {
			await Promise.all([loadInstallURL(), loadRepositories()]);
		} finally {
			refreshReposBusy = false;
		}
	}

	onMount(async () => {
		await syncGitHubInstallationFromURL();
		await Promise.all([refreshApps(), refreshRepos()]);
	});
</script>

<section class="page">
	<div class="toolbar">
		<div>
			<h1>Apps</h1>
		</div>
	</div>

	<div class="tabs" role="tablist" aria-label="Apps sections">
		<button
			type="button"
			role="tab"
			class="tab-button"
			class:active={activeTab === 'create'}
			aria-selected={activeTab === 'create'}
			on:click={() => (activeTab = 'create')}
		>
			Create New App
		</button>
		<button
			type="button"
			role="tab"
			class="tab-button"
			class:active={activeTab === 'apps'}
			aria-selected={activeTab === 'apps'}
			on:click={() => (activeTab = 'apps')}
		>
			Existing Apps
		</button>
	</div>

	{#if activeTab === 'create'}
		<div class="card create-card" role="tabpanel">
			<div class="panel-head">
				<div>
					<h2>Create App</h2>
					<p class="muted">Pick a GitHub repository and branch, then Labra will manage deploy runs for this app.</p>
				</div>
				<button
					type="button"
					class="secondary"
					on:click={refreshRepos}
					disabled={refreshReposBusy || reposLoading || installURLLoading}
				>
					{refreshReposBusy || reposLoading || installURLLoading ? 'Refreshing Repos...' : 'Refresh Repos'}
				</button>
			</div>
			{#if installStatus}
				<p class="success">{installStatus}</p>
			{/if}

			{#if reposError}
				<p class="error">{reposErrorDisplay}</p>
				{#if needsGitHubInstall}
					<p class="muted">Install the GitHub App, then return here to choose a deployment repository.</p>
				{:else if needsGitHubConfig}
					<p class="muted">Labra GitHub App credentials are not configured on the backend yet.</p>
				{:else}
					<p class="muted">Manual fallback: enter repo as <code>owner/repo</code>.</p>
				{/if}
			{/if}

			{#if !needsGitHubConfig}
				{#if installURL}
					<button type="button" class="secondary install-button" on:click={openGitHubInstall}>
						{installActionLabel}
					</button>
				{:else if installURLLoading}
					<p class="muted">Loading install URL...</p>
				{:else if installURLError}
					<p class="muted">{installURLError}</p>
				{/if}
			{/if}

			<label for="repo">Repository</label>
			{#if repositories.length > 0}
				<select
					id="repo"
					bind:value={repoFullName}
					on:change={handleRepoChange}
				>
					{#each repositories as repo}
						<option value={repo.full_name}>
							{repo.full_name} {repo.private ? '(private)' : '(public)'}
						</option>
					{/each}
				</select>
			{:else if needsGitHubInstall || needsGitHubConfig}
				<input id="repo" value="" placeholder="Connect GitHub to load repositories" disabled />
			{:else}
				<input id="repo" bind:value={repoFullName} placeholder="owner/repo" />
			{/if}

			{#if selectedRepo}
				<div class="repo-link-row">
					<a class="repo-link" href={selectedRepo.html_url} target="_blank" rel="noreferrer">Open repo</a>
				</div>
			{/if}

			<label for="app-name">App Name</label>
			<input id="app-name" bind:value={appName} placeholder="my-app" />

			<label for="branch">Branch</label>
			{#if branchesLoading}
				<input id="branch" value={branch || 'Loading branches...'} disabled />
			{:else if branches.length > 0}
				<select id="branch" bind:value={branch}>
					{#each branches as branchOption}
						<option value={branchOption.name}>{branchOption.name}</option>
					{/each}
				</select>
			{:else}
				<input id="branch" bind:value={branch} placeholder="main" />
			{/if}
			{#if branchesError}
				<p class="muted">Branch list unavailable right now. Enter branch manually.</p>
			{/if}

			<details class="advanced-settings" bind:open={showAdvanced}>
				<summary>Advanced</summary>
				<div class="advanced-grid">
					<label for="output-dir-preset">Output Directory</label>
					<select id="output-dir-preset" bind:value={outputDirPreset}>
						<option value="dist">dist</option>
						<option value="build">build</option>
						<option value="out">out</option>
						<option value="custom">Custom...</option>
					</select>
					{#if outputDirPreset === 'custom'}
						<label for="output-dir-custom">Custom Output Directory</label>
						<input id="output-dir-custom" bind:value={customOutputDir} placeholder="dist" />
					{/if}
				</div>
			</details>

			<label class="checkbox-row" for="auto-deploy">
				<input id="auto-deploy" type="checkbox" bind:checked={autoDeployEnabled} />
				<span>Enable auto-deploy on new push events</span>
			</label>

			<button
				on:click={createApp}
				disabled={createBusy || needsGitHubInstall || needsGitHubConfig || appName.trim().length === 0 || repoFullName.trim().length === 0 || !hasValidOutputDir}
			>
				{createBusy ? 'Creating...' : 'Create App'}
			</button>

			{#if createError}
				<p class="error">{createError}</p>
			{:else if createSuccess}
				<p class="success">{createSuccess}</p>
			{/if}
		</div>
	{:else}
		<div role="tabpanel">
			<div class="panel-head">
				<div>
					<h2>Existing Apps</h2>
				</div>
				<button type="button" on:click={refreshApps} disabled={refreshAppsBusy || loading}>
					{refreshAppsBusy || loading ? 'Refreshing Apps...' : 'Refresh Apps'}
				</button>
			</div>
			{#if loading}
				<p class="muted">Loading apps...</p>
			{:else if error}
				<p class="error">{error}</p>
			{:else if apps.length === 0}
				<p class="muted">No apps yet. Create your first app in the Create New App tab.</p>
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
		</div>
	{/if}
</section>

<style>
	.tabs {
		display: flex;
		gap: 0.5rem;
		flex-wrap: wrap;
	}

	.tab-button {
		background: rgba(30, 32, 48, 0.82);
		color: var(--text-color, #cad3f5);
		border: 1px solid rgba(183, 189, 248, 0.32);
		box-shadow: none;
	}

	.tab-button.active {
		background: linear-gradient(130deg, var(--blue, #8aadf4), var(--lavender, #b7bdf8));
		color: var(--crust, #181926);
		border-color: rgba(138, 173, 244, 0.6);
	}

	.panel-head {
		display: flex;
		justify-content: space-between;
		align-items: flex-start;
		gap: 0.8rem;
		flex-wrap: wrap;
	}

	.create-card {
		display: grid;
		gap: 0.8rem;
		margin-bottom: 0.45rem;
	}

	.repo-link-row {
		margin-top: -0.05rem;
		margin-bottom: 0.2rem;
		padding-left: 0.7rem;
	}

	.repo-link {
		display: inline-flex;
		width: fit-content;
		align-items: center;
		text-decoration: none;
		font-size: 0.92rem;
		font-weight: 600;
		color: var(--blue, #8aadf4);
	}

	.repo-link:hover {
		color: var(--lavender, #b7bdf8);
	}

	.install-button {
		width: fit-content;
	}

	.checkbox-row {
		display: flex;
		align-items: center;
		gap: 0.55rem;
	}

	.checkbox-row input[type='checkbox'] {
		width: 1.05rem;
		height: 1.05rem;
	}

	.advanced-settings {
		justify-self: start;
		width: min(100%, 20rem);
		border: 1px solid rgba(183, 189, 248, 0.24);
		border-radius: var(--radius-sm);
		padding: 0.4rem 0.6rem;
		background: rgba(24, 25, 38, 0.28);
	}

	.advanced-settings summary {
		display: inline-flex;
		align-items: center;
		cursor: pointer;
		font-size: 0.86rem;
		font-weight: 600;
		letter-spacing: 0.01em;
		list-style: none;
	}

	.advanced-settings summary::-webkit-details-marker {
		display: none;
	}

	.advanced-settings summary::before {
		content: '▸';
		display: inline-block;
		margin-right: 0.35rem;
		transition: transform 120ms ease;
	}

	.advanced-settings[open] summary::before {
		transform: rotate(90deg);
	}

	.advanced-grid {
		display: grid;
		gap: 0.4rem;
		margin-top: 0.45rem;
	}

	.advanced-grid label {
		font-size: 0.82rem;
		opacity: 0.86;
	}

	.advanced-grid :is(select, input) {
		width: min(100%, 14.5rem);
		font-size: 0.9rem;
		padding: 0.42rem 0.58rem;
	}
</style>
