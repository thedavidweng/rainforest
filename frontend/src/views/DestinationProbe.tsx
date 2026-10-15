import { useEffect, useMemo, useState } from "react"

import type { AuthStatus, Backend, DestinationCapabilityDTO } from "@/backend"
import { errorMessageFrom } from "@/backend"

export function DestinationProbe({
  backend,
  auth,
  id,
}: {
  backend: Backend
  auth: AuthStatus | null
  id: string
}) {
  const [revision, setRevision] = useState(0)
  const request = useMemo(() => ({ backend, auth, id, revision }), [backend, auth, id, revision])
  const [response, setResponse] = useState<{
    request: typeof request
    probe?: DestinationCapabilityDTO
    error?: string
  } | null>(null)
  const probe = response?.request === request ? response.probe : null
  const error = response?.request === request ? response.error : ""
  useEffect(() => {
    let live = true
    backend.transfers.probeDestination(id).then(
      (result) => {
        if (live) setResponse({ request, probe: result })
      },
      (err) => {
        if (live) setResponse({ request, error: errorMessageFrom(err) })
      },
    )
    return () => {
      live = false
    }
  }, [backend, id, request])
  return (
    <div className="text-xs text-slate-300 space-y-1">
      <p>
        Destination: {probe?.name || id || "Account root"} · {probe?.kind} ·{" "}
        {error ? "failed" : probe?.status || "pending"}
      </p>
      <p>{error || probe?.message}</p>
      <button
        type="button"
        className="px-2 py-1 rounded bg-slate-800"
        onClick={() => setRevision(revision + 1)}
      >
        Check Destination
      </button>
    </div>
  )
}
