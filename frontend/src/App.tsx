import type { Backend } from "@/backend"

export interface AppProps {
  backend: Backend
}

export function App({ backend: _backend }: AppProps) {
  return (
    <main className="min-h-screen bg-slate-950 text-slate-100 p-6">
      <h1 className="text-xl font-semibold">Rainforest</h1>
    </main>
  )
}
