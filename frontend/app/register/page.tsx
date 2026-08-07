'use client'

import React, { useState, useEffect, useRef } from 'react';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import toast from 'react-hot-toast';

const InteractiveNetworkCanvas = () => {
    const canvasRef = useRef<HTMLCanvasElement>(null);
    const mouseRef = useRef({ x: -1000, y: -1000 });

    useEffect(() => {
        const canvas = canvasRef.current;
        if (!canvas) return;
        const ctx = canvas.getContext('2d');
        if (!ctx) return;

        let animationFrameId: number;
        let width: number;
        let height: number;
        let nodes: {x: number, y: number, vx: number, vy: number, radius: number}[] = [];
        const nodeCount = 35; // Denser network

        const initNodes = () => {
            nodes = [];
            for (let i = 0; i < nodeCount; i++) {
                nodes.push({
                    x: Math.random() * width,
                    y: Math.random() * height,
                    vx: (Math.random() - 0.5) * 1.2,
                    vy: (Math.random() - 0.5) * 1.2,
                    radius: Math.random() * 2 + 1
                });
            }
        };

        const resize = () => {
            if (!canvas.parentElement) return;
            width = canvas.width = canvas.parentElement.offsetWidth;
            height = canvas.height = canvas.parentElement.offsetHeight;
            initNodes();
        };

        const draw = () => {
            ctx.clearRect(0, 0, width, height);
            ctx.lineWidth = 1;

            // Update and draw connections
            for (let i = 0; i < nodes.length; i++) {
                const n1 = nodes[i];
                n1.x += n1.vx;
                n1.y += n1.vy;

                // Bounce
                if (n1.x < 0 || n1.x > width) n1.vx *= -1;
                if (n1.y < 0 || n1.y > height) n1.vy *= -1;

                // Calculate mouse distance
                const mouseDist = Math.hypot(n1.x - mouseRef.current.x, n1.y - mouseRef.current.y);
                
                // Repel and Fade out if near mouse
                let nodeOpacity = 1;
                if (mouseDist < 200 && mouseRef.current.x !== -1000) {
                    nodeOpacity = Math.max(0, mouseDist / 200);
                    // Add slight repelling physics
                    const angle = Math.atan2(n1.y - mouseRef.current.y, n1.x - mouseRef.current.x);
                    n1.x += Math.cos(angle) * (200 - mouseDist) * 0.05;
                    n1.y += Math.sin(angle) * (200 - mouseDist) * 0.05;
                }

                ctx.beginPath();
                ctx.arc(n1.x, n1.y, n1.radius, 0, Math.PI * 2);
                ctx.fillStyle = `rgba(0, 240, 255, ${nodeOpacity})`;
                ctx.fill();

                // Connect nodes to each other
                for (let j = i + 1; j < nodes.length; j++) {
                    const n2 = nodes[j];
                    const dist = Math.hypot(n1.x - n2.x, n1.y - n2.y);
                    
                    const mouseDist2 = Math.hypot(n2.x - mouseRef.current.x, n2.y - mouseRef.current.y);
                    let nodeOpacity2 = 1;
                    if (mouseDist2 < 200 && mouseRef.current.x !== -1000) {
                        nodeOpacity2 = Math.max(0, mouseDist2 / 200);
                    }

                    if (dist < 150) {
                        const lineOpacity = Math.min(nodeOpacity, nodeOpacity2) * (0.15 - dist/150 * 0.15);
                        if (lineOpacity > 0.01) {
                            ctx.beginPath();
                            ctx.moveTo(n1.x, n1.y);
                            ctx.lineTo(n2.x, n2.y);
                            ctx.strokeStyle = `rgba(0, 240, 255, ${lineOpacity})`;
                            ctx.stroke();
                        }
                    }
                }
            }
            animationFrameId = requestAnimationFrame(draw);
        };

        const handleMouseMove = (e: MouseEvent) => {
            const rect = canvas.getBoundingClientRect();
            mouseRef.current = {
                x: e.clientX - rect.left,
                y: e.clientY - rect.top
            };
        };
        
        const handleMouseLeave = () => {
            mouseRef.current = { x: -1000, y: -1000 };
        };

        window.addEventListener('resize', resize);
        canvas.addEventListener('mousemove', handleMouseMove);
        canvas.addEventListener('mouseleave', handleMouseLeave);
        
        resize();
        draw();

        return () => {
            window.removeEventListener('resize', resize);
            canvas.removeEventListener('mousemove', handleMouseMove);
            canvas.removeEventListener('mouseleave', handleMouseLeave);
            cancelAnimationFrame(animationFrameId);
        };
    }, []);

    return (
        <canvas 
            ref={canvasRef} 
            className="absolute top-0 left-0 w-full h-full z-0 cursor-crosshair"
        />
    );
};

