import react from "@vitejs/plugin-react";
import { defineConfig } from "vite";

// The build output is committed and embedded into the Go binary by assets.go,
// so `go build` and goreleaser never need node. Regenerate with `make web-build`.
export default defineConfig({
  plugins: [react()],
  build: {
    outDir: "dist",
    emptyOutDir: true,
  },
  server: {
    // `npm --prefix web run dev` serves the UI while `wataridori serve`
    // handles RPC on :8080.
    proxy: {
      "/wataridori.v1.DeploymentService": "http://localhost:8080",
    },
  },
});
