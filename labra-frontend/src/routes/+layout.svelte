<script lang="ts">
	import { browser } from '$app/environment';
	import { goto } from '$app/navigation';
	import { page } from '$app/stores';
	import { onMount } from 'svelte';
	import favicon from '$lib/assets/favicon.svg';
	import { getSessionToken } from '$lib/api';
	import Header from '$lib/components/header.svelte';

	const publicPaths = new Set(['/', '/login']);
	let isAuthenticated = false;

	function syncAuthState() {
		isAuthenticated = getSessionToken().trim().length > 0;
	}

	onMount(() => {
		syncAuthState();
		window.addEventListener('storage', syncAuthState);
		return () => window.removeEventListener('storage', syncAuthState);
	});

	$: if (browser) {
		syncAuthState();
		const path = $page.url.pathname;
		if (!publicPaths.has(path) && !isAuthenticated) {
			void goto('/');
		}
	}
</script>

<svelte:head>
	<link rel="icon" href={favicon} />
</svelte:head>

<div class="app-shell" class:with-header={isAuthenticated}>
	{#if isAuthenticated}
		<Header />
	{/if}
	<!-- svelte-ignore slot_element_deprecated -->
	<main><slot /></main>
</div>

<style>
	@import url('https://fonts.googleapis.com/css2?family=Sora:wght@400;600;700&family=Space+Grotesk:wght@400;500;600;700&display=swap');

	:root {
		/* Catppuccin Macchiato core */
		--rosewater: #f4dbd6;
		--flamingo: #f0c6c6;
		--pink: #f5bde6;
		--mauve: #c6a0f6;
		--red: #ed8796;
		--maroon: #ee99a0;
		--peach: #f5a97f;
		--yellow: #eed49f;
		--green: #a6da95;
		--teal: #8bd5ca;
		--sky: #91d7e3;
		--sapphire: #7dc4e4;
		--blue: #8aadf4;
		--lavender: #b7bdf8;
		--text-color: #cad3f5;
		--subtext: #b8c0e0;
		--overlay: #6e738d;
		--hr-color: #494d64;
		--surface0: #363a4f;
		--surface1: #494d64;
		--surface2: #5b6078;
		--bg-color: #24273a;
		--mantle: #1e2030;
		--crust: #181926;
		--ink: #090a0f;
		--radius-sm: 10px;
			--radius-md: 16px;
			--radius-lg: 22px;
			--shadow-soft: 0 12px 28px rgba(0, 0, 0, 0.34);
			--shadow-glow: 0 0 0 1px rgba(138, 173, 244, 0.2), 0 16px 34px rgba(138, 173, 244, 0.14);
			--space-1: 0.35rem;
			--space-2: 0.55rem;
			--space-3: 0.8rem;
			--space-4: 1rem;
			--space-5: 1.25rem;
			--space-6: 1.6rem;
		}

	:global(*) {
		box-sizing: border-box;
	}

	:global(html),
	:global(body) {
		margin: 0;
		padding: 0;
		min-height: 100%;
		background:
			radial-gradient(1200px 460px at 8% -20%, rgba(198, 160, 246, 0.22), transparent 62%),
			radial-gradient(1100px 440px at 95% -28%, rgba(138, 173, 244, 0.18), transparent 66%),
			radial-gradient(900px 360px at 50% 120%, rgba(139, 213, 202, 0.14), transparent 72%),
			var(--ink);
		color: var(--text-color);
		font-family: 'Space Grotesk', 'Sora', 'Avenir Next', 'Trebuchet MS', sans-serif;
		line-height: 1.45;
	}

	.app-shell {
		--header-offset: 0px;
		min-height: 100dvh;
		display: flex;
		flex-direction: column;
	}

	.app-shell.with-header {
		--header-offset: 86px;
	}

	@media (max-width: 940px) {
		.app-shell.with-header {
			--header-offset: 126px;
		}
	}

	main {
		flex: 1 1 auto;
		min-height: 0;
		background: linear-gradient(180deg, rgba(24, 25, 38, 0.74), rgba(24, 25, 38, 0.9));
		color: var(--text-color);
		position: relative;
		overflow: hidden;
	}

	main::before {
		content: '';
		position: absolute;
		inset: 0;
		pointer-events: none;
		background-image:
			linear-gradient(to right, rgba(202, 211, 245, 0.04) 1px, transparent 1px),
			linear-gradient(to bottom, rgba(202, 211, 245, 0.04) 1px, transparent 1px);
		background-size: 36px 36px;
		opacity: 0.11;
		mask-image: radial-gradient(circle at center, black 28%, transparent 92%);
	}

	:global(h1),
	:global(h2),
	:global(h3) {
		font-family: 'Sora', 'Space Grotesk', sans-serif;
		letter-spacing: 0.01em;
	}

		:global(h1) {
			font-size: clamp(1.75rem, 2.2vw, 2.45rem);
			margin: 0 0 var(--space-2);
		}

		:global(h2) {
			font-size: clamp(1.1rem, 1.4vw, 1.45rem);
			margin: 0 0 var(--space-2);
		}

	:global(p),
	:global(li),
	:global(td),
	:global(th),
	:global(label) {
		color: var(--subtext);
	}

	:global(strong) {
		color: var(--rosewater);
	}

	:global(a) {
		color: var(--lavender);
		transition: color 160ms ease;
	}

	:global(a:hover) {
		color: var(--sky);
	}

		:global(.page) {
			position: relative;
			z-index: 1;
			max-width: 1120px;
			margin: 0 auto;
			padding: 2.35rem 1.2rem 3rem;
			display: grid;
			gap: var(--space-5);
			animation: fade-rise 440ms cubic-bezier(0.18, 0.88, 0.22, 1.05);
		}

		:global(.toolbar) {
			display: flex;
			justify-content: space-between;
			align-items: end;
			flex-wrap: wrap;
			gap: var(--space-4);
			margin-bottom: var(--space-1);
		}

	:global(.controls) {
		display: flex;
		gap: 0.7rem;
		align-items: end;
		flex-wrap: wrap;
	}

		:global(.cards) {
			display: grid;
			grid-template-columns: repeat(auto-fill, minmax(250px, 1fr));
			gap: var(--space-4);
		}

		:global(.summary-grid) {
			display: grid;
			grid-template-columns: repeat(auto-fit, minmax(255px, 1fr));
			gap: var(--space-4);
		}

		:global(.card),
		:global(article) {
		background:
			linear-gradient(155deg, rgba(73, 77, 100, 0.34), rgba(36, 39, 58, 0.78) 42%, rgba(24, 25, 38, 0.95));
		border: 1px solid rgba(183, 189, 248, 0.2);
		border-radius: var(--radius-md);
			padding: 1.15rem;
			backdrop-filter: blur(6px);
			box-shadow: var(--shadow-soft);
			transition: transform 170ms ease, border-color 170ms ease, box-shadow 170ms ease;
		}

		:global(.page > .tabs) {
			margin-bottom: 0.1rem;
		}

		:global(.card h2),
		:global(article h2) {
			margin: 0 0 var(--space-2);
		}

		:global(.card > :is(p, ul, ol, table):last-child),
		:global(article > :is(p, ul, ol, table):last-child) {
			margin-bottom: 0;
		}

	:global(.card:hover),
	:global(article:hover) {
		transform: translateY(-2px);
		border-color: rgba(138, 173, 244, 0.44);
		box-shadow: var(--shadow-glow);
	}

	:global(.cards .card) {
		animation: float-in 460ms cubic-bezier(0.17, 0.92, 0.2, 1.04);
	}

	:global(label) {
		display: grid;
		gap: 0.35rem;
		font-size: 0.88rem;
		font-weight: 500;
	}

	:global(input),
	:global(textarea),
	:global(select) {
		background: rgba(24, 25, 38, 0.84);
		color: var(--text-color);
		border: 1px solid rgba(183, 189, 248, 0.25);
		border-radius: var(--radius-sm);
		padding: 0.58rem 0.7rem;
		font-family: inherit;
		font-size: 0.94rem;
		transition: border-color 140ms ease, box-shadow 140ms ease, background-color 140ms ease;
	}

	:global(input:focus),
	:global(textarea:focus),
	:global(select:focus) {
		outline: none;
		border-color: rgba(138, 173, 244, 0.75);
		box-shadow: 0 0 0 3px rgba(138, 173, 244, 0.15);
		background: rgba(30, 32, 48, 0.96);
	}

	:global(button),
	:global(a.button) {
		border: 1px solid rgba(138, 173, 244, 0.42);
		border-radius: var(--radius-sm);
		padding: 0.58rem 0.9rem;
		font-family: inherit;
		font-weight: 600;
		font-size: 0.92rem;
		cursor: pointer;
		color: var(--crust);
		background: linear-gradient(130deg, var(--blue), var(--lavender));
		box-shadow: 0 8px 20px rgba(138, 173, 244, 0.22);
		transition: transform 130ms ease, filter 130ms ease, box-shadow 130ms ease;
	}

	:global(button:hover),
	:global(a.button:hover) {
		transform: translateY(-1px);
		filter: brightness(1.06);
		box-shadow: 0 11px 22px rgba(138, 173, 244, 0.3);
	}

	:global(button:disabled) {
		cursor: not-allowed;
		opacity: 0.56;
		transform: none;
	}

	:global(.secondary) {
		background: linear-gradient(130deg, var(--teal), var(--sapphire));
		border-color: rgba(139, 213, 202, 0.5);
	}

	:global(.muted) {
		opacity: 0.78;
	}

	:global(.error) {
		color: var(--red);
	}

	:global(.ok),
	:global(.success) {
		color: var(--green);
	}

	:global(.back) {
		display: inline-block;
		margin-bottom: 0.38rem;
		text-decoration: none;
		color: var(--lavender);
	}

	:global(table) {
		width: 100%;
		border-collapse: collapse;
		border-radius: var(--radius-md);
		overflow: hidden;
		border: 1px solid rgba(183, 189, 248, 0.22);
		background: rgba(30, 32, 48, 0.84);
	}

	:global(th),
	:global(td) {
		text-align: left;
		padding: 0.62rem;
		border-bottom: 1px solid rgba(183, 189, 248, 0.18);
	}

	:global(th) {
		color: var(--rosewater);
		background: rgba(73, 77, 100, 0.42);
		font-weight: 600;
	}

	:global(.logs),
	:global(.config-history) {
		list-style: none;
		padding: 0;
		margin: 0;
		border: 1px solid rgba(183, 189, 248, 0.2);
		border-radius: var(--radius-md);
		background: rgba(30, 32, 48, 0.74);
	}

	:global(.logs li),
	:global(.config-history li) {
		padding: 0.62rem 0.78rem;
		border-bottom: 1px solid rgba(183, 189, 248, 0.16);
	}

	:global(.logs li:last-child),
	:global(.config-history li:last-child) {
		border-bottom: 0;
	}

	:global(.stamp) {
		opacity: 0.72;
	}

	:global(.level) {
		color: var(--sky);
		min-width: 52px;
		font-weight: 600;
	}

	@keyframes fade-rise {
		0% {
			opacity: 0;
			transform: translateY(10px);
		}
		100% {
			opacity: 1;
			transform: translateY(0);
		}
	}

	@keyframes float-in {
		0% {
			opacity: 0;
			transform: translateY(12px) scale(0.985);
		}
		100% {
			opacity: 1;
			transform: translateY(0) scale(1);
		}
	}

		@media (max-width: 760px) {
			:global(.page) {
				padding: 1.7rem 0.95rem 2.25rem;
				gap: var(--space-4);
			}

		:global(.toolbar) {
			align-items: start;
		}

		:global(.controls) {
			width: 100%;
		}

		:global(.controls > *) {
			flex: 1 1 auto;
		}

		:global(button),
		:global(a.button) {
			width: 100%;
		}
	}
</style>
