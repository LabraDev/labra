import { writable, type Readable } from 'svelte/store';
import type { AppDeleteSession } from '$lib/app-delete-session';

export type DeleteFlowState = {
	inProgress: boolean;
	progress: number;
	label: string;
	successNotice: string;
	redirectCountdown: number;
	activeSession: AppDeleteSession | null;
	completionFXActive: boolean;
};

export const initialDeleteFlowState: DeleteFlowState = {
	inProgress: false,
	progress: 0,
	label: '',
	successNotice: '',
	redirectCountdown: 0,
	activeSession: null,
	completionFXActive: false
};

export function createDeleteFlowStateStore(): {
	state: Readable<DeleteFlowState>;
	getSnapshot: () => DeleteFlowState;
	setState: (partial: Partial<DeleteFlowState>) => void;
} {
	const state = writable<DeleteFlowState>(initialDeleteFlowState);

	function getSnapshot(): DeleteFlowState {
		let snapshot = initialDeleteFlowState;
		const unsub = state.subscribe((value) => {
			snapshot = value;
		});
		unsub();
		return snapshot;
	}

	function setState(partial: Partial<DeleteFlowState>) {
		state.update((prev) => ({ ...prev, ...partial }));
	}

	return {
		state,
		getSnapshot,
		setState
	};
}
