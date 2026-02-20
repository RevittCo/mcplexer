import net from "node:net";

// MCPlexer uses a fixed port range (13333–13343) so the Electron app
// can reliably locate the backend without user configuration.
const PORT_RANGE_START = 13333;
const PORT_RANGE_END = 13343;

function tryPort(port: number): Promise<boolean> {
  return new Promise((resolve) => {
    const server = net.createServer();

    server.once("error", () => {
      resolve(false);
    });

    server.once("listening", () => {
      server.close(() => {
        resolve(true);
      });
    });

    server.listen(port, "127.0.0.1");
  });
}

export async function findFreePort(): Promise<number> {
  for (let port = PORT_RANGE_START; port <= PORT_RANGE_END; port++) {
    const available = await tryPort(port);
    if (available) {
      return port;
    }
  }

  throw new Error(
    `No free port found in range ${PORT_RANGE_START}-${PORT_RANGE_END}`
  );
}
