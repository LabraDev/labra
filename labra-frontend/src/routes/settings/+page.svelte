<script lang="ts">
	import { onMount } from 'svelte';
	import { slide } from 'svelte/transition';
	import { apiDELETE, apiGET, apiPOST } from '$lib/api';

	type AWSConnection = {
		id: number;
		role_arn: string;
		external_id: string;
		region: string;
		account_id: string;
		status: string;
		updated_at: number;
	};
	type AWSAccessTab = 'connection' | 'connected';

	const defaultPlatformPrincipalArn =
		(import.meta.env.VITE_PLATFORM_PRINCIPAL_ARN ?? '').trim() ||
		'arn:aws:iam::974646089985:role/labra-infra-dev-platform-backend-service-role';
	const defaultRoleName = 'LabraCustomerDeployRole';
	const defaultStackName = 'labra-customer-assume-role';
	const setupStartedStorageKey = 'labra_aws_setup_started';
	const externalIDStorageKey = 'labra_aws_external_id';
	const regionStorageKey = 'labra_aws_region';
	const usRegions = ['us-west-1', 'us-west-2', 'us-east-1', 'us-east-2'];
	const defaultCloudFormationTemplateS3URL =
		'https://labra-infra-dev-platform-974646089985-site.s3.us-west-1.amazonaws.com/customer-assume-role.cfn.yaml';
	const configuredCloudFormationTemplateS3URL = (
		import.meta.env.VITE_CLOUDFORMATION_TEMPLATE_S3_URL ?? ''
	).trim();

	let roleARN = '';
	let externalID = '';
	let region = '';
	let cloudFormationStackName = defaultStackName;
	let cloudFormationRoleName = defaultRoleName;
	let platformPrincipalArn = defaultPlatformPrincipalArn;
	let showCloudFormationAdvanced = false;
	let showConnectionAdvanced = false;
	let showSaveStep = false;
	let loading = false;
	let error = '';
	let success = '';
	let connections: AWSConnection[] = [];
	let templateURL = '';
	let quickCreateTemplateURL =
		configuredCloudFormationTemplateS3URL || defaultCloudFormationTemplateS3URL;
	let activeTab: AWSAccessTab = 'connection';

	function generateExternalID() {
		if (typeof window === 'undefined' || !window.crypto?.getRandomValues) {
			return `labra-ext-${Math.random().toString(16).slice(2)}${Date.now().toString(16)}`;
		}
		const bytes = new Uint8Array(16);
		window.crypto.getRandomValues(bytes);
		const hex = Array.from(bytes, (b) => b.toString(16).padStart(2, '0')).join('');
		return `labra-ext-${hex}`;
	}

	function buildQuickCreateURL(effectiveExternalID: string) {
		const templateURL = quickCreateTemplateURL.trim();
		if (!templateURL || region.trim().length === 0) return '';

		const params = new URLSearchParams();
		params.set('templateURL', templateURL);
		params.set('stackName', cloudFormationStackName.trim() || defaultStackName);
		params.set(
			'param_PlatformPrincipalArn',
			platformPrincipalArn.trim() || defaultPlatformPrincipalArn
		);
		params.set('param_ExternalId', effectiveExternalID);
		params.set('param_RoleName', cloudFormationRoleName.trim() || defaultRoleName);

		return `https://console.aws.amazon.com/cloudformation/home?region=${encodeURIComponent(region)}#/stacks/quickcreate?${params.toString()}`;
	}

	function startQuickCreate() {
		if (typeof window === 'undefined') return;
		error = '';
		const effectiveExternalID = externalID.trim() || generateExternalID();
		const destination = buildQuickCreateURL(effectiveExternalID);
		if (!destination) {
			error = 'Unable to open Quick Create right now. Re-select region and try again.';
			return;
		}
		externalID = effectiveExternalID;
		showSaveStep = true;
		window.localStorage.setItem(setupStartedStorageKey, '1');
		window.localStorage.setItem(externalIDStorageKey, effectiveExternalID);
		window.localStorage.setItem(regionStorageKey, region);
		const newTab = window.open('', '_blank');
		if (!newTab) {
			error = 'Pop-up blocked. Please allow pop-ups for Labra and try again.';
			return;
		}
		newTab.opener = null;
		newTab.location.href = destination;
	}

	async function refreshConnections() {
		const result = await apiGET<{ aws_connections: AWSConnection[] }>('/v1/aws-connections');
		connections = result.aws_connections;
	}

	async function connectAWS() {
		error = '';
		success = '';
		loading = true;
		try {
			await apiPOST('/v1/aws-connections', {
				role_arn: roleARN,
				external_id: externalID,
				region
			});
			success = 'AWS connection validated and saved.';
			if (typeof window !== 'undefined') {
				window.localStorage.removeItem(setupStartedStorageKey);
				window.localStorage.setItem(externalIDStorageKey, externalID.trim());
				window.localStorage.setItem(regionStorageKey, region.trim());
			}
			await refreshConnections();
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to connect AWS';
		} finally {
			loading = false;
		}
	}

	async function disconnectConnection(connectionID: number) {
		error = '';
		success = '';
		loading = true;
		try {
			await apiDELETE(`/v1/aws-connections/${connectionID}`);
			success = 'AWS connection removed.';
			await refreshConnections();
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to disconnect AWS connection';
		} finally {
			loading = false;
		}
	}

	onMount(async () => {
		if (typeof window !== 'undefined') {
			templateURL = `${window.location.origin}/customer-assume-role.cfn.yaml`;
			const setupStarted = window.localStorage.getItem(setupStartedStorageKey) === '1';
			const storedExternalID = (window.localStorage.getItem(externalIDStorageKey) ?? '').trim();
			const storedRegion = (window.localStorage.getItem(regionStorageKey) ?? '').trim();
			if (storedExternalID) {
				externalID = storedExternalID;
			}
			if (!externalID) {
				externalID = generateExternalID();
				window.localStorage.setItem(externalIDStorageKey, externalID);
			}
			if (storedRegion) {
				region = storedRegion;
			}
			showSaveStep = setupStarted;
		}

		try {
			await refreshConnections();
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load AWS connections';
		}
	});
