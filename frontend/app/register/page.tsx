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

                        <div className="space-y-4 mb-8">
                            <a 
                                href="/api/auth/github"
                                className="w-full flex items-center justify-center gap-3 bg-[#24292e] text-white font-bold py-4 rounded-lg hover:bg-[#2f363d] active:scale-95 transition-all duration-200 text-lg border border-outline-variant/30 shadow-[0_0_20px_rgba(0,240,255,0.15)]"
                            >
                                <svg height="24" aria-hidden="true" viewBox="0 0 16 16" version="1.1" width="24" data-view-component="true" className="fill-current">
                                    <path d="M8 0c4.42 0 8 3.58 8 8a8.013 8.013 0 0 1-5.45 7.59c-.4.08-.55-.17-.55-.38 0-.27.01-1.13.01-2.2 0-.75-.25-1.23-.54-1.48 1.78-.2 3.65-.88 3.65-3.95 0-.88-.31-1.59-.82-2.15.08-.2.36-1.02-.08-2.12 0 0-.67-.22-2.2.82-.64-.18-1.32-.27-2-.27-.68 0-1.36.09-2 .27-1.53-1.03-2.2-.82-2.2-.82-.44 1.1-.16 1.92-.08 2.12-.51.56-.82 1.28-.82 2.15 0 3.06 1.86 3.75 3.64 3.95-.23.2-.44.55-.51 1.07-.46.21-1.61.55-2.33-.66-.15-.24-.6-.83-1.23-.82-.67.01-.27.38.01.53.34.19.73.9.82 1.13.16.45.68 1.31 2.69.94 0 .67.01 1.3.01 1.49 0 .21-.15.45-.55.38A7.995 7.995 0 0 1 0 8c0-4.42 3.58-8 8-8Z"></path>
                                </svg>
                                Continue with GitHub
                            </a>
                        </div>
                        <div className="mt-8 text-center text-on-surface-variant text-sm">
                            <p>We've moved fully to GitHub. Email/password registration is disabled.</p>
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
