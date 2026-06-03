import type { Plugin, PluginInput, Hooks } from "@opencode-ai/plugin";
import { stateExists, getStatePath, readState } from "./storage.js";
import {
  handleSessionIdle,
  handleSessionError,
  handleSessionDeleted,
  type PluginContext,
} from "./harness-loop-event-handler.js";
import {
  handleHarnessOn,
  type HarnessOnContext,
} from "./commands/harness-on.js";
import { handleHarnessOff, type HarnessOffContext } from "./commands/harness-off.js";
import type { HarnessLoopState } from "./types.js";

const PLUGIN_VERSION = "0.1.0";
let versionLogged = false;

async function fetchMessages(
  input: PluginInput,
  sessionID: string
): Promise<Array<{ role: string; content: string }>> {
  const result = await input.client.session.messages({ path: { id: sessionID } });
  const data = result.data;
  if (!data) return [];
  return data.map((m) => ({
    role: m.info.role,
    content: m.parts
      .filter((p) => p.type === "text")
      .map((p) => ("text" in p ? (p as { text: string }).text : ""))
      .join(""),
  }));
}

function buildPluginContext(
  input: PluginInput,
  sessionID: string,
  messages: Array<{ role: string; content: string }>
): PluginContext {
  return {
    sessionId: sessionID,
    projectRoot: input.directory,
    getMessages: () => messages,
    injectMessage: async (text: string) => {
      await input.client.session.prompt({
        path: { id: sessionID },
        body: { parts: [{ type: "text", text }] },
      });
    },
    showToast: (message: string, variant: "info" | "warning" | "error") => {
      void input.client.tui.showToast({
        body: { message, variant },
      });
    },
    hasActiveBackgroundTasks: () => false,
    latestUserMessageTimestamp: () => 0,
  };
}

function buildHarnessOnContext(
  input: PluginInput,
  sessionID: string,
  messages: Array<{ role: string; content: string }>
): HarnessOnContext {
  return {
    projectRoot: input.directory,
    sessionId: sessionID,
    getMessageCount: () => messages.length,
    injectMessage: async (text: string) => {
      await input.client.session.prompt({
        path: { id: sessionID },
        body: { parts: [{ type: "text", text }] },
      });
    },
    showToast: (message: string, variant: "info" | "warning" | "error") => {
      void input.client.tui.showToast({
        body: { message, variant },
      });
    },
    askQuestion: async (_options) => "abort",
  };
}

function buildHarnessOffContext(
  input: PluginInput
): HarnessOffContext {
  return {
    projectRoot: input.directory,
    showToast: (message: string, variant: "info" | "warning" | "error") => {
      void input.client.tui.showToast({
        body: { message, variant },
      });
    },
  };
}

const HarnessLoopPlugin: Plugin = async (input: PluginInput): Promise<Hooks> => {
  if (!versionLogged) {
    console.log(`[harness-loop] Plugin loaded v${PLUGIN_VERSION}`);
    versionLogged = true;
  }

  return {
    async event({ event }) {
      if (event.type === "session.idle") {
        const sessionID = event.properties.sessionID;
        const statePath = getStatePath(input.directory);
        if (!stateExists(statePath)) return;
        const state = readState(statePath) as HarnessLoopState | null;
        if (!state?.loop.active) return;

        const messages = await fetchMessages(input, sessionID);
        const ctx = buildPluginContext(input, sessionID, messages);
        await handleSessionIdle(ctx);
        return;
      }

      if (event.type === "session.error") {
        const sessionID = event.properties.sessionID ?? "";
        if (!sessionID) return;
        const statePath = getStatePath(input.directory);
        if (!stateExists(statePath)) return;
        const state = readState(statePath) as HarnessLoopState | null;
        if (!state?.loop.active) return;

        const messages = await fetchMessages(input, sessionID);
        const ctx = buildPluginContext(input, sessionID, messages);
        const rawError = event.properties.error;
        const err = new Error(
          rawError && "message" in rawError ? String(rawError.message) : "session error"
        );
        await handleSessionError(ctx, err);
        return;
      }

      if (event.type === "session.deleted") {
        const deletedID = event.properties.info.id;
        const statePath = getStatePath(input.directory);
        if (!stateExists(statePath)) return;
        const ctx = buildPluginContext(input, deletedID, []);
        await handleSessionDeleted(ctx, deletedID);
      }
    },

    async "command.execute.before"(cmdInput, _output) {
      const { command, sessionID, arguments: argsStr } = cmdInput;

      if (command === "harness-on") {
        const args = argsStr ? argsStr.trim().split(/\s+/) : [];
        const messages = await fetchMessages(input, sessionID);
        const ctx = buildHarnessOnContext(input, sessionID, messages);
        await handleHarnessOn(ctx, args);
        return;
      }

      if (command === "harness-off") {
        const ctx = buildHarnessOffContext(input);
        await handleHarnessOff(ctx);
      }
    },
  };
};

export default HarnessLoopPlugin;
