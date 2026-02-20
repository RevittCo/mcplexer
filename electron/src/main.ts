import { app, BrowserWindow } from "electron";
import { ChildProcess, spawn } from "node:child_process";
import os from "node:os";
import path from "node:path";
import { findFreePort } from "./utils/port-finder.js";
import { getGoBinaryPath } from "./utils/go-binary.js";
import { waitForHealth } from "./utils/health-check.js";
import { ensureMCPClientsConfigured } from "./utils/mcp-clients.js";
import { initTray, setTrayStatus } from "./tray.js";
import { startApprovalListener, stopApprovalListener } from "./notifications.js";
import { createMarkIcon } from "./branding.js";

let mainWindow: BrowserWindow | null = null;
let goProcess: ChildProcess | null = null;
let serverPort: number | null = null;
let isQuitting = false;

export function getMainWindow(): BrowserWindow | null {
  return mainWindow;
}

export function getServerUrl(): string {
  if (serverPort === null) {
    throw new Error("Server port not yet assigned");
  }
  return `http://localhost:${serverPort}`;
}

function createWindow(): void {
  const launchedHidden = app.getLoginItemSettings().wasOpenedAsHidden;

  mainWindow = new BrowserWindow({
    width: 1280,
    height: 800,
    minWidth: 900,
    minHeight: 600,
    webPreferences: {
      preload: path.join(__dirname, "preload.js"),
      contextIsolation: true,
      nodeIntegration: false,
    },
    show: false,
    title: "MCPlexer",
    icon: createMarkIcon("app", 256),
  });

  mainWindow.once("ready-to-show", () => {
    if (!launchedHidden) {
      mainWindow?.show();
    }
  });

  mainWindow.on("close", (event) => {
    if (process.platform === "darwin" && !isQuitting) {
      event.preventDefault();
      mainWindow?.hide();
    }
  });

  mainWindow.on("closed", () => {
    mainWindow = null;
  });
}

function spawnGoServer(port: number): ChildProcess {
  const binaryPath = getGoBinaryPath();
  const socketPath = path.join(os.tmpdir(), "mcplexer.sock");
  console.log(`[mcplexer] Starting Go server: ${binaryPath} serve --mode=http --addr=:${port} --socket=${socketPath}`);

  const child = spawn(binaryPath, ["serve", "--mode=http", `--addr=:${port}`, `--socket=${socketPath}`], {
    stdio: "pipe",
    env: { ...process.env },
  });

  child.stdout?.on("data", (data: Buffer) => {
    const lines = data.toString().trim();
    if (lines) {
      console.log(`[mcplexer:stdout] ${lines}`);
    }
  });

  child.stderr?.on("data", (data: Buffer) => {
    const lines = data.toString().trim();
    if (lines) {
      console.error(`[mcplexer:stderr] ${lines}`);
    }
  });

  child.on("error", (err: Error) => {
    console.error(`[mcplexer] Failed to start Go server: ${err.message}`);
  });

  child.on("exit", (code: number | null, signal: string | null) => {
    console.log(`[mcplexer] Go server exited (code=${code}, signal=${signal})`);
    goProcess = null;

    if (!isQuitting) {
      setTrayStatus("stopped");
      stopApprovalListener();
      console.error("[mcplexer] Go server terminated unexpectedly");
      app.quit();
    }
  });

  return child;
}

async function shutdownGoProcess(): Promise<void> {
  setTrayStatus("stopped");
  stopApprovalListener();

  if (!goProcess || goProcess.exitCode !== null) {
    return;
  }

  console.log("[mcplexer] Sending SIGTERM to Go server...");
  goProcess.kill("SIGTERM");

  const waitTimeout = 5000;
  const exitPromise = new Promise<boolean>((resolve) => {
    const timer = setTimeout(() => resolve(false), waitTimeout);
    goProcess?.on("exit", () => {
      clearTimeout(timer);
      resolve(true);
    });
  });

  const exitedGracefully = await exitPromise;

  if (!exitedGracefully && goProcess && goProcess.exitCode === null) {
    console.log("[mcplexer] Go server did not exit in 5s, sending SIGKILL...");
    goProcess.kill("SIGKILL");
  }
}

async function startApp(): Promise<void> {
  try {
    initTray();
    setTrayStatus("starting");

    serverPort = await findFreePort();
    console.log(`[mcplexer] Using port ${serverPort}`);

    goProcess = spawnGoServer(serverPort);

    const serverUrl = `http://localhost:${serverPort}`;
    console.log(`[mcplexer] Waiting for health check at ${serverUrl}/api/v1/health`);
    await waitForHealth(serverUrl);
    console.log("[mcplexer] Go server is healthy");

    setTrayStatus("running");

    // On first launch, configure Claude Desktop and enable login item
    try {
      ensureMCPClientsConfigured();
      app.setLoginItemSettings({ openAtLogin: true, openAsHidden: true });
    } catch (err) {
      console.error("[mcplexer] First-launch setup error:", err);
    }

    createWindow();

    if (mainWindow) {
      await mainWindow.loadURL(serverUrl);
      startApprovalListener();
    }
  } catch (err) {
    console.error("[mcplexer] Failed to start:", err);
    await shutdownGoProcess();
    app.quit();
  }
}

app.on("ready", () => {
  if (process.platform === "darwin" && app.dock) {
    app.dock.setIcon(createMarkIcon("app", 512));
  }
  void startApp();
});

app.on("activate", () => {
  if (mainWindow === null) {
    void startApp();
  } else {
    mainWindow.show();
  }
});

app.on("window-all-closed", () => {
  if (process.platform !== "darwin") {
    app.quit();
  }
});

app.on("before-quit", () => {
  isQuitting = true;
  void shutdownGoProcess();
});
