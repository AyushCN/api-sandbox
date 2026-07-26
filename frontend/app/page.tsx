'use client'

import React, { useEffect, useRef, useState } from 'react';
import Link from 'next/link';

// Interactive Mouse Tracking Spotlight Card Component
const SpotlightCard = ({ children, className = "" }: { children: React.ReactNode, className?: string }) => {
    const divRef = useRef<HTMLDivElement>(null);
    const [isFocused, setIsFocused] = useState(false);
    const [position, setPosition] = useState({ x: 0, y: 0 });
    const [opacity, setOpacity] = useState(0);

    const handleMouseMove = (e: React.MouseEvent<HTMLDivElement>) => {
        if (!divRef.current || isFocused) return;
        const div = divRef.current;
        const rect = div.getBoundingClientRect();
        setPosition({ x: e.clientX - rect.left, y: e.clientY - rect.top });
    };

    const handleFocus = () => {
        setIsFocused(true);
        setOpacity(1);
    };

    const handleBlur = () => {
        setIsFocused(false);
        setOpacity(0);
    };

    return (
        <div 
            ref={divRef}
            onMouseMove={handleMouseMove}
            onFocus={handleFocus}
            onBlur={handleBlur}
            onMouseEnter={() => setOpacity(1)}
            onMouseLeave={() => setOpacity(0)}
            className={`relative overflow-hidden rounded-xl border border-outline-variant bg-surface-container-lowest transition-colors hover:border-primary-container/50 ${className}`}
        >
            <div 
                className="pointer-events-none absolute -inset-px opacity-0 transition duration-300 z-0"
                style={{
                    opacity,
                    background: `radial-gradient(600px circle at ${position.x}px ${position.y}px, rgba(0,240,255,0.15), transparent 40%)`,
                }}
            />
            <div className="relative z-10 h-full">
                {children}
            </div>
        </div>
    );
};

// Autonomous Floating Particle Network
const NetworkBackground = () => {
    const canvasRef = useRef<HTMLCanvasElement>(null);

    useEffect(() => {
        const canvas = canvasRef.current;
        if (!canvas) return;
        const ctx = canvas.getContext('2d');
        if (!ctx) return;

        let animationFrameId: number;
        let particles: {x: number, y: number, vx: number, vy: number, radius: number}[] = [];

        const initCanvas = () => {
            canvas.width = canvas.parentElement?.clientWidth || window.innerWidth;
            canvas.height = canvas.parentElement?.clientHeight || window.innerHeight;
            
            // Create particles
            const numParticles = Math.floor((canvas.width * canvas.height) / 15000);
            particles = [];
            for (let i = 0; i < numParticles; i++) {
                particles.push({
                    x: Math.random() * canvas.width,
                    y: Math.random() * canvas.height,
                    vx: (Math.random() - 0.5) * 1.5, // Random X velocity
                    vy: (Math.random() - 0.5) * 1.5, // Random Y velocity
                    radius: Math.random() * 2 + 1,
                });
            }
        };

        const draw = () => {
            if (!ctx || !canvas) return;
            ctx.clearRect(0, 0, canvas.width, canvas.height);

            // Update and draw particles
            for (let i = 0; i < particles.length; i++) {
                const p = particles[i];
                p.x += p.vx;
                p.y += p.vy;

                // Bounce off edges
                if (p.x < 0 || p.x > canvas.width) p.vx *= -1;
                if (p.y < 0 || p.y > canvas.height) p.vy *= -1;

                ctx.beginPath();
                ctx.arc(p.x, p.y, p.radius, 0, Math.PI * 2);
                ctx.fillStyle = '#00f0ff';
                ctx.fill();

                // Draw connecting lines
                for (let j = i + 1; j < particles.length; j++) {
                    const p2 = particles[j];
                    const dx = p.x - p2.x;
                    const dy = p.y - p2.y;
                    const dist = Math.sqrt(dx * dx + dy * dy);

                    if (dist < 150) {
                        ctx.beginPath();
                        ctx.moveTo(p.x, p.y);
                        ctx.lineTo(p2.x, p2.y);
                        ctx.lineWidth = 1;
                        // Opacity based on distance
                        ctx.strokeStyle = `rgba(0, 240, 255, ${0.2 - dist/150 * 0.2})`;
                        ctx.stroke();
                    }
                }
            }
            animationFrameId = requestAnimationFrame(draw);
        };

        initCanvas();
        draw();

        window.addEventListener('resize', initCanvas);
        return () => {
            window.removeEventListener('resize', initCanvas);
            cancelAnimationFrame(animationFrameId);
        };
    }, []);

    return (
        <canvas 
            ref={canvasRef} 
            className="absolute inset-0 w-full h-full pointer-events-none opacity-40 z-0"
        />
    );
};

