"use client";

import { useState, useEffect } from "react";
// @ts-expect-error react-hook-form 7.83.0 react-server types bug
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
    <div className="max-w-2xl mx-auto mt-12 mb-24">
      <div className="bg-surface-container-lowest border border-outline-variant rounded-2xl p-8 md:p-12 shadow-2xl relative overflow-hidden">
        <div className="absolute top-0 right-0 p-16 opacity-5 pointer-events-none">
          <span className="material-symbols-outlined text-[200px] text-primary-fixed">cloud_upload</span>
        </div>
        
        <div className="text-center mb-10 relative z-10">
          <div className="inline-flex items-center justify-center w-16 h-16 rounded-2xl bg-primary-container/20 text-primary-fixed mb-6 border border-primary-container/30 hover:scale-110 transition-transform">
            <span className="material-symbols-outlined text-[32px]">deployed_code</span>
          </div>
          <h1 className="text-3xl font-bold text-on-surface mb-3">Deploy New Sandbox</h1>
          <p className="text-on-surface-variant">Connect a public GitHub repository to instantly build and deploy an isolated container.</p>
        </div>

        <form onSubmit={handleSubmit(onSubmit)} className="space-y-6 relative z-10">
          <div className="space-y-2">
            <label className="text-sm font-bold tracking-wide text-on-surface-variant uppercase">Project Name</label>
            <input
              {...register("name")}
              placeholder="e.g. my-awesome-api"
              className="w-full bg-surface-container px-4 py-3 rounded-lg border border-outline-variant text-on-surface focus:border-primary-fixed focus:ring-1 focus:ring-primary-fixed transition-all"
            />
            {errors.name && <p className="text-sm text-error font-medium">{errors.name.message}</p>}
          </div>

          <div className="space-y-2">
            <label className="text-sm font-bold tracking-wide text-on-surface-variant uppercase">GitHub Repository URL</label>
            <input
              {...register("gitUrl")}
              placeholder="https://github.com/username/repo"
              className="w-full bg-surface-container px-4 py-3 rounded-lg border border-outline-variant text-on-surface focus:border-primary-fixed focus:ring-1 focus:ring-primary-fixed transition-all"
            />
            {errors.gitUrl && <p className="text-sm text-error font-medium">{errors.gitUrl.message}</p>}
          </div>

          <div className="space-y-2">
            <label className="text-sm font-bold tracking-wide text-on-surface-variant uppercase flex items-center gap-2">
              Branch 
              {isFetchingBranches && <Loader2 className="w-3 h-3 animate-spin text-primary-fixed" />}
            </label>
            {branches.length > 0 ? (
              <select
                {...register("githubBranch")}
                className="w-full bg-surface-container px-4 py-3 rounded-lg border border-outline-variant text-on-surface focus:border-primary-fixed focus:ring-1 focus:ring-primary-fixed transition-all appearance-none"
              >
                {branches.map(b => (
                  <option key={b} value={b} className="bg-surface-container text-on-surface">{b}</option>
                ))}
              </select>
            ) : (
              <input
                {...register("githubBranch")}
                placeholder="main"
                className="w-full bg-surface-container px-4 py-3 rounded-lg border border-outline-variant text-on-surface focus:border-primary-fixed focus:ring-1 focus:ring-primary-fixed transition-all"
              />
            )}
            {errors.githubBranch && <p className="text-sm text-error font-medium">{errors.githubBranch.message}</p>}
          </div>

          <button
            type="submit"
            disabled={isSubmitting}
            className="w-full py-4 mt-8 flex items-center justify-center text-lg bg-primary-container text-on-primary-fixed-variant rounded-xl font-bold hover:shadow-[0_0_20px_rgba(0,240,255,0.2)] active:scale-95 transition-all disabled:opacity-50 disabled:pointer-events-none"
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
