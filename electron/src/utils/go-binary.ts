import { app } from "electron";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";

export function getGoBinaryPath(): string {
  const ext = process.platform === "win32" ? ".exe" : "";

  if (!app.isPackaged) {
    // Development: binary is at <project-root>/bin/mcplexer
    // app.getAppPath() returns <project-root>/electron
    return path.join(app.getAppPath(), "..", "bin", `mcplexer${ext}`);
  }

  // Production: binary is bundled in app resources
  return path.join(process.resourcesPath, "bin", `mcplexer${ext}`);
}

/**
 * Returns the stable binary path (~/.mcplexer/bin/mcplexer), copying the
 * bundled binary there if it is missing or outdated (by mtime comparison).
 */
export function getStableBinaryPath(): string {
  const ext = process.platform === "win32" ? ".exe" : "";
  const stablePath = path.join(os.homedir(), ".mcplexer", "bin", `mcplexer${ext}`);
  const bundledPath = getGoBinaryPath();

  try {
    const bundledStat = fs.statSync(bundledPath);
    let needsCopy = true;

    try {
      const stableStat = fs.statSync(stablePath);
      needsCopy = bundledStat.mtimeMs > stableStat.mtimeMs;
    } catch {
      // Stable binary doesn't exist yet
    }

    if (needsCopy) {
      fs.mkdirSync(path.dirname(stablePath), { recursive: true });
      fs.copyFileSync(bundledPath, stablePath);
      fs.chmodSync(stablePath, 0o755);
    }
  } catch (err) {
    console.error("[mcplexer] Failed to install stable binary:", err);
    // Fall back to bundled path if copy fails
    return bundledPath;
  }

  return stablePath;
}
