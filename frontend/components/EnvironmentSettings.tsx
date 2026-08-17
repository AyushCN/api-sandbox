import { useState } from "react"
import toast from "react-hot-toast"
import { Loader2, Settings, Save } from "lucide-react"

export default function EnvironmentSettings({ env, mutate, isViewerRole }: { env: Record<string, unknown>, mutate: () => void, isViewerRole: boolean }) {
  const [startCommand, setStartCommand] = useState((env.startCommand as string) || "")
  const [port, setPort] = useState((env.port as number)?.toString() || "")
  const [healthCheckType, setHealthCheckType] = useState((env.healthCheckType as string) || "tcp")
  const [isSaving, setIsSaving] = useState(false)

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
          port: port ? parseInt(port) : null,
          healthCheckType
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
          <p className="text-xs text-on-surface-variant/70 mt-1">Leave blank to rely on automatic heuristics (e.g. package.json).</p>
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
            <option value="tcp">TCP/HTTP Port Check (Wait until port binds)</option>
            <option value="none">None (For workers or non-listening jobs)</option>
          </select>
          <p className="text-xs text-on-surface-variant/70 mt-1">TCP ensures you don&apos;t get 502 Bad Gateway during boot. Disable this for background workers.</p>
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
