import { defineConfig } from "mygo-cli"

export default defineConfig({
  name: "Rainforest",
  id: "com.thedavidweng.rainforest",
  version: "0.1.0",
  main: ".",
  devCommand: "bun --cwd frontend run dev",
  devUrl: "http://localhost:5173",
  buildCommand: "bun --cwd frontend run build",
  frontendDist: "frontend/dist",
  bindings: "frontend/src/mygo.ts",
})
