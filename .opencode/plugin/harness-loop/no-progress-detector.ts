export interface AssistantMessage {
  role: string;
  content: string;
}

export function latestAssistantTurnMadeNoProgress(
  messages: AssistantMessage[]
): boolean {
  for (let i = messages.length - 1; i >= 0; i--) {
    const msg = messages[i];
    if (msg?.role === "assistant") {
      const content = msg.content.trim();
      return content.length === 0;
    }
  }

  return false;
}
