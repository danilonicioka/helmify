"use client";

import React, { useState } from 'react';
import Editor from '@monaco-editor/react';
import { 
  Rocket, 
  Settings2, 
  Download, 
  ShieldCheck, 
  Zap, 
  Package,
  CheckCircle2,
  AlertCircle,
  Loader2
} from 'lucide-react';
import { motion, AnimatePresence } from 'framer-motion';

export default function HelmifyUI() {
  const [manifest, setManifest] = useState('');
  const [chartName, setChartName] = useState('my-chart');
  const [isGenerating, setIsGenerating] = useState(false);
  const [options, setOptions] = useState({
    crd: false,
    certManager: false,
    webhook: false,
    optionalCrds: false,
  });

  const handleGenerate = async () => {
    if (!manifest) return;
    setIsGenerating(true);
    
    try {
      const response = await fetch('/v1/generate', {
        method: 'POST',
        headers: {
          'X-Chart-Name': chartName,
          'X-Crd': String(options.crd),
          'X-Cert-Manager-Subchart': String(options.certManager),
          'X-Add-Webhook-Option': String(options.webhook),
          'X-Optional-Crds': String(options.optionalCrds),
        },
        body: manifest,
      });

      if (!response.ok) throw new Error('Generation failed');

      const blob = await response.blob();
      const url = window.URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = `${chartName}.tar.gz`;
      document.body.appendChild(a);
      a.click();
      a.remove();
    } catch (err) {
      console.error(err);
      alert('Failed to generate chart. Check console for details.');
    } finally {
      setIsGenerating(false);
    }
  };

  return (
    <div className="min-h-screen bg-[#020617] text-slate-100 font-sans selection:bg-blue-500/30">
      {/* Background Orbs */}
      <div className="fixed inset-0 overflow-hidden pointer-events-none">
        <div className="absolute top-[-10%] left-[-10%] w-[40%] h-[40%] bg-blue-600/10 blur-[120px] rounded-full" />
        <div className="absolute bottom-[-10%] right-[-10%] w-[40%] h-[40%] bg-purple-600/10 blur-[120px] rounded-full" />
      </div>

      <nav className="border-b border-slate-800/50 bg-slate-950/50 backdrop-blur-md sticky top-0 z-50">
        <div className="max-w-7xl mx-auto px-6 h-16 flex items-center justify-between">
          <div className="flex items-center gap-3">
            <div className="w-10 h-10 bg-gradient-to-br from-blue-500 to-indigo-600 rounded-xl flex items-center justify-center shadow-lg shadow-blue-500/20">
              <Rocket className="text-white w-6 h-6" />
            </div>
            <div>
              <h1 className="text-xl font-bold tracking-tight">Helmify <span className="text-blue-400">Pro</span></h1>
              <p className="text-[10px] text-slate-400 uppercase tracking-widest font-semibold">Engineered by TJPA</p>
            </div>
          </div>
          <div className="flex items-center gap-6 text-sm font-medium text-slate-400">
             <div className="flex items-center gap-2 px-3 py-1 bg-blue-500/10 text-blue-400 rounded-full border border-blue-500/20">
                <ShieldCheck className="w-4 h-4" />
                <span>Production Ready</span>
             </div>
          </div>
        </div>
      </nav>

      <main className="max-w-7xl mx-auto px-6 py-8 grid grid-cols-1 lg:grid-cols-12 gap-8 relative">
        {/* Editor Section */}
        <div className="lg:col-span-8 space-y-4">
          <div className="flex items-center justify-between mb-2">
             <div className="flex items-center gap-2">
                <Package className="w-5 h-5 text-blue-400" />
                <h2 className="text-sm font-semibold text-slate-300">Kubernetes Manifests</h2>
             </div>
             <span className="text-xs text-slate-500 italic">Paste your raw YAML or Kustomize output below</span>
          </div>
          <div className="rounded-2xl overflow-hidden border border-slate-800 shadow-2xl bg-[#1e1e1e]">
            <Editor
              height="70vh"
              defaultLanguage="yaml"
              theme="vs-dark"
              value={manifest}
              onChange={(v) => setManifest(v || '')}
              options={{
                minimap: { enabled: false },
                fontSize: 14,
                fontFamily: 'JetBrains Mono, Menlo, monospace',
                padding: { top: 20 },
                lineNumbers: 'on',
                roundedSelection: true,
                scrollBeyondLastLine: false,
                automaticLayout: true,
              }}
            />
          </div>
        </div>

        {/* Sidebar / Options */}
        <aside className="lg:col-span-4 space-y-6">
          <div className="bg-slate-900/50 border border-slate-800 rounded-2xl p-6 backdrop-blur-sm sticky top-24">
            <div className="flex items-center gap-2 mb-6">
              <Settings2 className="w-5 h-5 text-blue-400" />
              <h2 className="font-bold">Chart Settings</h2>
            </div>

            <div className="space-y-6">
              <div className="space-y-2">
                <label className="text-xs font-semibold text-slate-400 uppercase tracking-wider">Chart Name</label>
                <input 
                  type="text" 
                  value={chartName}
                  onChange={(e) => setChartName(e.target.value)}
                  placeholder="e.g. portal-certidao"
                  className="w-full bg-slate-950 border border-slate-800 rounded-xl px-4 py-3 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500/50 transition-all"
                />
              </div>

              <div className="space-y-4 pt-4 border-t border-slate-800">
                <Toggle 
                  label="Separate CRD Folder" 
                  description="Places CRDs in /crds directory" 
                  checked={options.crd}
                  onChange={(v: boolean) => setOptions({...options, crd: v})}
                />
                <Toggle 
                  label="Cert-Manager Subchart" 
                  description="Include cert-manager as dependency" 
                  checked={options.certManager}
                  onChange={(v: boolean) => setOptions({...options, certManager: v})}
                />
                <Toggle 
                  label="Webhook Support" 
                  description="Inject webhook enable/disable logic" 
                  checked={options.webhook}
                  onChange={(v: boolean) => setOptions({...options, webhook: v})}
                />
                <Toggle 
                  label="Optional CRDs" 
                  description="Allow toggling CRDs via values.yaml" 
                  checked={options.optionalCrds}
                  onChange={(v: boolean) => setOptions({...options, optionalCrds: v})}
                />
              </div>

              <button
                onClick={handleGenerate}
                disabled={!manifest || isGenerating}
                className={`w-full py-4 rounded-xl font-bold flex items-center justify-center gap-2 transition-all ${
                  !manifest || isGenerating 
                    ? 'bg-slate-800 text-slate-500 cursor-not-allowed' 
                    : 'bg-blue-600 hover:bg-blue-500 text-white shadow-lg shadow-blue-600/20 active:scale-[0.98]'
                }`}
              >
                {isGenerating ? (
                  <Loader2 className="w-5 h-5 animate-spin" />
                ) : (
                  <Zap className="w-5 h-5 fill-current" />
                )}
                {isGenerating ? 'Processing...' : 'Generate Helm Chart'}
              </button>
            </div>

            <div className="mt-8 pt-6 border-t border-slate-800 space-y-4">
               <div className="flex items-start gap-3">
                  <CheckCircle2 className="w-5 h-5 text-green-500 mt-0.5" />
                  <div className="text-xs text-slate-400 leading-relaxed">
                     <span className="text-slate-200 block font-medium mb-1">TJPA Standard Compliant</span>
                     Generates tiered probes, checksum annotations, and global config inheritance automatically.
                  </div>
               </div>
               <div className="flex items-start gap-3">
                  <AlertCircle className="w-5 h-5 text-amber-500 mt-0.5" />
                  <div className="text-xs text-slate-400 leading-relaxed">
                     <span className="text-slate-200 block font-medium mb-1">Zero-Default Base</span>
                     Manifests are clean blueprints. Operational choices stay in values.yaml.
                  </div>
               </div>
            </div>
          </div>
        </aside>
      </main>

      <footer className="max-w-7xl mx-auto px-6 py-12 text-center text-slate-500 text-sm">
        <p>© 2026 Helmify Pro — Built for the Advanced Agentic Coding environment.</p>
      </footer>
    </div>
  );
}

interface ToggleProps {
  label: string;
  description: string;
  checked: boolean;
  onChange: (v: boolean) => void;
}

function Toggle({ label, description, checked, onChange }: ToggleProps) {
  return (
    <div className="flex items-center justify-between group cursor-pointer" onClick={() => onChange(!checked)}>
      <div className="space-y-0.5">
        <div className="text-sm font-medium text-slate-200">{label}</div>
        <div className="text-[11px] text-slate-500">{description}</div>
      </div>
      <div className={`w-10 h-5 rounded-full transition-colors relative ${checked ? 'bg-blue-600' : 'bg-slate-800'}`}>
        <div className={`absolute top-1 w-3 h-3 bg-white rounded-full transition-all ${checked ? 'left-6' : 'left-1'}`} />
      </div>
    </div>
  )
}
