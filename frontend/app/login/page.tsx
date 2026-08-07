'use client'

import React, { useEffect, useRef, useState } from 'react';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { useAuth } from '@/lib/auth';
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

                ctx.beginPath();
                ctx.arc(n1.x, n1.y, n1.radius, 0, Math.PI * 2);
                ctx.fillStyle = '#00f0ff';
                ctx.fill();

                // Connect nodes to each other
                for (let j = i + 1; j < nodes.length; j++) {
                    const n2 = nodes[j];
                    const dist = Math.hypot(n1.x - n2.x, n1.y - n2.y);
                    if (dist < 150) {
                        ctx.beginPath();
                        ctx.moveTo(n1.x, n1.y);
                        ctx.lineTo(n2.x, n2.y);
                        ctx.strokeStyle = `rgba(0, 240, 255, ${0.15 - dist/150 * 0.15})`;
                        ctx.stroke();
                    }
                }

                // Connect to mouse
                const mouseDist = Math.hypot(n1.x - mouseRef.current.x, n1.y - mouseRef.current.y);
                if (mouseDist < 250) {
                    ctx.beginPath();
                    ctx.moveTo(n1.x, n1.y);
                    ctx.lineTo(mouseRef.current.x, mouseRef.current.y);
                    ctx.strokeStyle = `rgba(0, 240, 255, ${0.4 - mouseDist/250 * 0.4})`;
                    ctx.lineWidth = 1.5;
                    ctx.stroke();
                    ctx.lineWidth = 1;
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

export default function LoginPage() {
    const router = useRouter();
    const { login } = useAuth();
    const [email, setEmail] = useState('');
    const [password, setPassword] = useState('');
    const [error, setError] = useState('');
    const [loading, setLoading] = useState(false);
    const [showLoader, setShowLoader] = useState(false);
    
    const handleLogin = async (e: React.FormEvent) => {
        e.preventDefault();
        setError('');
        setLoading(true);

        try {
            const res = await fetch("/api/auth/login", {
                method: "POST",
                headers: { "Content-Type": "application/json" },
                body: JSON.stringify({ email, password }),
                credentials: "include"
            });
            
            const data = await res.json();
            if (!res.ok) throw new Error(data.error || "Login failed");
            
            toast.success("Welcome back!");
            setShowLoader(true);
            setTimeout(() => {
                login();
            }, 1500); // Simulate the beautiful loader animation for a bit before navigating
        } catch (err: any) {
            setError(err.message);
            toast.error(err.message);
        } finally {
            setLoading(false);
        }
    };

    if (showLoader) {
        return (
            <div className="bg-black text-on-surface min-h-screen overflow-hidden flex flex-col items-center justify-center relative font-sans selection:bg-primary-container selection:text-black">
                <style jsx global>{`
                    .blueprint-bg {
                        background-image: 
                            linear-gradient(rgba(0, 240, 255, 0.03) 1px, transparent 1px),
                            linear-gradient(90deg, rgba(0, 240, 255, 0.03) 1px, transparent 1px);
                        background-size: 40px 40px;
                        background-position: center center;
                    }
                    .spinner-ring {
                        border: 2px solid transparent;
                        border-top-color: #00f0ff;
                        border-right-color: rgba(0, 240, 255, 0.3);
                        border-radius: 50%;
                        animation: spin 1.5s linear infinite;
                        box-shadow: 0 0 15px rgba(0, 240, 255, 0.2), inset 0 0 10px rgba(0, 240, 255, 0.1);
                    }
                    .spinner-ring-inner {
                        border: 2px solid transparent;
                        border-left-color: #7df4ff;
                        border-bottom-color: rgba(125, 244, 255, 0.3);
                        border-radius: 50%;
                        animation: spin-reverse 2s linear infinite;
                    }
                    .pulse-glow {
                        animation: pulse-op 2s ease-in-out infinite alternate;
                    }
                    .terminal-text {
                        overflow: hidden;
                        border-right: .15em solid #00f0ff;
                        white-space: nowrap;
                        margin: 0 auto;
                        letter-spacing: .15em;
                        animation: typing 2.5s steps(40, end), blink-caret .75s step-end infinite;
                    }
                    @keyframes spin { 100% { transform: rotate(360deg); } }
                    @keyframes spin-reverse { 100% { transform: rotate(-360deg); } }
                    @keyframes pulse-op {
                        0% { opacity: 0.6; text-shadow: 0 0 5px rgba(0,240,255,0.2); }
                        100% { opacity: 1; text-shadow: 0 0 15px rgba(0,240,255,0.6); }
                    }
                    @keyframes typing { from { width: 0 } to { width: 100% } }
                    @keyframes blink-caret { from, to { border-color: transparent } 50% { border-color: #00f0ff; } }
                `}</style>
                
                {/* Background Grid */}
                <div className="absolute inset-0 blueprint-bg z-0 pointer-events-none opacity-50"></div>
                <main className="z-10 flex flex-col items-center justify-center flex-grow w-full max-w-[1440px] px-8 relative">
                    {/* Logo & Spinner Container */}
                    <div className="relative w-48 h-48 flex items-center justify-center mb-12">
                        <div className="absolute inset-0 spinner-ring"></div>
                        <div className="absolute inset-4 spinner-ring-inner"></div>
                        <div className="relative z-10 text-primary-container drop-shadow-[0_0_15px_rgba(0,240,255,0.5)]">
                            <span className="material-symbols-outlined text-[64px]" style={{ fontVariationSettings: "'FILL' 0, 'wght' 200" }}>hexagon</span>
                            <span className="material-symbols-outlined absolute inset-0 m-auto flex items-center justify-center text-[24px]" style={{ fontVariationSettings: "'FILL' 1, 'wght' 400" }}>memory</span>
                        </div>
                        <div className="absolute top-0 left-1/2 -translate-x-1/2 -translate-y-1/2 w-2 h-2 bg-primary-container rounded-full shadow-[0_0_8px_#00f0ff]"></div>
                        <div className="absolute bottom-0 left-1/2 -translate-x-1/2 translate-y-1/2 w-2 h-2 bg-primary-container rounded-full shadow-[0_0_8px_#00f0ff]"></div>
                    </div>
                    
                    {/* Text */}
                    <h1 className="text-2xl md:text-3xl font-bold text-primary-container tracking-widest uppercase pulse-glow text-center mb-4">
                        Preparing your environment...
                    </h1>
                    <p className="text-base text-on-surface-variant opacity-70 tracking-wide">
                        Initializing core modules
                    </p>
                </main>
                
                {/* Footer strings */}
                <footer className="z-10 w-full px-8 py-8 flex flex-col items-center justify-center absolute bottom-0 opacity-80">
                    <div className="w-full max-w-lg border-t border-primary-container/20 pt-4 flex justify-between items-center px-4">
                        <span className="font-mono text-primary-container/70 terminal-text text-xs md:text-sm">
                            ESTABLISHING SECURE CONNECTION // NODE: ALPHA-7
                        </span>
                        <span className="font-mono text-outline hidden md:block text-sm">
                            SYS.VER.4.5.2
                        </span>
                    </div>
                </footer>
            </div>
        );
    }

    return (
        <div className="bg-background text-on-surface min-h-screen selection:bg-primary-container/30">
            <style jsx global>{`
                .blueprint-bg {
                    background-image: radial-gradient(circle at 2px 2px, rgba(0, 219, 233, 0.05) 1px, transparent 0);
                    background-size: 40px 40px;
                }
                .glow-effect {
                    box-shadow: 0 0 20px rgba(0, 240, 255, 0.15);
                }
            `}</style>
            <main className="flex min-h-screen overflow-hidden">
                {/* Left Side: Visual Experience (55%) */}
                <section className="hidden lg:flex lg:w-[55%] relative flex-col justify-center items-center p-16 bg-surface-container-lowest overflow-hidden border-r border-outline-variant/30">
                    <div className="absolute inset-0 z-0 bg-[radial-gradient(circle_at_50%_50%,rgba(0,219,233,0.05),transparent_70%)]">
                        <InteractiveNetworkCanvas />
                    </div>
                    
                    <div className="relative z-10 text-center max-w-[36rem] mx-auto">
                        <h1 className="text-5xl font-bold text-on-surface mb-6 tracking-tighter">
                            Welcome Back
                        </h1>
                        <p className="text-lg text-on-surface-variant mb-16">
                            Build faster with isolated sandboxes. Deploy, test, and scale with infrastructure that feels like magic.
                        </p>
                    </div>
                    
                    <div className="absolute bottom-8 left-0 w-full text-center px-10">
                        <p className="text-xs text-outline uppercase tracking-[0.2em] font-bold">
                            Trusted by developers shipping 10x faster
                        </p>
                    </div>
                </section>

                {/* Right Side: Login Form (45%) */}
                <section className="w-full lg:w-[45%] flex flex-col justify-center items-center px-4 md:px-10 py-16 bg-surface relative">
                    <div className="w-full max-w-[420px]">
                        <div className="mb-16 text-center">
                            <Link href="/" className="inline-flex items-center gap-1 mb-6 hover:opacity-80 transition-opacity">
                                <span className="material-symbols-outlined text-primary-fixed-dim text-[32px]" style={{ fontVariationSettings: "'FILL' 1" }}>dataset</span>
                                <span className="text-2xl font-bold text-primary-fixed-dim tracking-tight">API Sandbox</span>
                            </Link>
                            <h2 className="text-3xl font-bold text-on-surface mb-1">
                                Sign in to your workspace
                            </h2>
                            <p className="text-base text-on-surface-variant">
                                Access your projects and sandboxes
                            </p>
                        </div>
                        
                        <form className="space-y-6" onSubmit={handleLogin}>
                            {error && (
                                <div className="p-4 bg-error-container text-on-error-container border border-error/50 rounded-lg text-sm font-semibold text-center animate-in fade-in slide-in-from-top-2">
                                    {error}
                                </div>
                            )}

                            <div className="space-y-2 animate-in fade-in slide-in-from-left-4">
                                <label className="text-xs font-bold text-on-surface-variant px-1 uppercase tracking-widest" htmlFor="email">Email Address</label>
                                <div className="relative group">
                                    <input 
                                        className="w-full bg-surface-container-low border border-outline-variant/50 rounded-lg px-6 py-4 text-on-surface focus:border-primary-container focus:ring-1 focus:ring-primary-container transition-all outline-none" 
                                        id="email" 
                                        name="email" 
                                        value={email}
                                        onChange={(e) => setEmail(e.target.value)}
                                        placeholder="name@company.com" 
                                        type="email" 
                                        required 
                                    />
                                </div>
                            </div>
                            
                            <div className="space-y-2 animate-in fade-in slide-in-from-left-4 delay-75">
                                <div className="flex justify-between items-center px-1">
                                    <label className="text-xs font-bold text-on-surface-variant uppercase tracking-widest" htmlFor="password">Password</label>
                                    <Link className="text-xs font-bold text-primary-fixed-dim hover:underline uppercase tracking-widest" href="/forgot-password">Forgot Password?</Link>
                                </div>
                                <div className="relative group">
                                    <input 
                                        className="w-full bg-surface-container-low border border-outline-variant/50 rounded-lg px-6 py-4 text-on-surface focus:border-primary-container focus:ring-1 focus:ring-primary-container transition-all outline-none" 
                                        id="password" 
                                        name="password" 
                                        value={password}
                                        onChange={(e) => setPassword(e.target.value)}
                                        placeholder="••••••••" 
                                        type="password" 
                                        required 
                                    />
                                </div>
                            </div>
                            
                            <button 
                                className="w-full flex items-center justify-center gap-2 bg-primary-container text-on-primary font-bold py-4 rounded-lg glow-effect hover:opacity-90 active:scale-95 transition-all duration-200 text-lg disabled:opacity-50 disabled:active:scale-100" 
                                type="submit"
                                disabled={loading}
                            >
                                {loading ? (
                                    <>
                                        <span className="material-symbols-outlined animate-spin" style={{ fontVariationSettings: "'FILL' 1" }}>progress_activity</span>
                                        Signing In...
                                    </>
                                ) : (
                                    <>Sign In</>
                                )}
                            </button>
                        </form>
                        
                        <div className="mt-16 text-center">
                            <p className="text-base text-on-surface-variant">
                                Don't have an account?{' '}
                                <Link className="text-primary-fixed-dim font-semibold hover:underline" href="/register">Sign up</Link>
                            </p>
                        </div>
                    </div>
                    
                    <div className="lg:hidden mt-auto pt-16 text-center">
                        <p className="text-xs font-bold text-outline opacity-50 uppercase tracking-widest">
                            © 2026 API Sandbox
                        </p>
                    </div>
                </section>
            </main>
        </div>
    );
}