export default function LandingPage() {
    useEffect(() => {
        // Simple visibility observer for fade-in animations
        const observerOptions = {
            threshold: 0.1
        };

        const observer = new IntersectionObserver((entries) => {
            entries.forEach(entry => {
                if (entry.isIntersecting) {
                    entry.target.classList.add('opacity-100', 'translate-y-0');
                    entry.target.classList.remove('opacity-0', 'translate-y-10');
                }
            });
        }, observerOptions);

        document.querySelectorAll('section > div').forEach(el => {
            el.classList.add('transition-all', 'duration-700', 'opacity-0', 'translate-y-10');
            observer.observe(el);
        });

        return () => {
            observer.disconnect();
        };
    }, []);

    // Create a double list for seamless marquee loop
    const logos = ["NODE_MODS", "REACT_CLOUD", "DOCKERLY", "VIRT_CORE", "G_FLOW", "NEXT_GEN", "SYS_ADMIN", "API_FORGE"];

    return (
        <div 
            className="bg-background text-on-surface font-sans min-h-screen selection:bg-primary-container selection:text-on-primary-container"
            style={{
                backgroundImage: 'linear-gradient(to right, #161616 1px, transparent 1px), linear-gradient(to bottom, #161616 1px, transparent 1px)',
                backgroundSize: '32px 32px'
            }}
        >
            <style jsx global>{`
                .primary-glow {
                    box-shadow: 0 0 15px rgba(0, 240, 255, 0.2);
                }
            `}</style>

            {/* Top Navigation Bar */}
            <header className="fixed top-0 w-full z-50 bg-background/80 backdrop-blur-xl border-b border-outline-variant">
                <div className="flex justify-between items-center px-4 md:px-10 py-4 max-w-[1440px] mx-auto">
                    <div className="flex items-center gap-2">
                        <span className="text-2xl font-bold text-primary tracking-tighter">Clarity Machine</span>
                    </div>
                    <nav className="hidden md:flex items-center gap-8">
                        <a className="text-base text-on-surface-variant hover:text-primary transition-colors duration-200" href="#">Features</a>
                        <a className="text-base text-on-surface-variant hover:text-primary transition-colors duration-200" href="#">How it Works</a>
                        <a className="text-base text-on-surface-variant hover:text-primary transition-colors duration-200" href="#">Pricing</a>
                    </nav>
                    <div className="flex items-center gap-4">
                        <Link href="/login" className="bg-primary-container text-on-primary-fixed-variant px-6 py-2 rounded-lg font-bold primary-glow active:scale-95 transition-all">
                            Login
                        </Link>
                    </div>
                </div>
            </header>

            <main className="pt-24">
                {/* Hero Section */}
                <section className="relative px-4 md:px-10 py-16 md:py-24 max-w-[1440px] mx-auto overflow-hidden">
                    {/* Floating Network Background Element */}
                    <NetworkBackground />
                    
                    <div className="relative z-10 flex flex-col items-center text-center max-w-[56rem] mx-auto mt-8">
                        <span className="text-xs font-bold text-primary bg-primary-container/10 px-4 py-1 rounded-full border border-primary/20 mb-6 tracking-widest uppercase shadow-[0_0_15px_rgba(0,240,255,0.2)]">
                            REVOLUTIONIZING BACKEND TESTING
                        </span>
                        <h1 className="text-5xl md:text-6xl font-bold text-primary mb-4 leading-tight tracking-tight">
                            Staging is Dead.<br/>Meet Smart Sandboxes.
                        </h1>
                        <p className="text-lg md:text-xl text-on-surface-variant max-w-[42rem] mx-auto mb-8">
                            Give your team, QA, and clients a live, secure, shareable backend environment in seconds — complete with isolated database and realistic data.
                        </p>
                        <div className="flex flex-col sm:flex-row gap-4 w-full sm:w-auto">
                            <Link href="/dashboard" className="bg-primary-container text-on-primary-fixed-variant px-8 py-4 rounded-xl font-bold text-lg primary-glow hover:brightness-110 active:scale-95 transition-all">
                                Try Demo
                            </Link>
                            <button className="bg-transparent border border-outline-variant text-white px-8 py-4 rounded-xl font-bold text-lg hover:bg-surface-container transition-all flex items-center justify-center gap-2">
                                <span className="material-symbols-outlined">play_circle</span>
                                Watch 90s Video
                            </button>
                        </div>
                        
                        {/* Infinite Marquee Logos */}
                        <div className="mt-16 w-full overflow-hidden whitespace-nowrap mask-image-fade">
                            <p className="text-xs font-bold text-on-surface-variant/50 mb-6 tracking-widest uppercase">
                                TRUSTED BY DEVELOPERS AT
                            </p>
                            <div className="inline-block animate-marquee opacity-50 grayscale hover:grayscale-0 hover:opacity-100 transition-all cursor-default">
                                {logos.concat(logos).map((logo, i) => (
                                    <span key={i} className="font-bold text-2xl tracking-tighter mx-8 inline-block">
                                        {logo}
                                    </span>
                                ))}
                            </div>
                        </div>
                    </div>
                </section>

                {/* Problem Section with Spotlight */}
                <section className="px-4 md:px-10 py-16 md:py-24 bg-surface-container-low border-y border-outline-variant">
                    <div className="max-w-[1440px] mx-auto grid grid-cols-1 md:grid-cols-2 gap-8 md:gap-16">
                        <SpotlightCard className="p-8 group">
                            <div className="absolute top-0 right-0 p-4 opacity-10 group-hover:opacity-20 transition-opacity">
                                <span className="material-symbols-outlined text-[120px]">warning</span>
                            </div>
                            <h3 className="text-2xl font-bold text-error-red mb-6 flex items-center gap-2">
                                <span className="material-symbols-outlined">history</span>
                                The Old Way
                            </h3>
                            <ul className="space-y-4 relative z-10">
                                <li className="flex items-start gap-3 text-base text-on-surface-variant">
                                    <span className="material-symbols-outlined text-error-red mt-0.5">close</span>
                                    Shared staging environments causing merge conflicts and data contamination.
                                </li>
                                <li className="flex items-start gap-3 text-base text-on-surface-variant">
                                    <span className="material-symbols-outlined text-error-red mt-0.5">close</span>
                                    Hours spent configuring Docker, DB migrations, and environment variables.
                                </li>
                                <li className="flex items-start gap-3 text-base text-on-surface-variant">
                                    <span className="material-symbols-outlined text-error-red mt-0.5">close</span>
                                    Expensive always-on infrastructure draining your cloud budget.
                                </li>
                            </ul>
                        </SpotlightCard>

                        <SpotlightCard className="p-8 group">
                            <div className="absolute top-0 right-0 p-4 opacity-10 group-hover:opacity-20 transition-opacity">
                                <span className="material-symbols-outlined text-[120px] text-primary">rocket_launch</span>
                            </div>
                            <h3 className="text-2xl font-bold text-primary mb-6 flex items-center gap-2">
                                <span className="material-symbols-outlined">bolt</span>
                                The New Way
                            </h3>
                            <ul className="space-y-4 relative z-10">
                                <li className="flex items-start gap-3 text-base text-on-primary-fixed">
                                    <span className="material-symbols-outlined text-primary mt-0.5">check_circle</span>
                                    <span className="text-on-surface">Live URLs generated in &lt; 20s for every single feature branch.</span>
                                </li>
                                <li className="flex items-start gap-3 text-base text-on-primary-fixed">
                                    <span className="material-symbols-outlined text-primary mt-0.5">check_circle</span>
                                    <span className="text-on-surface">Private, ephemeral database forked from prod with anonymized data.</span>
                                </li>
                                <li className="flex items-start gap-3 text-base text-on-primary-fixed">
                                    <span className="material-symbols-outlined text-primary mt-0.5">check_circle</span>
                                    <span className="text-on-surface">Hardened security with gVisor isolation and automatic hibernation.</span>
                                </li>
                            </ul>
                        </SpotlightCard>
                    </div>
                </section>

                {/* Features Grid with Spotlight */}
                <section className="px-4 md:px-10 py-16 md:py-24 bg-surface-container-lowest">
                    <div className="max-w-[1440px] mx-auto">
                        <h2 className="text-3xl md:text-4xl font-bold text-center mb-16 text-on-surface">From Commit to Sandbox in Seconds</h2>
                        <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
                            <SpotlightCard className="p-8">
                                <span className="material-symbols-outlined text-primary mb-4 text-3xl">fork_right</span>
                                <h4 className="text-xl font-bold mb-2 text-on-surface">Instant Forking</h4>
                                <p className="text-on-surface-variant">Fork entire environment states in a single click for parallel testing.</p>
                            </SpotlightCard>
                            
                            <SpotlightCard className="p-8">
                                <span className="material-symbols-outlined text-primary mb-4 text-3xl">account_tree</span>
                                <h4 className="text-xl font-bold mb-2 text-on-surface">Visual Blueprints</h4>
                                <p className="text-on-surface-variant">Manage complex microservices via a visual graph interface.</p>
                            </SpotlightCard>

                            <SpotlightCard className="p-8">
                                <span className="material-symbols-outlined text-primary mb-4 text-3xl">psychology</span>
                                <h4 className="text-xl font-bold mb-2 text-on-surface">AI Smart Setup</h4>
                                <p className="text-on-surface-variant">Automatically detects ports, env vars, and startup scripts.</p>
                            </SpotlightCard>
                        </div>
                    </div>
                </section>

                {/* Footer CTA */}
                <section className="px-4 md:px-10 py-24 text-center">
                    <div className="max-w-[42rem] mx-auto">
                        <h2 className="text-4xl md:text-5xl font-bold text-white mb-4">Ready to kill your staging environment?</h2>
                        <p className="text-lg text-on-surface-variant mb-12">Join 5,000+ developers automating their testing workflows with Clarity Machine.</p>
                        <div className="flex flex-col sm:flex-row gap-4 justify-center">
                            <Link href="/signup" className="bg-primary-container text-on-primary-fixed-variant px-8 py-4 rounded-xl font-bold text-lg primary-glow active:scale-95 transition-all">
                                Sign Up Free
                            </Link>
                        </div>
                    </div>
                </section>
            </main>

            {/* Footer */}
            <footer className="w-full py-12 bg-surface-container-lowest border-t border-outline-variant">
                <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-8 px-4 md:px-10 max-w-[1440px] mx-auto">
                    <div className="col-span-1 lg:col-span-1">
                        <span className="text-lg font-bold text-primary block mb-4">Clarity Machine</span>
                        <p className="text-sm text-on-surface-variant">© 2026 Clarity Machine. All rights reserved.</p>
                    </div>
                    <div className="flex flex-col gap-2">
                        <h5 className="text-xs font-bold text-white mb-2 uppercase tracking-widest">Product</h5>
                        <a className="text-sm text-on-surface-variant hover:text-primary transition-colors" href="#">Documentation</a>
                        <a className="text-sm text-on-surface-variant hover:text-primary transition-colors" href="#">API Reference</a>
                    </div>
                    <div className="flex flex-col gap-2">
                        <h5 className="text-xs font-bold text-white mb-2 uppercase tracking-widest">Company</h5>
                        <a className="text-sm text-on-surface-variant hover:text-primary transition-colors" href="#">Privacy</a>
                        <a className="text-sm text-on-surface-variant hover:text-primary transition-colors" href="#">Terms</a>
                    </div>
                    <div className="flex flex-col gap-2">
                        <h5 className="text-xs font-bold text-white mb-2 uppercase tracking-widest">Connect</h5>
                        <a className="text-sm text-on-surface-variant hover:text-primary transition-colors" href="#">Twitter / X</a>
                        <a className="text-sm text-on-surface-variant hover:text-primary transition-colors" href="#">GitHub</a>
                    </div>
                </div>
            </footer>
        </div>
    );
}
