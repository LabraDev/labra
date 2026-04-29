import type { Readable } from 'svelte/store';
import type { AppDeleteSession, AppDeleteSessionPhase } from '$lib/app-delete-session';
import { readDeleteFlowSessionForApp, persistDeleteFlowSession } from '$lib/app-delete-flow-persistence';
import { createDeleteFlowStateStore, type DeleteFlowState } from '$lib/app-delete-flow-state';
import { createDeleteFlowTimers } from '$lib/app-delete-flow-timers';

type DeleteFlowDeps = {
	appID: number;
	getAppName: () => string;
	checkDeleted: (appID: number) => Promise<boolean>;
	onRedirectToApps: () => Promise<void>;
};

const deleteVerifyPollIntervalMS = 2500;
const deleteVerifyTimeoutMS = 3 * 60 * 1000;
const deleteRedirectDelaySeconds = 15;
const deleteRequestStepMS = 2500;
const deleteInfraStepMS = 3000;
const deleteFinalizeStepMS = 7000;
const deleteVerifyStepMS = 20000;
const completionFXDurationMS = 5600;
const progressTickIntervalMS = 850;

export function createAppDeleteFlow(deps: DeleteFlowDeps): {
	state: Readable<DeleteFlowState>;
	restoreIfNeeded: () => Promise<void>;
	startDeleteProgress: (phase?: AppDeleteSessionPhase, startedAt?: number) => void;
	markVerifyingFromTimeout: () => void;
	waitForDeletion: (appID: number, timeoutMS?: number) => Promise<boolean>;
	completeAndRedirect: () => Promise<void>;
	handleFailureAfterDeleteRequest: () => void;
	clear: () => void;
	destroy: () => void;
	getSnapshot: () => DeleteFlowState;
} {
	const { state, getSnapshot, setState } = createDeleteFlowStateStore();

	const timers = createDeleteFlowTimers({
		onProgressTick: () => {
			const liveSession = getSnapshot().activeSession;
			if (!liveSession) return;
			updateProgressFromSession(liveSession);
		},
		onExistencePollTick: () => {
			void checkDeletionCompletionInBackground();
		},
		onCountdownTick: () => {
			setState({ redirectCountdown: Math.max(0, getSnapshot().redirectCountdown - 1) });
		},
		onCompletionFXTimeout: () => {
			setState({ completionFXActive: false });
		}
	});

	function setActiveSession(next: AppDeleteSession | null) {
		setState({ activeSession: next });
		persistDeleteFlowSession(next);
	}

	function triggerCompletionFX() {
		if (typeof window === 'undefined') return;
		setState({ completionFXActive: true });
		timers.startCompletionFX(completionFXDurationMS);
	}

	function persistProgress(progress: number, label: string) {
		const nextProgress = Math.max(0, Math.min(100, Math.round(progress)));
		setState({ progress: nextProgress, label });

		const current = getSnapshot().activeSession;
		if (!current) return;
		setActiveSession({
			...current,
			progress: nextProgress,
			progressUpdatedAt: Date.now(),
			updatedAt: Date.now()
		});
	}

	function maybeAdvanceDeleteProgress(
		session: AppDeleteSession,
		ceiling: number,
		step: number,
		stepMS: number,
		label: string
	): void {
		const fallbackProgress = getSnapshot().progress;
		const currentProgress = Number.isFinite(session.progress) ? Number(session.progress) : fallbackProgress;
		if (currentProgress >= ceiling) {
			setState({ label });
			return;
		}
		const now = Date.now();
		const lastProgressAt = session.progressUpdatedAt ?? session.updatedAt ?? session.startedAt;
		if (now - lastProgressAt < stepMS) {
			setState({ label });
			return;
		}
		persistProgress(Math.min(ceiling, currentProgress + step), label);
	}

	function updateProgressFromSession(session: AppDeleteSession) {
		const fallbackProgress = getSnapshot().progress;
		const currentProgress = Number.isFinite(session.progress) ? Number(session.progress) : fallbackProgress;
		if (session.phase === 'complete') {
			persistProgress(100, 'Deletion complete.');
			return;
		}
		if (session.phase === 'verifying') {
			const phaseSince = session.phaseSince ?? session.updatedAt ?? session.startedAt;
			const elapsedSeconds = Math.max(0, Math.floor((Date.now() - phaseSince) / 1000));
			maybeAdvanceDeleteProgress(
				session,
				99,
				1,
				deleteVerifyStepMS,
				`Delete request timed out. Verifying teardown completion... (${elapsedSeconds}s)`
			);
			return;
		}
		if (currentProgress < 28) {
			maybeAdvanceDeleteProgress(session, 28, 2, deleteRequestStepMS, 'Submitting delete request...');
			return;
		}
		if (currentProgress < 72) {
			maybeAdvanceDeleteProgress(session, 72, 1, deleteInfraStepMS, 'Deleting AWS infrastructure...');
			return;
		}
		if (currentProgress < 94) {
			maybeAdvanceDeleteProgress(session, 94, 1, deleteFinalizeStepMS, 'Finalizing cleanup...');
			return;
		}
		setState({ label: 'Finalizing cleanup...' });
	}

	function startDeleteProgress(phase: AppDeleteSessionPhase = 'request', startedAt = Date.now()) {
		timers.stopProgressAndExistence();
		const current = getSnapshot().activeSession;
		const sessionProgress =
			current && Number.isFinite(current.progress) ? Number(current.progress) : phase === 'complete' ? 100 : 6;

		setState({
			inProgress: true,
			progress: sessionProgress,
			label: phase === 'complete' ? 'Deletion complete.' : 'Starting deletion...',
			successNotice: '',
			redirectCountdown: 0
		});

		if (!current) {
			setActiveSession({
				appID: deps.appID,
				appName: deps.getAppName(),
				startedAt,
				updatedAt: Date.now(),
				progressUpdatedAt: Date.now(),
				phase,
				progress: sessionProgress,
				phaseSince: Date.now()
			});
		} else {
			setActiveSession({
				...current,
				appID: deps.appID,
				appName: deps.getAppName() || current.appName,
				startedAt,
				updatedAt: Date.now(),
				progressUpdatedAt: current.phase === phase ? current.progressUpdatedAt ?? Date.now() : Date.now(),
				phase,
				progress: sessionProgress,
				phaseSince: current.phase === phase ? current.phaseSince ?? Date.now() : Date.now()
			});
		}

		const nextSession = getSnapshot().activeSession;
		if (nextSession) {
			updateProgressFromSession(nextSession);
		}

		timers.startProgressAndExistence(progressTickIntervalMS, deleteVerifyPollIntervalMS);
	}

	function completeDeleteProgress() {
		timers.stopProgressAndExistence();
		setState({
			inProgress: true,
			progress: 100,
			label: 'Deletion complete.'
		});
		const current = getSnapshot().activeSession;
		if (!current) return;
		setActiveSession({
			...current,
			phase: 'complete',
			progress: 100,
			updatedAt: Date.now(),
			progressUpdatedAt: Date.now(),
			phaseSince: Date.now(),
			completedAt: Date.now()
		});
		triggerCompletionFX();
	}

	async function beginRedirectCountdown(seconds: number) {
		setState({ redirectCountdown: seconds });
		timers.startCountdown(1000);

		await new Promise<void>((resolve) => {
			window.setTimeout(resolve, seconds * 1000);
		});

		timers.stopAll();
		setActiveSession(null);
		setState({ inProgress: false });
		await deps.onRedirectToApps();
	}

	async function completeAndRedirect() {
		completeDeleteProgress();
		setState({
			label: 'Deletion complete. Redirecting to Existing Apps...',
			successNotice: 'Successful deletion! You will be redirected to Apps page.'
		});
		await beginRedirectCountdown(deleteRedirectDelaySeconds);
	}

	function markVerifyingFromTimeout() {
		const current = getSnapshot().activeSession;
		if (!current) return;
		setActiveSession({
			...current,
			phase: 'verifying',
			progress: Number.isFinite(current.progress) ? current.progress : getSnapshot().progress,
			phaseSince: Date.now(),
			updatedAt: Date.now()
		});
		setState({ label: 'Delete request timed out. Verifying teardown completion...' });
	}

	async function waitForDeletion(appID: number, timeoutMS = deleteVerifyTimeoutMS): Promise<boolean> {
		const started = Date.now();
		while (Date.now() - started < timeoutMS) {
			const deleted = await deps.checkDeleted(appID);
			if (deleted) {
				return true;
			}

			const elapsedSeconds = Math.floor((Date.now() - started) / 1000);
			setState({
				label: `Delete request timed out. Verifying teardown completion... (${elapsedSeconds}s)`
			});
			const current = getSnapshot().activeSession;
			if (current) {
				setActiveSession({
					...current,
					phase: 'verifying',
					progress: Number.isFinite(current.progress) ? current.progress : getSnapshot().progress,
					phaseSince: current.phase === 'verifying' ? current.phaseSince ?? Date.now() : Date.now(),
					updatedAt: Date.now()
				});
			}

			await new Promise<void>((resolve) => {
				window.setTimeout(resolve, deleteVerifyPollIntervalMS);
			});
		}
		return false;
	}

	async function checkDeletionCompletionInBackground() {
		const snapshot = getSnapshot();
		if (!snapshot.activeSession || snapshot.activeSession.phase === 'complete') return;
		if (snapshot.redirectCountdown > 0) return;

		const deleted = await deps.checkDeleted(deps.appID);
		if (!deleted) return;

		await completeAndRedirect();
	}

	function handleFailureAfterDeleteRequest() {
		const snapshot = getSnapshot();
		if (snapshot.activeSession?.phase === 'verifying') {
			setState({ inProgress: true });
			return;
		}
		timers.stopAll();
		setActiveSession(null);
		setState({
			inProgress: false,
			progress: 0,
			label: '',
			successNotice: '',
			redirectCountdown: 0
		});
	}

	async function restoreIfNeeded() {
		if (typeof window === 'undefined') return;
		const saved = readDeleteFlowSessionForApp(deps.appID);
		if (!saved) return;

		setActiveSession(saved);
		setState({ inProgress: true });
		startDeleteProgress(saved.phase, saved.startedAt);

		const deleted = await deps.checkDeleted(deps.appID);
		if (deleted) {
			await completeAndRedirect();
			return;
		}
		if (saved.phase === 'verifying') {
			const done = await waitForDeletion(deps.appID);
			if (done) {
				await completeAndRedirect();
			}
		}
	}

	function clear() {
		timers.stopAll();
		setActiveSession(null);
		setState({
			inProgress: false,
			progress: 0,
			label: '',
			successNotice: '',
			redirectCountdown: 0,
			completionFXActive: false
		});
	}

	function destroy() {
		timers.destroy();
	}

	return {
		state,
		restoreIfNeeded,
		startDeleteProgress,
		markVerifyingFromTimeout,
		waitForDeletion,
		completeAndRedirect,
		handleFailureAfterDeleteRequest,
		clear,
		destroy,
		getSnapshot
	};
}