</script>

<section class="page">
	<h1>AWS Access</h1>

	<div class="tabs" role="tablist" aria-label="AWS access sections">
		<button
			type="button"
			role="tab"
			class="tab-button"
			class:active={activeTab === 'connection'}
			aria-selected={activeTab === 'connection'}
			on:click={() => (activeTab = 'connection')}
		>
			Connection
		</button>
		<button
			type="button"
			role="tab"
			class="tab-button"
			class:active={activeTab === 'connected'}
			aria-selected={activeTab === 'connected'}
			on:click={() => (activeTab = 'connected')}
		>
			Connected Accounts
		</button>
	</div>

	{#if activeTab === 'connection'}
		<div class="card" role="tabpanel">
			<h2>1. Select AWS Deployment Region</h2>
			<div class="quickcreate-region">
				<label for="quickcreate-region">Region</label>
				<select id="quickcreate-region" bind:value={region}>
					<option value="" disabled>Select a region</option>
					{#each usRegions as regionOption}
						<option value={regionOption}>{regionOption}</option>
					{/each}
				</select>
			</div>
		</div>

		{#if region.trim().length > 0}
			<div class="card" role="tabpanel" transition:slide={{ duration: 170 }}>
				<h2>2. Create AssumeRole In AWS</h2>
				<p class="muted">
					After stack creation, copy the <code>CustomerRoleArn</code> value from Outputs.
				</p>
				<div class="actions">
					<button type="button" class="button" on:click={startQuickCreate}>
						Open CloudFormation Quick Create
					</button>
					<a class="ghost" href={templateURL} target="_blank" rel="noreferrer">
						View Template YAML
					</a>
				</div>
				<details class="advanced-settings" bind:open={showCloudFormationAdvanced}>
					<summary>Advanced CloudFormation Parameters</summary>
					<div class="advanced-grid">
						<label for="cf-stack-name">Stack Name</label>
						<input id="cf-stack-name" bind:value={cloudFormationStackName} />
						<label for="cf-platform-principal">Platform Principal ARN</label>
						<input id="cf-platform-principal" bind:value={platformPrincipalArn} />
						<label for="cf-external-id">External ID</label>
						<input id="cf-external-id" bind:value={externalID} />
						<label for="cf-role-name">Role Name</label>
						<input id="cf-role-name" bind:value={cloudFormationRoleName} />
					</div>
				</details>
			</div>

			{#if showSaveStep}
				<div class="card" role="tabpanel" transition:slide={{ duration: 170 }}>
					<h2>3. Save Connection in Labra</h2>
					<p class="muted">
						If you used Quick Create defaults, copy only <code>CustomerRoleArn</code> from AWS.
					</p>
					<div class="aws-form-grid">
						<label for="role-arn">Role ARN</label>
						<input
							id="role-arn"
							bind:value={roleARN}
							placeholder="arn:aws:iam::123456789012:role/LabraCustomerDeployRole"
						/>
					</div>
					<details class="advanced-settings" bind:open={showConnectionAdvanced}>
						<summary>Advanced Connection Options</summary>
						<div class="advanced-grid">
							<label for="save-external-id">External ID</label>
							<input id="save-external-id" bind:value={externalID} placeholder="labra-ext-..." />
						</div>
					</details>
					<div class="form-actions">
						<button on:click={connectAWS} disabled={loading}>
							{loading ? 'Saving...' : 'Validate + Save'}
						</button>
					</div>
				</div>
			{/if}
		{/if}
	{:else}
		<div class="card connected-accounts-card" role="tabpanel">
			<h2>Connected AWS Accounts</h2>
			{#if connections.length === 0}
				<p class="muted">No connections yet.</p>
			{:else}
				<ul class="connection-list">
					{#each connections as c}
						<li>
								<div class="connection-row">
									<strong>{c.role_arn || c.account_id}</strong>
									<div class="connection-meta">{c.region}</div>
									<button
										type="button"
										class="disconnect-button"
									on:click={() => disconnectConnection(c.id)}
									disabled={loading}
								>
									Disconnect
								</button>
							</div>
						</li>
					{/each}
				</ul>
			{/if}
		</div>
	{/if}

	{#if error}<p class="error">{error}</p>{/if}
	{#if success}<p class="success">{success}</p>{/if}
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

	.actions {
		display: flex;
		flex-wrap: wrap;
		gap: 0.75rem;
		margin: 0.66rem 0 0.82rem;
	}

	.card h2 {
		margin-bottom: 0.56rem;
	}

	.actions .button {
		text-decoration: none;
	}

	.ghost {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		padding: 0.58rem 0.9rem;
		border-radius: var(--radius-sm);
		border: 1px solid rgba(183, 189, 248, 0.32);
		background: rgba(30, 32, 48, 0.82);
		text-decoration: none;
		font-weight: 600;
	}

	.quickcreate-region {
		display: grid;
		gap: 0.45rem;
		max-width: 15rem;
	}

	.quickcreate-region label {
		font-size: 0.84rem;
		opacity: 0.9;
	}

	.advanced-settings {
		border: 1px solid rgba(183, 189, 248, 0.24);
		border-radius: var(--radius-sm);
		padding: 0.45rem 0.62rem;
		background: rgba(24, 25, 38, 0.26);
	}

	.advanced-settings summary {
		cursor: pointer;
		display: inline-flex;
		align-items: center;
		font-size: 0.88rem;
		font-weight: 600;
		list-style: none;
	}

	.advanced-settings summary::-webkit-details-marker {
		display: none;
	}

	.advanced-settings summary::before {
		content: '▸';
		display: inline-block;
		margin-right: 0.38rem;
		transition: transform 120ms ease;
	}

	.advanced-settings[open] summary::before {
		transform: rotate(90deg);
	}

	.advanced-grid {
		display: grid;
		gap: 0.5rem;
		margin-top: 0.62rem;
	}

	.advanced-grid label {
		margin-top: 0.1rem;
		font-size: 0.84rem;
		opacity: 0.86;
	}

	.connection-list {
		list-style: none;
		display: grid;
		gap: 0.68rem;
		padding: 0;
		margin: 0;
	}

	.connection-list li {
		padding: 0.7rem 0.8rem;
		border: 1px solid rgba(183, 189, 248, 0.2);
		border-radius: var(--radius-sm);
		background: rgba(24, 25, 38, 0.45);
		transition: transform 170ms ease, border-color 170ms ease, box-shadow 170ms ease;
	}

	.connection-list li:hover {
		transform: translateY(-2px);
		border-color: rgba(138, 173, 244, 0.42);
		box-shadow: var(--shadow-glow);
	}

	.connected-accounts-card:hover {
		transform: none;
		border-color: rgba(183, 189, 248, 0.2);
		box-shadow: var(--shadow-soft);
	}

	.connection-row {
		display: grid;
		gap: 0.46rem;
	}

	.connection-row strong {
		font-size: 1.02rem;
		line-height: 1.35;
	}

	.connection-meta {
		font-size: 0.96rem;
		opacity: 0.86;
	}

	.disconnect-button {
		width: fit-content;
		color: #f5b7c0;
		background: rgba(237, 135, 150, 0.14);
		border-color: rgba(237, 135, 150, 0.42);
		box-shadow: none;
		transition:
			background-color 130ms ease,
			border-color 130ms ease,
			color 130ms ease;
	}

	.disconnect-button:hover {
		transform: none;
		filter: none;
		box-shadow: none;
		background: rgba(237, 135, 150, 0.2);
		border-color: rgba(237, 135, 150, 0.56);
		color: #ffd6dc;
	}

	.form-actions {
		display: flex;
		margin-top: 0.86rem;
	}

	.form-actions button {
		width: fit-content;
	}

	.aws-form-grid {
		display: grid;
		gap: 0.56rem;
	}

	.aws-form-grid label {
		margin-top: 0.25rem;
	}

	.aws-form-grid + .advanced-settings {
		margin-top: 0.68rem;
	}
</style>
