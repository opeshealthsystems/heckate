// The PDF preview renders pages in the browser with pdf.js; its worker must be served from our own origin
// (the Content-Security-Policy allows no other script source). Copied before dev and build, not committed.
import { copyFileSync } from "node:fs";
copyFileSync("node_modules/pdfjs-dist/build/pdf.worker.min.mjs", "public/pdf.worker.min.mjs");
