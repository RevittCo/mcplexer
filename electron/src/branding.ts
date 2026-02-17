import { nativeImage } from "electron";

type MarkVariant = "template" | "running" | "stopped" | "starting" | "app";

const STATUS_COLORS: Record<Exclude<MarkVariant, "template" | "app">, string> = {
  running: "#22c55e",
  stopped: "#ef4444",
  starting: "#f59e0b",
};

function mPath(): string {
  return "M8 56 V8 H20 L32 28 L44 8 H56 V56 H44 V29 L32 48 L20 29 V56 Z";
}

function svgForVariant(variant: MarkVariant, size: number): string {
  if (variant === "template") {
    return [
      `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 64 64" width="${size}" height="${size}">`,
      `<path d="${mPath()}" fill="#000"/>`,
      "</svg>",
    ].join("");
  }

  const main = variant === "app" ? "#e0e4ef" : STATUS_COLORS[variant];
  return [
    `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 64 64" width="${size}" height="${size}">`,
    `<path d="${mPath()}" fill="#2ea4e0" opacity="0.45" transform="translate(2 -1)"/>`,
    `<path d="${mPath()}" fill="#ef4444" opacity="0.25" transform="translate(-1 1)"/>`,
    `<path d="${mPath()}" fill="${main}"/>`,
    "</svg>",
  ].join("");
}

export function createMarkIcon(variant: MarkVariant, size: number = 18): Electron.NativeImage {
  const svg = svgForVariant(variant, size);
  const encoded = Buffer.from(svg).toString("base64");
  return nativeImage.createFromDataURL(`data:image/svg+xml;base64,${encoded}`);
}