export default function SignupPage() {
    const router = useRouter();
    const [email, setEmail] = useState('');
    const [password, setPassword] = useState('');
    const [confirmPassword, setConfirmPassword] = useState('');
    const [showPassword, setShowPassword] = useState(false);
    
    // Request State
    const [isSubmitting, setIsSubmitting] = useState(false);
    const [error, setError] = useState<string | null>(null);
    const [success, setSuccess] = useState<string | null>(null);

    // Mouse glow coordinate tracking
    const glowRef = useRef<HTMLDivElement>(null);
    useEffect(() => {
        const handleMouseMove = (e: MouseEvent) => {
            if (glowRef.current) {
                glowRef.current.style.left = `${e.clientX}px`;
                glowRef.current.style.top = `${e.clientY}px`;
            }
        };
        window.addEventListener('mousemove', handleMouseMove);
        return () => window.removeEventListener('mousemove', handleMouseMove);
    }, []);

    // Calculate password strength (0 to 4 score)
    const getPasswordStrength = () => {
        if (!password) return 0;
        let score = 0;
        if (password.length >= 12) score++;
        if (/[A-Z]/.test(password)) score++;
        if (/[0-9]/.test(password)) score++;
        if (/[^A-Za-z0-9]/.test(password)) score++;
        return score;
    };

    const strength = getPasswordStrength();
    const strengthLabels = ['Weak', 'Fair', 'Moderate', 'Strong', 'Excellent'];

    const handleSubmit = async (e: React.FormEvent) => {
        e.preventDefault();
        setError(null);
        setSuccess(null);

        // Pre-validation
        if (!email.trim()) {
            setError("Email address is required");
            return;
        }
        if (password.length < 12) {
            setError("Password must be at least 12 characters long");
            return;
        }
        if (password !== confirmPassword) {
            setError("Passwords do not match");
            return;
        }

        setIsSubmitting(true);

        try {
            const response = await fetch("/api/auth/register", {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify({ email, password }),
            });

            const data = await response.json();

            if (!response.ok) {
                throw new Error(data.error || 'Registration failed');
            }

            toast.success(data.message || "Account created! Please check your email to verify.");
            setSuccess("Account created! Please check your email to verify.");
            setEmail('');
            setPassword('');
            setConfirmPassword('');
            
            setTimeout(() => {
                router.push("/login");
            }, 2000);
        } catch (err: any) {
            setError(err.message || 'Something went wrong');
            toast.error(err.message);
        } finally {
            setIsSubmitting(false);
        }
    };

    return (
        <div className="bg-background text-on-surface min-h-screen w-full flex relative overflow-hidden font-sans">
            {/* Split Screen Layout */}
            <main className="flex w-full min-h-screen">
                
                {/* LEFT SIDE: Visual Blueprint (55%) */}
                <section className="hidden lg:flex lg:w-[55%] relative bg-surface-container-lowest overflow-hidden flex-col items-center justify-center p-24">
                    {/* Animated grid pattern */}
                    <div 
                        className="absolute inset-0 opacity-20 pointer-events-none"
                        style={{
                            backgroundImage: 'radial-gradient(circle, rgba(0, 240, 255, 0.07) 1px, transparent 1px)',
                            backgroundSize: '32px 32px'
                        }}
                    ></div>
                    <div className="absolute inset-0 bg-[radial-gradient(circle_at_50%_50%,rgba(0,219,233,0.05),transparent_70%)]">
                        <InteractiveNetworkCanvas />
                    </div>
                    
                    {/* Content Block */}
                    <div className="relative w-full h-full flex flex-col items-center justify-center">
                        
                        <div className="z-10 text-center space-y-4 max-w-[36rem] mx-auto">
                            <h1 className="font-display-lg text-4xl md:text-5xl font-bold text-primary leading-tight tracking-tighter">
                                Start Building Today
                            </h1>
                            <p className="font-headline-md text-xl md:text-2xl text-on-surface-variant max-w-[28rem] mx-auto">
                                Create isolated environments in seconds
                            </p>
                        </div>
                    </div>

                    <div className="absolute bottom-8 left-8 right-8 flex justify-between items-end">
                        <div className="flex flex-col gap-1">
                            <span className="font-mono text-xs text-primary-fixed-dim/60">SYS_CORE: STABLE</span>
                            <span className="font-mono text-xs text-primary-fixed-dim/60">LATENCY: 12ms</span>
                        </div>
                        <div className="font-mono text-xs text-primary-fixed-dim/60">API_SANDBOX_v4.2.0</div>
                    </div>
                </section>

                {/* RIGHT SIDE: Signup Form (45%) */}
                <section className="w-full lg:w-[45%] bg-surface flex items-center justify-center p-6 md:p-10 overflow-y-auto">
                    <div className="w-full max-w-[480px] space-y-6">
                        
                        {/* Header & Brand */}
                        <div className="text-center lg:text-left space-y-4">
                            <div className="flex items-center gap-2 justify-center lg:justify-start">
                                <span className="material-symbols-outlined text-primary-fixed-dim text-[32px]">dataset</span>
                                <span className="font-sans text-2xl font-bold text-primary-fixed-dim tracking-tight">API Sandbox</span>
                            </div>
                            <div className="space-y-1">
                                <h2 className="text-2xl md:text-3xl font-semibold text-on-surface">Create your free account</h2>
                                <p className="text-sm text-on-surface-variant">No credit card required</p>
                            </div>
                        </div>

                        {/* Form Card */}
                        <div className="bg-surface-container-low border border-outline-variant/30 rounded-xl p-6 md:p-8 space-y-6 shadow-2xl">
                            {error && (
                                <div className="bg-error-container/20 border border-error/50 rounded-lg p-3 text-error text-sm">
                                    {error}
                                </div>
                            )}

                            {success && (
                                <div className="bg-tertiary-container/20 border border-tertiary-fixed-dim/50 rounded-lg p-3 text-tertiary-fixed-dim text-sm">
                                    {success}
                                </div>
                            )}

                            <form className="space-y-4" onSubmit={handleSubmit}>
                                {/* Email */}
                                <div className="space-y-1">
                                    <label className="text-xs font-semibold text-on-surface-variant uppercase tracking-wider" htmlFor="email">Email Address</label>
                                    <input 
                                        className="w-full bg-surface-container-highest border border-outline-variant/50 rounded-lg px-4 py-2.5 text-on-surface placeholder:text-outline/70 focus:outline-none focus:ring-2 focus:ring-primary-fixed-dim/40 focus:border-primary-fixed-dim transition-all text-sm" 
                                        id="email" 
                                        placeholder="dev@company.com" 
                                        type="email" 
                                        value={email}
                                        onChange={(e) => setEmail(e.target.value)}
                                        required
                                    />
                                </div>

                                {/* Password */}
                                <div className="space-y-1">
                                    <label className="text-xs font-semibold text-on-surface-variant uppercase tracking-wider" htmlFor="password">Password</label>
                                    <div className="relative">
                                        <input 
                                            className="w-full bg-surface-container-highest border border-outline-variant/50 rounded-lg px-4 py-2.5 pr-10 text-on-surface placeholder:text-outline/70 focus:outline-none focus:ring-2 focus:ring-primary-fixed-dim/40 focus:border-primary-fixed-dim transition-all text-sm" 
                                            id="password" 
                                            placeholder="••••••••" 
                                            type={showPassword ? "text" : "password"} 
                                            value={password}
                                            onChange={(e) => setPassword(e.target.value)}
                                            required
                                            minLength={12}
                                        />
                                        <button 
                                            className="absolute right-3 top-1/2 -translate-y-1/2 text-on-surface-variant hover:text-primary transition-colors" 
                                            type="button"
                                            onClick={() => setShowPassword(!showPassword)}
                                        >
                                            <span className="material-symbols-outlined text-lg">{showPassword ? "visibility_off" : "visibility"}</span>
                                        </button>
                                    </div>
                                    
                                    {/* Strength Indicator */}
                                    {password && (
                                        <div className="flex gap-1 mt-2 items-center">
                                            {[...Array(4)].map((_, i) => (
                                                <div 
                                                    key={i} 
                                                    className={`h-1 flex-1 rounded-full transition-all duration-300 ${
                                                        i < strength 
                                                            ? (strength <= 1 ? 'bg-error' : strength <= 3 ? 'bg-primary-fixed-dim' : 'bg-tertiary-fixed-dim') 
                                                            : 'bg-surface-variant'
                                                    }`}
                                                ></div>
                                            ))}
                                            <span className="text-[10px] font-semibold text-on-surface-variant uppercase ml-2 tracking-wider">
                                                {strengthLabels[strength]}
                                            </span>
                                        </div>
                                    )}
                                </div>

                                {/* Confirm Password */}
                                <div className="space-y-1">
                                    <label className="text-xs font-semibold text-on-surface-variant uppercase tracking-wider" htmlFor="confirm-password">Confirm Password</label>
                                    <input 
                                        className="w-full bg-surface-container-highest border border-outline-variant/50 rounded-lg px-4 py-2.5 text-on-surface placeholder:text-outline/70 focus:outline-none focus:ring-2 focus:ring-primary-fixed-dim/40 focus:border-primary-fixed-dim transition-all text-sm" 
                                        id="confirm-password" 
                                        placeholder="••••••••" 
                                        type="password" 
                                        value={confirmPassword}
                                        onChange={(e) => setConfirmPassword(e.target.value)}
                                        required
                                        minLength={12}
                                    />
                                </div>

                                {/* Submit Button */}
                                <button 
                                    className="w-full bg-primary-container text-on-primary font-bold text-base py-3 rounded-lg hover:brightness-110 active:scale-95 transition-all duration-200 mt-6 shadow-[0_0_20px_rgba(0,240,255,0.15)] flex justify-center items-center" 
                                    type="submit"
                                    disabled={isSubmitting}
                                >
                                    {isSubmitting ? (
                                        <span className="material-symbols-outlined animate-spin text-xl">progress_activity</span>
                                    ) : (
                                        "Create Account"
                                    )}
                                </button>
                            </form>

                            {/* Divider */}
                            <div className="relative py-2 flex items-center gap-4">
                                <div className="flex-grow border-t border-outline-variant/20"></div>
                                <span className="text-[10px] font-bold text-outline uppercase tracking-widest shrink-0">Or continue with</span>
                                <div className="flex-grow border-t border-outline-variant/20"></div>
                            </div>

                            {/* Social Signups */}
                            <div className="grid grid-cols-2 gap-3">
                                <button type="button" disabled className="flex items-center justify-center gap-2 bg-surface-container-highest border border-primary-fixed-dim/20 opacity-50 cursor-not-allowed text-on-surface text-xs font-semibold py-2.5 px-4 rounded-lg transition-all duration-200">
                                    <svg className="w-4 h-4 fill-current" viewBox="0 0 24 24">
                                        <path d="M12 0c-6.626 0-12 5.373-12 12 0 5.302 3.438 9.8 8.207 11.387.599.111.793-.261.793-.577v-2.234c-3.338.726-4.033-1.416-4.033-1.416-.546-1.387-1.333-1.756-1.333-1.756-1.089-.745.083-.729.083-.729 1.205.084 1.839 1.237 1.839 1.237 1.07 1.834 2.807 1.304 3.492.997.107-.775.418-1.305.762-1.604-2.665-.305-5.467-1.334-5.467-5.931 0-1.311.469-2.381 1.236-3.221-.124-.303-.535-1.524.117-3.176 0 0 1.008-.322 3.301 1.23.957-.266 1.983-.399 3.003-.404 1.02.005 2.047.138 3.006.404 2.291-1.552 3.297-1.23 3.297-1.23.653 1.653.242 2.874.118 3.176.77.84 1.235 1.911 1.235 3.221 0 4.609-2.807 5.624-5.479 5.921.43.372.823 1.102.823 2.222v3.293c0 .319.192.694.801.576 4.765-1.589 8.199-6.086 8.199-11.386 0-6.627-5.373-12-12-12z"></path>
                                    </svg>
                                    <span>GitHub</span>
                                </button>
                                <button type="button" disabled className="flex items-center justify-center gap-2 bg-surface-container-highest border border-outline-variant/50 opacity-50 cursor-not-allowed text-on-surface text-xs font-semibold py-2.5 px-4 rounded-lg transition-all duration-200">
                                    <svg className="w-4 h-4" viewBox="0 0 48 48">
                                        <path d="M24 9.5c3.54 0 6.71 1.22 9.21 3.6l6.85-6.85C35.9 2.38 30.47 0 24 0 14.62 0 6.51 5.38 2.56 13.22l7.98 6.19C12.43 13.72 17.74 9.5 24 9.5z" fill="#EA4335"></path>
                                        <path d="M46.98 24.55c0-1.57-.15-3.09-.38-4.55H24v9.02h12.94c-.58 2.96-2.26 5.48-4.78 7.18l7.73 6c4.51-4.18 7.09-10.36 7.09-17.65z" fill="#4285F4"></path>
                                        <path d="M10.53 28.59c-.48-1.45-.76-2.99-.76-4.59s.27-3.14.76-4.59l-7.98-6.19C.92 16.46 0 20.12 0 24c0 3.88.92 7.54 2.56 10.78l7.97-6.19z" fill="#FBBC05"></path>
                                        <path d="M24 48c6.48 0 11.93-2.13 15.89-5.81l-7.73-6c-2.15 1.45-4.92 2.3-8.16 2.3-6.26 0-11.57-4.22-13.47-9.91l-7.98 6.19C6.51 42.62 14.62 48 24 48z" fill="#34A853"></path>
                                    </svg>
                                    <span>Google</span>
                                </button>
                            </div>
                        </div>

                        {/* Bottom Links */}
                        <div className="text-center space-y-3">
                            <p className="text-xs text-on-surface-variant max-w-[320px] mx-auto leading-relaxed">
                                By signing up, you agree to our Terms of Service and Privacy Policy.
                            </p>
                            <p className="text-sm text-on-surface">
                                Already have an account? <Link className="text-primary-fixed-dim font-bold hover:underline transition-all" href="/login">Sign in</Link>
                            </p>
                        </div>
                    </div>
                </section>
            </main>

            {/* Ambient mouse cursor glow */}
            <div 
                ref={glowRef}
                className="fixed pointer-events-none w-[600px] h-[600px] bg-[radial-gradient(circle,rgba(0,240,255,0.03)_0%,transparent_70%)] rounded-full -translate-x-1/2 -translate-y-1/2 z-0" 
                id="cursor-glow"
                style={{ transition: 'transform 0.1s ease-out' }}
            ></div>
        </div>
    );
}
