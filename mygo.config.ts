export default {
  name: "Rainforest",
  id: "com.thedavidweng.rainforest",
  version: "0.1.0",
  main: ".",
  devCommand: "bun run --cwd frontend dev",
  devUrl: "http://localhost:5173",
  buildCommand: "bun run --cwd frontend build",
  frontendDist: "frontend/dist",
  bindings: "frontend/src/mygo.ts",
}
