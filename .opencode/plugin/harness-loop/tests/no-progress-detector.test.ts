import { describe, it, expect } from "vitest";
import { latestAssistantTurnMadeNoProgress } from "../no-progress-detector.js";

describe("latestAssistantTurnMadeNoProgress", () => {
  it("returns false when there are no messages at all", () => {
    expect(latestAssistantTurnMadeNoProgress([])).toBe(false);
  });

  it("returns false when there is no assistant message", () => {
    const messages = [{ role: "user", content: "hello" }];
    expect(latestAssistantTurnMadeNoProgress(messages)).toBe(false);
  });

  it("returns true when the last assistant message is empty string", () => {
    const messages = [
      { role: "user", content: "do something" },
      { role: "assistant", content: "" },
    ];
    expect(latestAssistantTurnMadeNoProgress(messages)).toBe(true);
  });

  it("returns true when the last assistant message is whitespace only", () => {
    const messages = [
      { role: "assistant", content: "   \n\t  " },
    ];
    expect(latestAssistantTurnMadeNoProgress(messages)).toBe(true);
  });

  it("returns false when the last assistant message has non-empty content", () => {
    const messages = [
      { role: "user", content: "do something" },
      { role: "assistant", content: "I fixed it." },
    ];
    expect(latestAssistantTurnMadeNoProgress(messages)).toBe(false);
  });

  it("checks the last assistant message even when followed by a user message", () => {
    const messages = [
      { role: "assistant", content: "I tried." },
      { role: "user", content: "not good enough" },
      { role: "assistant", content: "" },
    ];
    expect(latestAssistantTurnMadeNoProgress(messages)).toBe(true);
  });

  it("ignores earlier empty assistant messages if the latest is non-empty", () => {
    const messages = [
      { role: "assistant", content: "" },
      { role: "user", content: "try again" },
      { role: "assistant", content: "Done." },
    ];
    expect(latestAssistantTurnMadeNoProgress(messages)).toBe(false);
  });
});
