<script lang="ts">
	import type { App } from '$lib/api';

	// one shared card so apps list and dashboard render the same shape
	export let app: App;
	export let href = `/apps/${app.id}`;
	export let showBuild = false;
	export let showBranch = true;
</script>

<a class="card app-list-card" href={href}>
	<h2>{app.name}</h2>
	<p><strong>Repo:</strong> {app.repo_full_name}</p>
	{#if showBranch}
		<p><strong>Branch:</strong> {app.branch}</p>
	{/if}
	{#if showBuild}
		<p><strong>Build:</strong> {app.build_type}</p>
	{/if}
</a>

<style>
	.app-list-card {
		display: block;
		text-decoration: none;
		color: inherit;
	}

	.app-list-card h2 {
		margin: 0 0 0.35rem;
	}

	.app-list-card p {
		margin: 0.16rem 0;
	}
</style>
