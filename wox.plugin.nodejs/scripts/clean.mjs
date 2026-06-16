import { rmSync } from "fs";
import path from "path";
import { fileURLToPath } from "url";

const packageRoot = fileURLToPath(new URL("..", import.meta.url));
const distPath = path.join(packageRoot, "dist");

// Node 26 rejects the old rmdirSync({ recursive: true }) cleanup path.
// rmSync is the supported recursive deletion API, and force keeps clean idempotent when dist has not been generated yet.
rmSync(distPath, { recursive: true, force: true });
