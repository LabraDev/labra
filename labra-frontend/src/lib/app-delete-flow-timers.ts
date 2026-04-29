type TimerCallbacks = {
	onProgressTick: () => void;
	onExistencePollTick: () => void;
	onCountdownTick: () => void;
	onCompletionFXTimeout: () => void;
};

export function createDeleteFlowTimers(callbacks: TimerCallbacks): {
	startProgressAndExistence: (progressIntervalMS: number, existencePollIntervalMS: number) => void;
	startCountdown: (intervalMS: number) => void;
	startCompletionFX: (durationMS: number) => void;
	stopProgressAndExistence: () => void;
	stopCountdown: () => void;
	stopAll: () => void;
	destroy: () => void;
} {
	let progressTimer: number | null = null;
	let existencePollTimer: number | null = null;
	let countdownTimer: number | null = null;
	let completionFXTimer: number | null = null;

	function stopProgressAndExistence() {
		if (typeof window === 'undefined') return;
		if (progressTimer !== null) window.clearInterval(progressTimer);
		if (existencePollTimer !== null) window.clearInterval(existencePollTimer);
		progressTimer = null;
		existencePollTimer = null;
	}

	function stopCountdown() {
		if (typeof window === 'undefined') return;
		if (countdownTimer !== null) window.clearInterval(countdownTimer);
		countdownTimer = null;
	}

	function stopAll() {
		stopProgressAndExistence();
		stopCountdown();
	}

	function startProgressAndExistence(progressIntervalMS: number, existencePollIntervalMS: number) {
		stopProgressAndExistence();
		if (typeof window === 'undefined') return;

		progressTimer = window.setInterval(() => {
			callbacks.onProgressTick();
		}, progressIntervalMS);
		existencePollTimer = window.setInterval(() => {
			callbacks.onExistencePollTick();
		}, existencePollIntervalMS);
	}

	function startCountdown(intervalMS: number) {
		stopCountdown();
		if (typeof window === 'undefined') return;

		countdownTimer = window.setInterval(() => {
			callbacks.onCountdownTick();
		}, intervalMS);
	}

	function startCompletionFX(durationMS: number) {
		if (typeof window === 'undefined') return;
		if (completionFXTimer !== null) {
			window.clearTimeout(completionFXTimer);
		}
		completionFXTimer = window.setTimeout(() => {
			completionFXTimer = null;
			callbacks.onCompletionFXTimeout();
		}, durationMS);
	}

	function destroy() {
		stopAll();
		if (typeof window === 'undefined') return;
		if (completionFXTimer !== null) {
			window.clearTimeout(completionFXTimer);
		}
		completionFXTimer = null;
	}

	return {
		startProgressAndExistence,
		startCountdown,
		startCompletionFX,
		stopProgressAndExistence,
		stopCountdown,
		stopAll,
		destroy
	};
}
