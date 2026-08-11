import React, { useState, useEffect } from "react";
// @ts-expect-error react-hook-form 7.83.0 react-server types bug
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import * as z from "zod";
import { X, Server, Database, Cloud, Zap, Loader2 } from "lucide-react";
import toast from "react-hot-toast";
import { fetchWithAuth } from "@/lib/auth";

const schema = z.object({
  name: z.string().min(1, "Name is required").default("Untitled Deployment"),
  projectId: z.string().min(1, "Project is required"),
  gitUrl: z.string().url("Must be a valid URL").regex(/^https:\/\/github\.com/, "Must be a GitHub repository"),
  gitBranch: z.string().min(1, "Branch is required").default("main"),
  providerType: z.enum(["docker"]).default("docker"),
  dbAddon: z.enum(["none", "postgres", "mongo", "redis"]).default("none"),
});

type FormData = z.infer<typeof schema>;

interface Props {
  isOpen: boolean;
  onClose: () => void;
  onDeploy: (deployment: any) => void;
}

export default function NewDeploymentModal({ isOpen, onClose, onDeploy }: Props) {
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [projects, setProjects] = useState<any[]>([]);

  const {
    register,
    handleSubmit,
    watch,
    formState: { errors },
    reset,
  } = useForm<FormData>({
    resolver: zodResolver(schema),
    defaultValues: { name: "", gitBranch: "main", providerType: "docker", dbAddon: "none" },
  });

  const providerType = watch("providerType");
  const dbAddon = watch("dbAddon");

  useEffect(() => {
    if (isOpen) {
      fetchWithAuth("/api/projects").then(setProjects).catch(console.error);
    }
  }, [isOpen]);

  const onSubmit = async (data: FormData) => {
    setIsSubmitting(true);
    try {
      // 1. Create Deployment
      const deployRes = await fetchWithAuth("/api/deployments", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          name: data.name,
          projectId: data.projectId,
          gitUrl: data.gitUrl,
          gitBranch: data.gitBranch,
          providerType: data.providerType,
        }),
      });

      // 2. Add Add-on if selected
      if (data.dbAddon !== "none") {
        await fetchWithAuth(`/api/deployments/${deployRes.id}/addons`, {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ type: data.dbAddon, plan: "free" }),
        });
      }

      toast.success("Deployment queued successfully!");
      onDeploy(deployRes);
      reset();
      onClose();
    } catch (error: any) {
      toast.error(error.message || "Failed to create deployment");
    } finally {
      setIsSubmitting(false);
    }
  };

  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/60 backdrop-blur-sm">
      <div className="bg-surface-container border border-outline-variant rounded-2xl w-full max-w-2xl max-h-[90vh] overflow-y-auto">
        <div className="sticky top-0 z-10 flex items-center justify-between p-6 bg-surface-container/90 backdrop-blur border-b border-outline-variant/50">
          <div>
            <h2 className="text-xl font-bold text-on-surface">New Deployment</h2>
            <p className="text-sm text-on-surface-variant">Configure provider and add-ons for your sandbox.</p>
          </div>
          <button onClick={onClose} className="p-2 hover:bg-on-surface/5 rounded-xl transition-colors">
            <X className="w-5 h-5 text-on-surface-variant" />
          </button>
        </div>

        <form onSubmit={handleSubmit(onSubmit)} className="p-6 space-y-6">
          <div className="space-y-2">
            <label className="text-xs font-bold uppercase tracking-wider text-on-surface-variant">Deployment Name</label>
            <input {...register("name")} className="w-full bg-surface-container-lowest border border-outline-variant rounded-xl px-4 py-2.5 text-sm" placeholder="e.g. Production API Sandbox" />
            {errors.name && <p className="text-xs text-error">{errors.name.message}</p>}
          </div>

          <div className="grid grid-cols-2 gap-4">
            <div className="space-y-2">
              <label className="text-xs font-bold uppercase tracking-wider text-on-surface-variant">Project</label>
              <select {...register("projectId")} className="w-full bg-surface-container-lowest border border-outline-variant rounded-xl px-4 py-2.5 text-sm">
                <option value="">Select Project...</option>
                {projects.map(p => (
                  <option key={p.id} value={p.id}>{p.name}</option>
                ))}
              </select>
              {errors.projectId && <p className="text-xs text-error">{errors.projectId.message}</p>}
            </div>
            <div className="space-y-2">
              <label className="text-xs font-bold uppercase tracking-wider text-on-surface-variant">Git Branch</label>
              <input {...register("gitBranch")} className="w-full bg-surface-container-lowest border border-outline-variant rounded-xl px-4 py-2.5 text-sm" placeholder="main" />
              {errors.gitBranch && <p className="text-xs text-error">{errors.gitBranch.message}</p>}
            </div>
          </div>

          <div className="space-y-2">
            <label className="text-xs font-bold uppercase tracking-wider text-on-surface-variant">GitHub Repository URL</label>
            <input {...register("gitUrl")} className="w-full bg-surface-container-lowest border border-outline-variant rounded-xl px-4 py-2.5 text-sm" placeholder="https://github.com/org/repo" />
            {errors.gitUrl && <p className="text-xs text-error">{errors.gitUrl.message}</p>}
          </div>

          <div className="space-y-3">
            <label className="text-xs font-bold uppercase tracking-wider text-on-surface-variant">Infrastructure Provider</label>
            <div className="grid grid-cols-3 gap-3">
              {[
                { id: "docker", name: "Local Docker", icon: Server },
              ].map(provider => (
                <label key={provider.id} className={`flex flex-col items-center gap-2 p-4 rounded-xl border cursor-pointer transition-all ${providerType === provider.id ? 'border-primary-fixed bg-primary-fixed/10 text-primary-fixed' : 'border-outline-variant hover:border-outline text-on-surface-variant'}`}>
                  <input type="radio" value={provider.id} {...register("providerType")} className="hidden" />
                  <provider.icon className="w-6 h-6" />
                  <span className="text-sm font-semibold">{provider.name}</span>
                </label>
              ))}
            </div>
          </div>

          <div className="space-y-3">
            <label className="text-xs font-bold uppercase tracking-wider text-on-surface-variant">Database Add-on (Optional)</label>
            <div className="grid grid-cols-4 gap-3">
              {[
                { id: "none", name: "None" },
                { id: "postgres", name: "PostgreSQL" },
                { id: "mongo", name: "MongoDB" },
                { id: "redis", name: "Redis" },
              ].map(db => (
                <label key={db.id} className={`flex items-center justify-center p-3 rounded-xl border cursor-pointer transition-all ${dbAddon === db.id ? 'border-primary-fixed bg-primary-fixed/10 text-primary-fixed' : 'border-outline-variant hover:border-outline text-on-surface-variant'}`}>
                  <input type="radio" value={db.id} {...register("dbAddon")} className="hidden" />
                  <span className="text-sm font-semibold">{db.name}</span>
                </label>
              ))}
            </div>
          </div>

          <div className="pt-4 border-t border-outline-variant flex justify-end gap-3">
            <button type="button" onClick={onClose} className="px-5 py-2.5 text-sm font-bold text-on-surface-variant hover:text-on-surface transition-colors">Cancel</button>
            <button type="submit" disabled={isSubmitting} className="px-6 py-2.5 bg-primary-fixed text-on-primary-fixed rounded-xl text-sm font-bold hover:brightness-110 active:scale-95 transition-all flex items-center gap-2">
              {isSubmitting ? <Loader2 className="w-4 h-4 animate-spin" /> : <Server className="w-4 h-4" />}
              Deploy
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
