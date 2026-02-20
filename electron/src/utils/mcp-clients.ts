import path from "node:path";
import fs from "node:fs";
import os from "node:os";
import { getGoBinaryPath } from "./go-binary.js";

interface MCPClient {
  name: string;
  configPath: () => string;
}

const knownClients: MCPClient[] = [
  {
    name: "Claude Desktop",
    configPath: () => {
      const home = os.homedir();
      switch (process.platform) {
        case "darwin":
          return path.join(home, "Library", "Application Support", "Claude", "claude_desktop_config.json");
        case "linux":
          return path.join(home, ".config", "Claude", "claude_desktop_config.json");
        default:
          return "";
      }
    },
  },
  {
    name: "Claude Code",
    configPath: () => path.join(os.homedir(), ".claude", "settings.json"),
  },
  {
    name: "Cursor",
    configPath: () => path.join(os.homedir(), ".cursor", "mcp.json"),
  },
  {
    name: "Windsurf",
    configPath: () => path.join(os.homedir(), ".codeium", "windsurf", "mcp_config.json"),
  },
  {
    name: "Codex",
    configPath: () => path.join(os.homedir(), ".codex", "mcp.json"),
  },
  {
    name: "OpenCode",
    configPath: () => path.join(os.homedir(), ".opencode", "mcp.json"),
  },
  {
    name: "Gemini CLI",
    configPath: () => path.join(os.homedir(), ".gemini", "settings.json"),
  },
];

function clientInstalled(client: MCPClient): boolean {
  const p = client.configPath();
  if (!p) return false;
  try {
    fs.statSync(path.dirname(p));
    return true;
  } catch {
    return false;
  }
}

function isConfigured(configPath: string): boolean {
  try {
    const data = fs.readFileSync(configPath, "utf-8");
    const cfg = JSON.parse(data) as Record<string, unknown>;
    const servers = cfg.mcpServers as Record<string, unknown> | undefined;
    return servers !== undefined && "mx" in servers;
  } catch {
    return false;
  }
}

function writeConfig(configPath: string): void {
  let cfg: Record<string, unknown> = {};
  try {
    const data = fs.readFileSync(configPath, "utf-8");
    cfg = JSON.parse(data) as Record<string, unknown>;
  } catch {
    // Start fresh
  }

  const servers = (cfg.mcpServers as Record<string, unknown>) ?? {};
  servers.mx = {
    command: getGoBinaryPath(),
    args: ["connect", `--socket=${path.join(os.tmpdir(), "mcplexer.sock")}`],
  };
  cfg.mcpServers = servers;

  fs.mkdirSync(path.dirname(configPath), { recursive: true });
  fs.writeFileSync(configPath, JSON.stringify(cfg, null, 2) + "\n", "utf-8");
}

export function ensureMCPClientsConfigured(): void {
  for (const client of knownClients) {
    if (!clientInstalled(client)) continue;

    const configPath = client.configPath();
    if (isConfigured(configPath)) {
      console.log(`[mcplexer] ${client.name} already configured`);
      continue;
    }

    try {
      writeConfig(configPath);
      console.log(`[mcplexer] ${client.name} configured`);
    } catch (err) {
      console.error(`[mcplexer] Failed to configure ${client.name}:`, err);
    }
  }
}
