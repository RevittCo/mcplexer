import { Notification } from "electron";
import { getMainWindow, getServerUrl } from "./main.js";

const INITIAL_DELAY_MS = 1000;
const MAX_DELAY_MS = 30000;
const BACKOFF_FACTOR = 2;

let abortController: AbortController | null = null;
let reconnectTimer: ReturnType<typeof setTimeout> | null = null;
let currentDelay = INITIAL_DELAY_MS;

interface Approval {
  id: string;
  tool_name: string;
}

interface ApprovalEvent {
  type: "pending" | "resolved";
  approval: Approval;
}

function parseSSEEvents(chunk: string): ApprovalEvent[] {
  const events: ApprovalEvent[] = [];
  const blocks = chunk.split("\n\n");

  for (const block of blocks) {
    const lines = block.trim().split("\n");
    let data = "";

    for (const line of lines) {
      if (line.startsWith("data:")) {
        data += line.slice(5).trim();
      }
    }

    if (data.length === 0) {
      continue;
    }

    try {
      const parsed = JSON.parse(data) as ApprovalEvent;
      if (parsed.type && parsed.approval) {
        events.push(parsed);
      }
    } catch {
      // Skip malformed SSE data
    }
  }

  return events;
}

function showApprovalNotification(approval: Approval): void {
  const notification = new Notification({
    title: "MCPlexer: Approval Required",
    body: approval.tool_name,
  });

  notification.on("click", () => {
    const win = getMainWindow();
    if (win) {
      win.show();
      win.focus();
      void win.loadURL(`${getServerUrl()}/approvals`);
    }
  });

  notification.show();
}

async function connectSSE(): Promise<void> {
  const controller = new AbortController();
  abortController = controller;

  const streamUrl = `${getServerUrl()}/api/v1/approvals/stream`;
  console.log(`[mcplexer:notifications] Connecting to SSE: ${streamUrl}`);

  try {
    const response = await fetch(streamUrl, {
      method: "GET",
      headers: { Accept: "text/event-stream" },
      signal: controller.signal,
    });

    if (!response.ok) {
      throw new Error(`SSE response status: ${response.status}`);
    }

    if (!response.body) {
      throw new Error("SSE response has no body");
    }

    // Reset backoff on successful connection
    currentDelay = INITIAL_DELAY_MS;
    console.log("[mcplexer:notifications] SSE connected");

    const reader = response.body.getReader();
    const decoder = new TextDecoder();
    let buffer = "";

    while (true) {
      const { done, value } = await reader.read();

      if (done) {
        console.log("[mcplexer:notifications] SSE stream ended");
        break;
      }

      buffer += decoder.decode(value, { stream: true });

      // Process complete events (delimited by double newlines)
      const lastDoubleNewline = buffer.lastIndexOf("\n\n");
      if (lastDoubleNewline === -1) {
        continue;
      }

      const complete = buffer.slice(0, lastDoubleNewline + 2);
      buffer = buffer.slice(lastDoubleNewline + 2);

      const events = parseSSEEvents(complete);
      for (const event of events) {
        if (event.type === "pending") {
          showApprovalNotification(event.approval);
        }
      }
    }
  } catch (err: unknown) {
    if (controller.signal.aborted) {
      console.log("[mcplexer:notifications] SSE connection aborted");
      return;
    }

    const message = err instanceof Error ? err.message : String(err);
    console.error(
      `[mcplexer:notifications] SSE error: ${message}`
    );
  }

  // Schedule reconnection unless stopped
  scheduleReconnect();
}

function scheduleReconnect(): void {
  if (abortController === null) {
    // stopApprovalListener was called — do not reconnect
    return;
  }

  console.log(
    `[mcplexer:notifications] Reconnecting in ${currentDelay}ms...`
  );
  reconnectTimer = setTimeout(() => {
    reconnectTimer = null;
    void connectSSE();
  }, currentDelay);

  currentDelay = Math.min(currentDelay * BACKOFF_FACTOR, MAX_DELAY_MS);
}

export function startApprovalListener(): void {
  if (abortController !== null) {
    return;
  }

  currentDelay = INITIAL_DELAY_MS;
  void connectSSE();
}

export function stopApprovalListener(): void {
  if (reconnectTimer !== null) {
    clearTimeout(reconnectTimer);
    reconnectTimer = null;
  }

  if (abortController !== null) {
    abortController.abort();
    abortController = null;
  }

  console.log("[mcplexer:notifications] Listener stopped");
}
