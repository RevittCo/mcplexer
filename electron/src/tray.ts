import { app, Menu, nativeImage, Tray } from "electron";
import path from "node:path";
import { getMainWindow } from "./main.js";
import { createMarkIcon } from "./branding.js";

type TrayStatus = "running" | "stopped" | "starting";

let tray: Tray | null = null;

function trayAssetsDir(): string {
  if (!app.isPackaged) {
    return path.join(app.getAppPath(), "assets", "tray");
  }
  return path.join(process.resourcesPath, "tray");
}

function getTrayIcon(_status: TrayStatus): Electron.NativeImage {
  if (process.platform === "darwin") {
    // Load pre-rendered blue multiplexer icon PNGs (matches website favicon)
    const dir = trayAssetsDir();
    const img1x = path.join(dir, "trayIcon.png");
    const img2x = path.join(dir, "trayIcon@2x.png");

    // Build multi-resolution image: 1x for standard, 2x for Retina
    const icon = nativeImage.createEmpty();
    icon.addRepresentation({ scaleFactor: 1.0, dataURL: nativeImage.createFromPath(img1x).toDataURL() });
    icon.addRepresentation({ scaleFactor: 2.0, dataURL: nativeImage.createFromPath(img2x).toDataURL() });
    return icon;
  }

  return createMarkIcon(_status, 22);
}

function buildContextMenu(): Electron.Menu {
  const loginSettings = app.getLoginItemSettings();

  return Menu.buildFromTemplate([
    {
      label: "Show Window",
      click: () => {
        const win = getMainWindow();
        win?.show();
        win?.focus();
      },
    },
    { type: "separator" },
    {
      label: "Open at Login",
      type: "checkbox",
      checked: loginSettings.openAtLogin,
      click: (menuItem) => {
        app.setLoginItemSettings({
          openAtLogin: menuItem.checked,
          openAsHidden: true,
        });
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

  // Rebuild menu on right-click to update checkbox state
  tray.on("right-click", () => {
    tray?.setContextMenu(buildContextMenu());
  });
}

export function setTrayStatus(status: TrayStatus): void {
  if (tray === null) {
    return;
  }
  tray.setImage(getTrayIcon(status));
  tray.setToolTip(tooltipForStatus(status));
}
