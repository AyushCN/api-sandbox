import { useState, useEffect } from "react"
import toast from "react-hot-toast"
import { Loader2, Settings, Save, Trash2, Plus, Moon, Clock, ShieldAlert, CheckCircle2, AlertTriangle, ShieldCheck } from "lucide-react"

export default function EnvironmentSettings({ env, mutate, isViewerRole }: { env: Record<string, unknown>, mutate: () => void, isViewerRole: boolean }) {
  const [startCommand, setStartCommand] = useState((env.startCommand as string) || "")
  const [rootDirectory, setRootDirectory] = useState((env.rootDirectory as string) || "")
  const [dockerfilePath, setDockerfilePath] = useState((env.dockerfilePath as string) || "")
  const [preDeployCommand, setPreDeployCommand] = useState((env.preDeployCommand as string) || "")
  const [runtimeType, setRuntimeType] = useState((env.runtimeType as string) || "")
  const [port, setPort] = useState((env.port as number)?.toString() || "")
  const [healthCheckType, setHealthCheckType] = useState((env.healthCheckType as string) || "http")
  const [envVars, setEnvVars] = useState<{key: string, value: string}[]>(() => {
    if (!env.envVars || !Array.isArray(env.envVars)) return []
    return env.envVars.map((v: any) => ({ key: v.key, value: v.value }))
  })
  const [isSaving, setIsSaving] = useState(false)

  // Lifecycle & Timer state
  const status = ((env.status as string) || "IDLE").toUpperCase()
  const idleTimeoutSec = Number(env.idle_timeout_seconds || env.idleTimeoutSeconds || 900)
  const expiresAtStr = (env.expires_at || env.expiresAt) as string | undefined
  const lastActiveStr = (env.last_request_at || env.lastRequestAt || env.last_activity_at || env.updatedAt || env.createdAt) as string | undefined

  const [idleTimeout, setIdleTimeout] = useState(idleTimeoutSec.toString())
  const [maxLifetime, setMaxLifetime] = useState(String(env.lifetime_seconds || env.lifetimeSeconds || 3600))
  const [now, setNow] = useState(Date.now())

  useEffect(() => {
    const timer = setInterval(() => setNow(Date.now()), 1000)
    return () => clearInterval(timer)
  }, [])

  // Compute hibernation countdown
  let hibernationText = "Active"
  if (status === "HIBERNATED") {
    hibernationText = "Hibernated (Inactive)"
  } else if (status === "EXPIRED" || status === "DELETED") {
    hibernationText = "Inactive (Torn Down)"
  } else if (status === "RUNNING") {
    const lastActive = lastActiveStr ? new Date(lastActiveStr).getTime() : now
    const targetHibernate = lastActive + idleTimeoutSec * 1000
    const rem = Math.max(0, Math.floor((targetHibernate - now) / 1000))
    const mins = Math.floor(rem / 60)
    const secs = rem % 60
    hibernationText = `${mins}m ${secs.toString().padStart(2, "0")}s`
  } else {
    hibernationText = status
  }

  // Compute expiration countdown
  let expirationText = "No Expiration"
  if (status === "EXPIRED" || status === "DELETED") {
    expirationText = "Expired (Torn Down)"
  } else if (expiresAtStr) {
    const targetExpire = new Date(expiresAtStr).getTime()
    const rem = Math.max(0, Math.floor((targetExpire - now) / 1000))
    if (rem <= 0) {
      expirationText = "Expired"
    } else {
      const days = Math.floor(rem / 86400)
      const hours = Math.floor((rem % 86400) / 3600)
      const mins = Math.floor((rem % 3600) / 60)
      const secs = rem % 60
      if (days > 0) {
        expirationText = `${days}d ${hours}h ${mins}m`
      } else {
        expirationText = `${hours.toString().padStart(2, "0")}:${mins.toString().padStart(2, "0")}:${secs.toString().padStart(2, "0")}`
      }
    }
  }

  const handleSave = async () => {
    setIsSaving(true)
    try {
      const res = await fetch(`/api/environments/${env.id}/settings`, {
        method: "PUT",
        headers: {
          "Authorization": `Bearer ${localStorage.getItem("token")}`,
          "Content-Type": "application/json"
        },
        body: JSON.stringify({
          startCommand: startCommand || null,
          rootDirectory: rootDirectory || null,
          dockerfilePath: dockerfilePath || null,
          preDeployCommand: preDeployCommand || null,
          runtimeType: runtimeType || null,
          port: port ? parseInt(port) : null,
          healthCheckType,
          idleTimeoutSeconds: parseInt(idleTimeout),
          lifetimeSeconds: parseInt(maxLifetime),
          envVars: envVars.reduce((acc, curr) => {
            if (curr.key.trim()) acc[curr.key.trim()] = curr.value
            return acc
          }, {} as Record<string, string>)
        })
      })
      const data = await res.json()
      if (res.ok) {
        toast.success(data.message || "Settings updated!")
        mutate()
      } else {
        throw new Error(data.error || "Failed to update settings")
      }
    } catch(err) {
      if (err instanceof Error) {
        toast.error((err as Error).message)
      } else {
        toast.error("An unknown error occurred")
      }
    } finally {
      setIsSaving(false)
    }
  }

  return (
    <div className="bg-surface-container-lowest border border-outline-variant rounded-xl p-6 max-w-3xl">
      <div className="flex items-center gap-2 mb-6">
        <Settings className="w-5 h-5 text-on-surface-variant/70" />
        <h3 className="font-semibold text-lg">Environment Settings</h3>
      </div>
      
      <div className="space-y-6">
        {/* Sandbox Lifecycle & Security Policy Card */}
        <div className="border border-outline-variant/60 rounded-xl bg-surface-container/30 p-5 space-y-5">
          <div className="flex items-center justify-between pb-3 border-b border-outline-variant/40">
            <div className="flex items-center gap-2">
              <Clock className="w-5 h-5 text-primary-fixed" />
              <div>
                <h4 className="text-sm font-bold text-on-surface">Lifecycle & Security Policy</h4>
                <p className="text-xs text-on-surface-variant/70">Server-enforced idle timeouts, auto-hibernation, and maximum lifetime caps.</p>
              </div>
            </div>
            <span className="text-[11px] font-mono px-2.5 py-1 rounded-full border border-outline-variant bg-surface-container-high text-on-surface-variant font-semibold">
              Status: {status}
            </span>
          </div>

          {/* Live Dual Countdowns */}
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
            {/* Hibernation Timer */}
            <div className="p-4 rounded-xl bg-surface-container/50 border border-outline-variant/50 space-y-1.5">
              <div className="flex items-center justify-between text-xs text-on-surface-variant">
                <span className="flex items-center gap-1.5 font-semibold text-amber-500">
                  <Moon className="w-4 h-4" />
                  Time Until Hibernation
                </span>
                <span className="text-[10px] uppercase font-bold tracking-wider px-2 py-0.5 rounded bg-amber-500/10 text-amber-500 border border-amber-500/20">
                  {status === "HIBERNATED" ? "Hibernated" : "Inactivity Timer"}
                </span>
              </div>
              <div className="text-2xl font-mono font-bold text-on-surface tracking-tight">
                {hibernationText}
              </div>
              <p className="text-[11px] text-on-surface-variant/70">
                {status === "HIBERNATED" 
                  ? "Container is paused on disk. Requests to the preview URL automatically wake it."
                  : `Hibernates if no HTTP traffic arrives for ${Math.floor(parseInt(idleTimeout) / 60)} minutes.`}
              </p>
            </div>

            {/* Expiration Timer */}
            <div className="p-4 rounded-xl bg-surface-container/50 border border-outline-variant/50 space-y-1.5">
              <div className="flex items-center justify-between text-xs text-on-surface-variant">
                <span className="flex items-center gap-1.5 font-semibold text-red-400">
                  <Clock className="w-4 h-4" />
                  Time Until Expiration
                </span>
                <span className="text-[10px] uppercase font-bold tracking-wider px-2 py-0.5 rounded bg-red-400/10 text-red-400 border border-red-400/20">
                  Hard Cap (TTL)
                </span>
              </div>
              <div className="text-2xl font-mono font-bold text-on-surface tracking-tight">
                {expirationText}
              </div>
              <p className="text-[11px] text-on-surface-variant/70">
                Irreversible shutdown. Once expired, the container is permanently torn down.
              </p>
            </div>
          </div>

          {/* Policy Configuration Controls */}
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-4 pt-2">
            <div>
              <label className="block text-xs font-semibold text-on-surface-variant mb-1">
                Idle Inactivity Timeout
              </label>
              <select
                value={idleTimeout}
                onChange={(e) => setIdleTimeout(e.target.value)}
                disabled={isViewerRole}
                className="w-full px-3 py-2 text-xs bg-surface-container/60 border border-outline-variant rounded-lg focus:border-primary-fixed focus:outline-none disabled:opacity-50 text-on-surface cursor-pointer"
              >
                <option value="300">5 minutes</option>
                <option value="600">10 minutes</option>
                <option value="900">15 minutes (Default)</option>
                <option value="1800">30 minutes</option>
                <option value="3600">1 hour</option>
                <option value="7200">2 hours</option>
              </select>
            </div>

            <div>
              <label className="block text-xs font-semibold text-on-surface-variant mb-1">
                Max Sandbox Lifetime
              </label>
              <select
                value={maxLifetime}
                onChange={(e) => setMaxLifetime(e.target.value)}
                disabled={isViewerRole}
                className="w-full px-3 py-2 text-xs bg-surface-container/60 border border-outline-variant rounded-lg focus:border-primary-fixed focus:outline-none disabled:opacity-50 text-on-surface cursor-pointer"
              >
                <option value="3600">1 hour</option>
                <option value="21600">6 hours</option>
                <option value="43200">12 hours</option>
                <option value="86400">24 hours (1 day)</option>
                <option value="604800">7 days (Platform Hard Cap)</option>
              </select>
            </div>
          </div>

          <div className="flex items-center gap-2 p-3 rounded-lg bg-surface-container/40 border border-outline-variant/30 text-[11px] text-on-surface-variant">
            <ShieldAlert className="w-4 h-4 text-primary-fixed shrink-0" />
            <span>
              {isViewerRole 
                ? "Viewer permissions: Lifecycle configurations are read-only. Contact the project Owner to modify."
                : "Security policy: Health checks and automated probes do not reset the inactivity timer. Only genuine developer traffic keeps sandboxes alive."}
            </span>
          </div>
        </div>
        <div>
          <label className="block text-sm font-medium text-on-surface-variant mb-1">Runtime Type</label>
          <select
            value={runtimeType}
            onChange={e => setRuntimeType(e.target.value)}
            disabled={isViewerRole}
            className="w-full px-3 py-2 bg-surface-container/50 border border-outline-variant rounded-lg focus:border-primary-fixed focus:ring-1 focus:ring-primary-fixed outline-none transition-all disabled:opacity-50"
          >
            <option value="">Auto-detect</option>
            <option value="node">Node.js</option>
            <option value="python">Python</option>
            <option value="go">Go</option>
            <option value="docker">Docker Build</option>
          </select>
          <p className="text-xs text-on-surface-variant/70 mt-1">If Auto-detect fails, specify the correct language runtime.</p>
        </div>

        <div>
          <label className="block text-sm font-medium text-on-surface-variant mb-1">Root Directory <span className="text-on-surface-variant/50 font-normal">(Optional)</span></label>
          <input 
            type="text" 
            value={rootDirectory}
            onChange={e => setRootDirectory(e.target.value)}
            disabled={isViewerRole}
            placeholder="e.g. backend"
            className="w-full px-3 py-2 bg-surface-container/50 border border-outline-variant rounded-lg focus:border-primary-fixed focus:ring-1 focus:ring-primary-fixed outline-none transition-all disabled:opacity-50"
          />
          <p className="text-xs text-on-surface-variant/70 mt-1">If set, commands run from this directory instead of the repository root.</p>
        </div>

        {runtimeType === 'docker' && (
          <div>
            <label className="block text-sm font-medium text-on-surface-variant mb-1">Dockerfile Path</label>
            <input 
              type="text" 
              value={dockerfilePath}
              onChange={e => setDockerfilePath(e.target.value)}
              disabled={isViewerRole}
              placeholder="e.g. ./Dockerfile"
              className="w-full px-3 py-2 bg-surface-container/50 border border-outline-variant rounded-lg focus:border-primary-fixed focus:ring-1 focus:ring-primary-fixed outline-none transition-all disabled:opacity-50"
            />
            <p className="text-xs text-on-surface-variant/70 mt-1">Path to your Dockerfile, relative to the repo root. Defaults to ./Dockerfile.</p>
          </div>
        )}

        <div>
          <label className="block text-sm font-medium text-on-surface-variant mb-1">Pre-Deploy Command</label>
          <input 
            type="text" 
            value={preDeployCommand}
            onChange={e => setPreDeployCommand(e.target.value)}
            disabled={isViewerRole}
            placeholder="e.g. npm run build"
            className="w-full px-3 py-2 bg-surface-container/50 border border-outline-variant rounded-lg focus:border-primary-fixed focus:ring-1 focus:ring-primary-fixed outline-none transition-all disabled:opacity-50"
          />
          <p className="text-xs text-on-surface-variant/70 mt-1">Runs before the start command. Useful for database migrations or build steps.</p>
        </div>

        <div>
          <label className="block text-sm font-medium text-on-surface-variant mb-1">Start Command Override</label>
          <input 
            type="text" 
            value={startCommand}
            onChange={e => setStartCommand(e.target.value)}
            disabled={isViewerRole}
            placeholder="e.g. npm run dev"
            className="w-full px-3 py-2 bg-surface-container/50 border border-outline-variant rounded-lg focus:border-primary-fixed focus:ring-1 focus:ring-primary-fixed outline-none transition-all disabled:opacity-50"
          />
          <p className="text-xs text-on-surface-variant/70 mt-1">Leave blank to rely on automatic heuristics. Required if Runtime Type is explicitly set.</p>
        </div>

        <div>
          <label className="block text-sm font-medium text-on-surface-variant mb-1">Container Port Override</label>
          <input 
            type="number" 
            value={port}
            onChange={e => setPort(e.target.value)}
            disabled={isViewerRole}
            placeholder="e.g. 8080"
            className="w-full px-3 py-2 bg-surface-container/50 border border-outline-variant rounded-lg focus:border-primary-fixed focus:ring-1 focus:ring-primary-fixed outline-none transition-all disabled:opacity-50"
          />
          <p className="text-xs text-on-surface-variant/70 mt-1">The port your application listens on inside the container.</p>
        </div>

        <div>
          <label className="block text-sm font-medium text-on-surface-variant mb-1">Boot Health Check</label>
          <select
            value={healthCheckType}
            onChange={e => setHealthCheckType(e.target.value)}
            disabled={isViewerRole}
            className="w-full px-3 py-2 bg-surface-container/50 border border-outline-variant rounded-lg focus:border-primary-fixed focus:ring-1 focus:ring-primary-fixed outline-none transition-all disabled:opacity-50"
          >
            <option value="http">HTTP application readiness (requires 2xx)</option>
            <option value="tcp">TCP port connection</option>
            <option value="none">None (For workers or non-listening jobs)</option>
          </select>
          <p className="text-xs text-on-surface-variant/70 mt-1">HTTP requires a 2xx preview response. TCP requires a successful connection to the runtime port. None checks only that the runtime process remains alive.</p>
        </div>

        <div>
          <div className="flex items-center justify-between mb-4">
            <h4 className="text-sm font-medium text-on-surface-variant">Environment Variables</h4>
            {!isViewerRole && (
              <button
                onClick={() => setEnvVars([...envVars, { key: "", value: "" }])}
                className="text-xs flex items-center gap-1.5 px-3 py-1.5 bg-surface-container hover:bg-surface-container-high rounded-md transition-colors border border-outline-variant"
              >
                <Plus className="w-3.5 h-3.5" />
                Add Variable
              </button>
            )}
          </div>
          
          <div className="space-y-3">
            {envVars.length === 0 && (
              <p className="text-sm text-on-surface-variant/60 italic border border-dashed border-outline-variant rounded-lg p-4 text-center">
                No environment variables set.
              </p>
            )}
            {envVars.map((v, i) => (
              <div key={i} className="flex items-start gap-3">
                <div className="flex-1">
                  <input
                    type="text"
                    value={v.key}
                    onChange={(e) => {
                      const newVars = [...envVars]
                      newVars[i].key = e.target.value.toUpperCase().replace(/[^A-Z0-9_]/g, '')
                      setEnvVars(newVars)
                    }}
                    disabled={isViewerRole}
                    placeholder="NAME_OF_VARIABLE"
                    className="w-full px-3 py-2 bg-surface-container/50 border border-outline-variant rounded-lg focus:border-primary-fixed focus:ring-1 focus:ring-primary-fixed outline-none transition-all disabled:opacity-50 font-mono text-sm"
                  />
                </div>
                <div className="flex-1">
                  <input
                    type="text"
                    value={v.value}
                    onChange={(e) => {
                      const newVars = [...envVars]
                      newVars[i].value = e.target.value
                      setEnvVars(newVars)
                    }}
                    disabled={isViewerRole}
                    placeholder="value"
                    className="w-full px-3 py-2 bg-surface-container/50 border border-outline-variant rounded-lg focus:border-primary-fixed focus:ring-1 focus:ring-primary-fixed outline-none transition-all disabled:opacity-50 font-mono text-sm"
                  />
                </div>
                {!isViewerRole && (
                  <button
                    onClick={() => {
                      const newVars = [...envVars]
                      newVars.splice(i, 1)
                      setEnvVars(newVars)
                    }}
                    className="p-2 text-error/70 hover:text-error hover:bg-error/10 rounded-lg transition-colors mt-0.5 shrink-0"
                    title="Remove variable"
                  >
                    <Trash2 className="w-4 h-4" />
                  </button>
                )}
              </div>
            ))}
          </div>
        </div>

        {!isViewerRole && (
          <div className="pt-4 border-t border-outline-variant flex items-center justify-between">
            <p className="text-xs text-on-surface-variant/80">Changes apply on the next restart.</p>
            <button
              onClick={handleSave}
              disabled={isSaving}
              className="px-4 py-2 bg-primary-fixed text-on-primary-fixed rounded-lg text-sm font-semibold hover:opacity-90 transition-opacity flex items-center gap-2 disabled:opacity-50"
            >
              {isSaving ? <Loader2 className="w-4 h-4 animate-spin" /> : <Save className="w-4 h-4" />}
              Save Settings
            </button>
          </div>
        )}
      </div>
    </div>
  )
}
