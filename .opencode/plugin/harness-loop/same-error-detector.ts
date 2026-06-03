import { SAME_ERROR_TRIP_THRESHOLD } from "./constants.js";
import type { HarnessLoopState } from "./types.js";

function arraysEqual(a: string[], b: string[]): boolean {
  if (a.length !== b.length) return false;
  const sortedA = [...a].sort();
  const sortedB = [...b].sort();
  return sortedA.every((val, idx) => val === sortedB[idx]);
}

export function hasRepeatedSameError(
  state: HarnessLoopState,
  currentGate: string
): boolean {
  const history = state.loop.same_error_history[currentGate];

  if (!history || history.length < SAME_ERROR_TRIP_THRESHOLD) {
    return false;
  }

  const lastN = history.slice(-SAME_ERROR_TRIP_THRESHOLD);

  if (lastN.some((entry) => entry.length === 0)) {
    return false;
  }

  const first = lastN[0];
  if (!first) return false;

  return lastN.every((entry) => arraysEqual(entry, first));
}
