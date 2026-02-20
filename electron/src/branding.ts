import { nativeImage } from "electron";

type MarkVariant = "template" | "running" | "stopped" | "starting" | "app";

const STATUS_COLORS: Record<Exclude<MarkVariant, "template" | "app">, string> = {
  running: "#22c55e",
  stopped: "#ef4444",
  starting: "#f59e0b",
};

function forkPaths(stroke: string, opacity: string = "1", strokeWidth: string = "2.5"): string {
  return [
    `<path d="M3 7H9L17 16" stroke="${stroke}" stroke-opacity="${opacity}" stroke-width="${strokeWidth}" stroke-linecap="round" stroke-linejoin="round"/>`,
    `<path d="M3 16H17" stroke="${stroke}" stroke-opacity="${opacity}" stroke-width="${strokeWidth}" stroke-linecap="round"/>`,
    `<path d="M3 25H9L17 16" stroke="${stroke}" stroke-opacity="${opacity}" stroke-width="${strokeWidth}" stroke-linecap="round" stroke-linejoin="round"/>`,
    `<path d="M17 16H29" stroke="${stroke}" stroke-opacity="${opacity}" stroke-width="${strokeWidth}" stroke-linecap="round"/>`,
    `<circle cx="17" cy="16" r="${Number(strokeWidth) * 1.1}" fill="${stroke}" fill-opacity="${opacity}"/>`,
  ].join("");
}

function svgForVariant(variant: MarkVariant, size: number): string {
  if (variant === "template") {
    // macOS template: thicker strokes for visibility at small sizes, rendered at 2x
    return [
      `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32" width="${size}" height="${size}">`,
      forkPaths("#000", "1", "3.5"),
      "</svg>",
    ].join("");
  }

  const main = variant === "app" ? "#e0e4ef" : STATUS_COLORS[variant];
  return [
    `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32" width="${size}" height="${size}">`,
    `<g transform="translate(1.5 -0.8)">${forkPaths("#2ea4e0", "0.35")}</g>`,
    `<g transform="translate(-0.8 0.8)">${forkPaths("#ef4444", "0.2")}</g>`,
    forkPaths(main),
    "</svg>",
  ].join("");
}

export function createMarkIcon(variant: MarkVariant, size: number = 18): Electron.NativeImage {
  const svg = svgForVariant(variant, size);
  const encoded = Buffer.from(svg).toString("base64");
  return nativeImage.createFromDataURL(`data:image/svg+xml;base64,${encoded}`);
}
