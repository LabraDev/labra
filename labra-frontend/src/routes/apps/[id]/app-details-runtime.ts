import type { App, Deployment } from '$lib/api';
import { APIError, apiGET, apiPATCH, apiPOST } from '$lib/api';

export type HistoryResponse = {
	app_id: number;
	app_name: string;
	repo: string;
	branch: string;
	deployments: Deployment[];
};

export type InfraOutputResponse = {
	app_id: number;
	outputs: {
		bucket_name: string;
		distribution_id: string;
		site_url: string;
		updated_at: number;
	};
};

type AWSConnection = {
	id: number;
	account_id: string;
	status: string;
};

export type AppDetailsBundle = {
	app: App;
	history: HistoryResponse;
	infraOutputs: InfraOutputResponse;
	deployAccountID: string;
};

export async function fetchAppDetailsBundle(appID: number): Promise<AppDetailsBundle> {
	// grab all page data in one place so the route file stays focused on ui stuff
	const app = await apiGET<App>(`/v1/apps/${appID}`);
	const history = await apiGET<HistoryResponse>(`/v1/apps/${appID}/deploys`);
	// keep newest deploy first even if backend ordering changes later
	history.deployments = [...(history.deployments ?? [])].sort((a, b) => {
		if (b.updated_at !== a.updated_at) return b.updated_at - a.updated_at;
		return b.id - a.id;
	});

	const infraOutputs = await apiGET<InfraOutputResponse>(`/v1/apps/${appID}/infra-outputs`);
	const awsConnections = await apiGET<{ aws_connections: AWSConnection[] }>('/v1/aws-connections');
	// pick the validated account so the page can show what account deploys are tied to
	const selectedConnection = (awsConnections.aws_connections ?? []).find(
		(connection) => connection.status?.toLowerCase() === 'validated'
	);

	return {
		app,
		history,
		infraOutputs,
		deployAccountID: selectedConnection?.account_id ?? ''
	};
}

export async function queueDeployment(appID: number): Promise<number> {
	// return plain id so callers can decide how to message it
	const response = await apiPOST<{ deployment: { id: number } }>(`/v1/apps/${appID}/deploy`, {});
	return response.deployment?.id ?? 0;
}

export async function updateAutoDeploy(app: App, enabled: boolean): Promise<App> {
	// backend returns the updated app shape so caller can replace local state
	return apiPATCH<App>(`/v1/apps/${app.id}`, {
		auto_deploy_enabled: enabled
	});
}

export async function appNoLongerExists(appID: number): Promise<boolean> {
	try {
		await apiGET<App>(`/v1/apps/${appID}`);
		return false;
	} catch (err) {
		if (err instanceof APIError && err.status === 404) {
			return true;
		}
	}

	// fallback for short eventual consistency windows after deletes
	try {
		const appsResponse = await apiGET<{ apps: App[] }>('/v1/apps');
		return !(appsResponse.apps ?? []).some((candidate) => candidate.id === appID);
	} catch {
		return false;
	}
}

type PollerDeps = {
	intervalMS: number;
	shouldPoll: () => boolean;
	onTick: () => Promise<void>;
	onActiveChange?: (active: boolean) => void;
};

export function createAppDetailsPoller(deps: PollerDeps): {
	sync: () => void;
	stop: () => void;
	destroy: () => void;
} {
	// one timer max so we never stack intervals
	let timer: number | null = null;
	// avoid overlapping refresh calls if network is slow
	let tickInFlight = false;

	function setActive(active: boolean) {
		deps.onActiveChange?.(active);
	}

	function stop() {
		// safe to call a lot this is our universal cleanup path
		if (timer !== null && typeof window !== 'undefined') {
			window.clearInterval(timer);
		}
		timer = null;
		setActive(false);
	}

	async function tick() {
		if (tickInFlight) return;
		tickInFlight = true;
		try {
			await deps.onTick();
		} finally {
			tickInFlight = false;
		}
	}

	function sync() {
		if (typeof window === 'undefined') return;
		// polling only runs when caller says we are in a pending state
		if (!deps.shouldPoll()) {
			stop();
			return;
		}

		setActive(true);
		if (timer !== null) return;
		// use the caller interval so route can tune it per screen
		timer = window.setInterval(() => {
			void tick();
		}, deps.intervalMS);
	}

	function destroy() {
		stop();
	}

	return {
		sync,
		stop,
		destroy
	};
}
