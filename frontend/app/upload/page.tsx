"use client";

import { useState, useEffect } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import * as z from "zod";
import { useRouter } from "next/navigation";
import toast from "react-hot-toast";
import { Code, Loader2 } from "lucide-react";
import { fetchWithAuth } from "@/lib/auth";

const schema = z.object({
  name: z.string().min(3, "Name must be at least 3 characters").max(50),
  gitUrl: z.string().url("Must be a valid URL").regex(/^https:\/\/github\.com/, "Must be a GitHub repository"),
  githubBranch: z.string().min(1, "Branch is required").default("main"),
});

type FormData = z.infer<typeof schema>;

export default function UploadPage() {
  const router = useRouter();
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [branches, setBranches] = useState<string[]>([]);
  const [isFetchingBranches, setIsFetchingBranches] = useState(false);

  const {
    register,
    handleSubmit,
    watch,
    setValue,
    formState: { errors },
  } = useForm<FormData>({
    resolver: zodResolver(schema),
    defaultValues: { githubBranch: "main" },
  });

  const gitUrl = watch("gitUrl");

  useEffect(() => {
    const fetchBranches = async () => {
      if (!gitUrl || !gitUrl.startsWith("https://github.com/")) return;
      
      const match = gitUrl.match(/https:\/\/github\.com\/([^/]+)\/([^/.]+)/);
      if (!match) return;

      const owner = match[1];
      const repo = match[2];

      setIsFetchingBranches(true);
      try {
        const res = await fetch(`https://api.github.com/repos/${owner}/${repo}/branches`);
        if (!res.ok) {
          setBranches([]);
          return;
        }
        const data = await res.json();
        const branchNames = data.map((b: any) => b.name);
        setBranches(branchNames);
        
        // Auto-select main or master if available
        if (branchNames.includes("main")) setValue("githubBranch", "main");
        else if (branchNames.includes("master")) setValue("githubBranch", "master");
        else if (branchNames.length > 0) setValue("githubBranch", branchNames[0]);
      } catch (err) {
        // Silently fail and fallback to manual input if network request fails entirely
        setBranches([]);
      } finally {
        setIsFetchingBranches(false);
      }
    };

    const timeout = setTimeout(fetchBranches, 800);
    return () => clearTimeout(timeout);
  }, [gitUrl, setValue]);

  const onSubmit = async (data: FormData) => {
    setIsSubmitting(true);
    try {
      const response = await fetchWithAuth("/api/environments", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(data),
      });

      const env = response;
      toast.success("Sandbox created! Building image...");
      router.push(`/environments/${env.id}`);
    } catch (error: any) {
      toast.error(error.message);
    } finally {
      setIsSubmitting(false);
    }
  };

  return (
    <div className="max-w-2xl mx-auto mt-12">
      <div className="glass-panel p-8 md:p-12">
        <div className="text-center mb-10">
          <div className="inline-flex items-center justify-center w-16 h-16 rounded-2xl bg-primary/10 text-primary mb-6 ring-1 ring-primary/20">
            <Code className="w-8 h-8" />
          </div>
          <h1 className="text-3xl font-bold text-white mb-3">Deploy New Sandbox</h1>
          <p className="text-white/60">Connect a public GitHub repository to instantly build and deploy an isolated container.</p>
        </div>

        <form onSubmit={handleSubmit(onSubmit)} className="space-y-6">
          <div className="space-y-2">
            <label className="text-sm font-medium text-white/80">Project Name</label>
            <input
              {...register("name")}
              placeholder="e.g. my-awesome-api"
              className="glass-input w-full px-4 py-3"
            />
            {errors.name && <p className="text-sm text-red-400">{errors.name.message}</p>}
          </div>

          <div className="space-y-2">
            <label className="text-sm font-medium text-white/80">GitHub Repository URL</label>
            <input
              {...register("gitUrl")}
              placeholder="https://github.com/username/repo"
              className="glass-input w-full px-4 py-3"
            />
            {errors.gitUrl && <p className="text-sm text-red-400">{errors.gitUrl.message}</p>}
          </div>

          <div className="space-y-2">
            <label className="text-sm font-medium text-white/80 flex items-center gap-2">
              Branch 
              {isFetchingBranches && <Loader2 className="w-3 h-3 animate-spin text-white/50" />}
            </label>
            {branches.length > 0 ? (
              <select
                {...register("githubBranch")}
                className="glass-input w-full px-4 py-3 appearance-none bg-transparent"
              >
                {branches.map(b => (
                  <option key={b} value={b} className="bg-slate-900 text-white">{b}</option>
                ))}
              </select>
            ) : (
              <input
                {...register("githubBranch")}
                placeholder="main"
                className="glass-input w-full px-4 py-3"
              />
            )}
            {errors.githubBranch && <p className="text-sm text-red-400">{errors.githubBranch.message}</p>}
          </div>

          <button
            type="submit"
            disabled={isSubmitting}
            className="glass-button w-full py-4 mt-8 flex items-center justify-center text-lg"
          >
            {isSubmitting ? (
              <>
                <Loader2 className="w-5 h-5 mr-2 animate-spin" />
                Deploying...
              </>
            ) : (
              "Deploy Project"
            )}
          </button>
        </form>
      </div>
    </div>
  );
}
