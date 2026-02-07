/// <reference path="../../preload/index.d.ts" />
import React, { useEffect, useState, useRef } from 'react'
import { motion, AnimatePresence } from 'framer-motion'
import { Power, Shield, Activity, Settings, Terminal, X, Minimize2, CheckCircle2, AlertCircle } from 'lucide-react'
import clsx from 'clsx'

type AppState = 'idle' | 'starting' | 'running' | 'stopping'
type LogMessage = { message: string, timestamp?: string }
type Stats = { sent: number, recv: number, speed_sent: number, speed_recv: number }

function App() {
    const [state, setState] = useState<AppState>('idle')
    const [logs, setLogs] = useState<LogMessage[]>([])
    const [stats, setStats] = useState<Stats>({ sent: 0, recv: 0, speed_sent: 0, speed_recv: 0 })
    const [strictMode, setStrictMode] = useState(false)
    const [sysProxy, setSysProxy] = useState(false)
    const [showSettings, setShowSettings] = useState(false)

    const logsEndRef = useRef<HTMLDivElement>(null)

    useEffect(() => {
        // Listen for events from Python
        const removeListener = window.electron.on('python-event', (event: any) => {
            if (event.type === 'status') {
                setState(event.data.state)
            } else if (event.type === 'log') {
                setLogs(prev => [...prev, { message: event.data.message, timestamp: new Date().toLocaleTimeString() }])
            } else if (event.type === 'stats') {
                setStats(event.data)
            }
        })
        return () => removeListener()
    }, [])

    useEffect(() => {
        logsEndRef.current?.scrollIntoView({ behavior: 'smooth' })
    }, [logs])

    const toggleConnection = () => {
        if (state === 'idle' || state === 'stopping') {
            window.electron.send('to-python', {
                cmd: 'start',
                config: { strict: strictMode, sysproxy: sysProxy }
            })
        } else {
            window.electron.send('to-python', { cmd: 'stop' })
        }
    }

    const formatBytes = (bytes: number) => {
        if (bytes === 0) return '0 B'
        const k = 1024
        const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
        const i = Math.floor(Math.log(bytes) / Math.log(k))
        return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i]
    }

    return (
        <div className="h-screen w-screen flex flex-col bg-dark-bg text-gray-200 font-sans selection:bg-neon-green selection:text-black">
            {/* Title Bar logic handled by OS usually, but we can add Custom Drag Region */}
            <div className="h-8 bg-panel-bg flex items-center justify-between px-4 select-none" style={{ WebkitAppRegion: 'drag' } as any}>
                <div className="flex items-center gap-2">
                    <Shield className="w-4 h-4 text-neon-green" />
                    <span className="text-xs font-bold tracking-widest text-neon-green">SHADOWLINK</span>
                </div>
                {/* Window Controls could go here if frame: false */}
            </div>

            <div className="flex-1 flex flex-col p-6 gap-6 overflow-hidden">

                {/* Main Status & Control */}
                <div className="flex-1 flex flex-col items-center justify-center relative">

                    {/* Pulse Effect */}
                    <AnimatePresence>
                        {state === 'running' && (
                            <motion.div
                                initial={{ scale: 0.8, opacity: 0 }}
                                animate={{ scale: 1.5, opacity: 0 }}
                                transition={{ duration: 2, repeat: Infinity, ease: "easeOut" }}
                                className="absolute w-64 h-64 rounded-full border border-neon-green/30"
                            />
                        )}
                    </AnimatePresence>

                    <motion.button
                        whileHover={{ scale: 1.05 }}
                        whileTap={{ scale: 0.95 }}
                        onClick={toggleConnection}
                        className={clsx(
                            "relative z-10 w-48 h-48 rounded-full flex flex-col items-center justify-center transition-all duration-500 shadow-2xl border-4",
                            state === 'running'
                                ? "bg-neon-green/10 border-neon-green shadow-neon-green/20"
                                : "bg-panel-bg border-gray-700 hover:border-gray-500"
                        )}
                    >
                        <Power className={clsx("w-16 h-16 mb-2 transition-colors", state === 'running' ? "text-neon-green" : "text-gray-500")} />
                        <span className={clsx("font-bold tracking-wider", state === 'running' ? "text-neon-green" : "text-gray-500")}>
                            {state === 'running' ? 'SECURE' : state === 'starting' ? 'INITIALIZING' : 'CONNECT'}
                        </span>
                    </motion.button>

                    <div className="mt-8 grid grid-cols-2 gap-8 w-full max-w-md">
                        <div className="bg-panel-bg p-4 rounded-xl border border-white/5 flex flex-col items-center">
                            <span className="text-xs text-gray-500 mb-1">UPLOAD</span>
                            <span className="text-xl font-mono text-neon-green">{formatBytes(stats.sent)}</span>
                            <Activity className="w-4 h-4 text-neon-green/50 mt-2" />
                        </div>
                        <div className="bg-panel-bg p-4 rounded-xl border border-white/5 flex flex-col items-center">
                            <span className="text-xs text-gray-500 mb-1">DOWNLOAD</span>
                            <span className="text-xl font-mono text-blue-400">{formatBytes(stats.recv)}</span>
                            <Activity className="w-4 h-4 text-blue-400/50 mt-2" />
                        </div>
                    </div>

                </div>

                {/* Bottom Panel: Logs & Settings Toggle */}
                <div className="h-48 flex gap-4">
                    <div className="flex-1 bg-panel-bg rounded-xl border border-white/5 p-4 flex flex-col overflow-hidden relative group">
                        <div className="flex items-center justify-between mb-2">
                            <div className="flex items-center gap-2 text-gray-400">
                                <Terminal className="w-4 h-4" />
                                <span className="text-xs font-bold">SYSTEM LOGS</span>
                            </div>
                        </div>
                        <div className="flex-1 overflow-y-auto font-mono text-xs space-y-1 scrollbar-hide">
                            {logs.length === 0 && <span className="text-gray-600 italic">Ready to initialize...</span>}
                            {logs.map((log, i) => (
                                <div key={i} className="text-gray-300">
                                    <span className="text-gray-600 mr-2">[{log.timestamp}]</span>
                                    {log.message}
                                </div>
                            ))}
                            <div ref={logsEndRef} />
                        </div>
                    </div>

                    <motion.button
                        whileHover={{ scale: 1.05 }}
                        whileTap={{ scale: 0.95 }}
                        onClick={() => setShowSettings(true)}
                        className="w-16 bg-panel-bg rounded-xl border border-white/5 flex items-center justify-center hover:bg-white/5 transition-colors"
                    >
                        <Settings className="w-6 h-6 text-gray-400" />
                    </motion.button>
                </div>
            </div>

            {/* Settings Modal */}
            <AnimatePresence>
                {showSettings && (
                    <motion.div
                        initial={{ opacity: 0 }}
                        animate={{ opacity: 1 }}
                        exit={{ opacity: 0 }}
                        className="fixed inset-0 bg-black/80 backdrop-blur-sm z-50 flex items-center justify-center p-8"
                        onClick={() => setShowSettings(false)}
                    >
                        <motion.div
                            initial={{ scale: 0.9, y: 20 }}
                            animate={{ scale: 1, y: 0 }}
                            exit={{ scale: 0.9, y: 20 }}
                            className="bg-panel-bg w-full max-w-md rounded-2xl border border-white/10 p-6 shadow-2xl"
                            onClick={e => e.stopPropagation()}
                        >
                            <div className="flex items-center justify-between mb-6">
                                <h2 className="text-xl font-bold text-white flex items-center gap-2">
                                    <Settings className="w-5 h-5 text-neon-green" />
                                    Configuration
                                </h2>
                                <button onClick={() => setShowSettings(false)} className="text-gray-500 hover:text-white">
                                    <X className="w-5 h-5" />
                                </button>
                            </div>

                            <div className="space-y-4">
                                <div className="flex items-center justify-between p-4 bg-black/20 rounded-lg border border-white/5">
                                    <div>
                                        <div className="font-bold text-gray-200">Strict Mode</div>
                                        <div className="text-xs text-gray-500">Kill switch if connection drops</div>
                                    </div>
                                    <button
                                        onClick={() => setStrictMode(!strictMode)}
                                        className={clsx("w-12 h-6 rounded-full transition-colors relative", strictMode ? "bg-neon-green" : "bg-gray-700")}
                                    >
                                        <div className={clsx("absolute top-1 left-1 w-4 h-4 bg-white rounded-full transition-transform", strictMode ? "translate-x-6" : "translate-x-0")} />
                                    </button>
                                </div>

                                <div className="flex items-center justify-between p-4 bg-black/20 rounded-lg border border-white/5">
                                    <div>
                                        <div className="font-bold text-gray-200">System Proxy</div>
                                        <div className="text-xs text-gray-500">Route all system traffic</div>
                                    </div>
                                    <button
                                        onClick={() => setSysProxy(!sysProxy)}
                                        className={clsx("w-12 h-6 rounded-full transition-colors relative", sysProxy ? "bg-neon-green" : "bg-gray-700")}
                                    >
                                        <div className={clsx("absolute top-1 left-1 w-4 h-4 bg-white rounded-full transition-transform", sysProxy ? "translate-x-6" : "translate-x-0")} />
                                    </button>
                                </div>
                            </div>

                            <div className="mt-6 text-center text-xs text-gray-600">
                                v2.0.0-electron • ShadowLink Core
                            </div>
                        </motion.div>
                    </motion.div>
                )}
            </AnimatePresence>
        </div>
    )
}

export default App
