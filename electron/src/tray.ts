import { app, Menu, Tray } from "electron";
import { getMainWindow } from "./main.js";
import { createMarkIcon } from "./branding.js";

type TrayStatus = "running" | "stopped" | "starting";

let tray: Tray | null = null;

function getTrayIcon(status: TrayStatus): Electron.NativeImage {
  if (process.platform === "darwin") {
    const icon = createMarkIcon("template", 22);
    icon.setTemplateImage(true);
    return icon;
  }

  return createMarkIcon(status, 22);
}

function buildContextMenu(): Electron.Menu {
  return Menu.buildFromTemplate([
    {
      label: "Show Window",
      click: () => {
        const win = getMainWindow();
        win?.show();
        win?.focus();
      },
    },
    {
      label: "Restart Server",
      click: () => {
        console.log("[mcplexer:tray] Restart not implemented");
      },
    },
    { type: "separator" },
    {
      label: "Quit",
      click: () => {
        app.quit();
      },
    },
  ]);
}

function tooltipForStatus(status: TrayStatus): string {
  const labels: Record<TrayStatus, string> = {
    running: "MCPlexer \u2014 Running",
    stopped: "MCPlexer \u2014 Stopped",
    starting: "MCPlexer \u2014 Starting",
  };
  return labels[status];
}

export function initTray(): void {
  if (tray !== null) {
    return;
  }

  const icon = getTrayIcon("stopped");
  tray = new Tray(icon);
  tray.setToolTip(tooltipForStatus("stopped"));
  tray.setContextMenu(buildContextMenu());

  tray.on("click", () => {
    const win = getMainWindow();
    win?.show();
    win?.focus();
  });
}

export function setTrayStatus(status: TrayStatus): void {
  if (tray === null) {
    return;
  }
  tray.setImage(getTrayIcon(status));
  tray.setToolTip(tooltipForStatus(status));
}
