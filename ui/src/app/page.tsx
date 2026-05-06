"use client";

import React, { useState, useEffect, useCallback } from 'react';
import Editor from '@monaco-editor/react';
import { 
  Rocket, 
  Settings2, 
  Zap, 
  Package,
  CheckCircle2,
  AlertCircle,
  Loader2,
  FileCode,
  Layers,
  ChevronRight,
  ShieldCheck
} from 'lucide-react';
import { motion, AnimatePresence } from 'framer-motion';

export default function HelmifyUI() {
  const [manifest, setManifest] = useState('');
  const [chartName, setChartName] = useState('my-chart');
  const [isGenerating, setIsGenerating] = useState(false);
  const [isPreviewLoading, setIsPreviewLoading] = useState(false);
  const [previewFiles, setPreviewFiles] = useState<Record<string, string>>({});
  const [selectedFile, setSelectedFile] = useState<string>('values.yaml');

  // Debounced preview update
  const updatePreview = useCallback(async (currentManifest: string, name: string) => {
    if (!currentManifest) {
      setPreviewFiles({});
      return;
    }
    
    setIsPreviewLoading(true);
    try {
      const response = await fetch('/v1/preview', {
        method: 'POST',
        headers: {
          'X-Chart-Name': name,
          'X-Crd': 'false',
          'X-Cert-Manager-Subchart': 'false',
          'X-Add-Webhook-Option': 'false',
          'X-Optional-Crds': 'false',
        },
        body: currentManifest,
      });

      if (response.ok) {
        const data = await response.json();
        setPreviewFiles(data);
        // Reset selected file if it's not in the new set
        if (!data[selectedFile]) {
          const files = Object.keys(data);
          if (files.includes('values.yaml')) setSelectedFile('values.yaml');
          else if (files.length > 0) setSelectedFile(files[0]);
        }
      }
    } catch (err) {
      console.error('Preview failed:', err);
    } finally {
      setIsPreviewLoading(false);
    }
  }, [selectedFile]);

  useEffect(() => {
    const timer = setTimeout(() => {
      updatePreview(manifest, chartName);
    }, 600);
    return () => clearTimeout(timer);
  }, [manifest, chartName, updatePreview]);

  const handleGenerate = async () => {
    if (!manifest) return;
    setIsGenerating(true);
    
    try {
      const response = await fetch('/v1/generate', {
        method: 'POST',
        headers: {
          'X-Chart-Name': chartName,
          'X-Crd': 'false',
          'X-Cert-Manager-Subchart': 'false',
          'X-Add-Webhook-Option': 'false',
          'X-Optional-Crds': 'false',
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

  const sortedFiles = Object.keys(previewFiles).sort((a, b) => {
    if (a === 'values.yaml') return -1;
    if (b === 'values.yaml') return 1;
    if (a === 'Chart.yaml') return -1;
    if (b === 'Chart.yaml') return 1;
    return a.localeCompare(b);
  });

  return (
    <div className="min-h-screen bg-[#020617] text-slate-100 font-sans selection:bg-blue-500/30 overflow-x-hidden">
      {/* Background Orbs */}
      <div className="fixed inset-0 overflow-hidden pointer-events-none">
        <div className="absolute top-[-10%] left-[-10%] w-[40%] h-[40%] bg-blue-600/10 blur-[120px] rounded-full" />
        <div className="absolute bottom-[-10%] right-[-10%] w-[40%] h-[40%] bg-purple-600/10 blur-[120px] rounded-full" />
      </div>

      <nav className="border-b border-slate-800/50 bg-slate-950/50 backdrop-blur-md sticky top-0 z-50">
        <div className="max-w-[1600px] mx-auto px-6 h-16 flex items-center justify-between">
          <div className="flex items-center gap-3">
            <div className="w-10 h-10 bg-gradient-to-br from-blue-500 to-indigo-600 rounded-xl flex items-center justify-center shadow-lg shadow-blue-500/20">
              <Rocket className="text-white w-6 h-6" />
            </div>
            <div>
              <h1 className="text-xl font-bold tracking-tight">Helmify <span className="text-blue-400">Pro</span></h1>
              <p className="text-[10px] text-slate-400 uppercase tracking-widest font-semibold">Engineered by TJPA</p>
            </div>
          </div>
          <div className="hidden md:flex items-center gap-6 text-sm font-medium text-slate-400">
             <div className="flex items-center gap-2 px-3 py-1 bg-blue-500/10 text-blue-400 rounded-full border border-blue-500/20">
                <ShieldCheck className="w-4 h-4" />
                <span>Production Ready</span>
             </div>
          </div>
        </div>
      </nav>

      <main className="max-w-[1600px] mx-auto px-6 py-6 grid grid-cols-1 lg:grid-cols-12 gap-6 relative">
        {/* Input Section */}
        <div className="lg:col-span-5 space-y-4">
          <div className="flex items-center justify-between">
             <div className="flex items-center gap-2">
                <Package className="w-5 h-5 text-blue-400" />
                <h2 className="text-sm font-semibold text-slate-300 uppercase tracking-wider">Input Manifest</h2>
             </div>
             <span className="text-[10px] text-slate-500 font-mono uppercase">YAML / K8S</span>
          </div>
          <div className="rounded-2xl overflow-hidden border border-slate-800 shadow-2xl bg-[#0f172a] h-[75vh]">
            <Editor
              height="100%"
              defaultLanguage="yaml"
              theme="vs-dark"
              value={manifest}
              onChange={(v) => setManifest(v || '')}
              options={{
                minimap: { enabled: false },
                fontSize: 13,
                fontFamily: 'JetBrains Mono, Menlo, monospace',
                padding: { top: 16 },
                lineNumbers: 'on',
                roundedSelection: true,
                scrollBeyondLastLine: false,
                automaticLayout: true,
                backgroundColor: '#0f172a'
              }}
            />
          </div>
        </div>

        {/* Preview Section */}
        <div className="lg:col-span-4 space-y-4">
          <div className="flex items-center justify-between">
             <div className="flex items-center gap-2">
                <FileCode className="w-5 h-5 text-purple-400" />
                <h2 className="text-sm font-semibold text-slate-300 uppercase tracking-wider">Live Preview</h2>
             </div>
             {isPreviewLoading && <Loader2 className="w-4 h-4 text-blue-400 animate-spin" />}
          </div>
          
          <div className="flex flex-col rounded-2xl overflow-hidden border border-slate-800 shadow-2xl bg-[#0f172a] h-[75vh]">
            {/* File Selector Tabs */}
            <div className="flex items-center gap-1 p-2 bg-slate-900/80 border-b border-slate-800 overflow-x-auto scrollbar-hide">
              {sortedFiles.length === 0 ? (
                <div className="px-3 py-1 text-xs text-slate-500 italic">No output generated yet</div>
              ) : (
                sortedFiles.map(file => (
                  <button
                    key={file}
                    onClick={() => setSelectedFile(file)}
                    className={`px-3 py-1.5 rounded-lg text-[11px] font-medium transition-all whitespace-nowrap flex items-center gap-2 ${
                      selectedFile === file 
                        ? 'bg-blue-600/20 text-blue-400 border border-blue-500/30' 
                        : 'text-slate-500 hover:bg-slate-800 hover:text-slate-300 border border-transparent'
                    }`}
                  >
                    <Layers className="w-3 h-3" />
                    {file}
                  </button>
                ))
              )}
            </div>

            <div className="flex-1">
              <AnimatePresence mode="wait">
                <motion.div
                  key={selectedFile}
                  initial={{ opacity: 0, x: 10 }}
                  animate={{ opacity: 1, x: 0 }}
                  exit={{ opacity: 0, x: -10 }}
                  transition={{ duration: 0.15 }}
                  className="h-full"
                >
                  <Editor
                    height="100%"
                    language="yaml"
                    theme="vs-dark"
                    value={previewFiles[selectedFile] || ''}
                    options={{
                      readOnly: true,
                      minimap: { enabled: false },
                      fontSize: 12,
                      fontFamily: 'JetBrains Mono, Menlo, monospace',
                      padding: { top: 16 },
                      lineNumbers: 'on',
                      roundedSelection: true,
                      scrollBeyondLastLine: false,
                      automaticLayout: true,
                    }}
                  />
                </motion.div>
              </AnimatePresence>
            </div>
          </div>
        </div>

        {/* Sidebar / Options */}
        <aside className="lg:col-span-3 space-y-6">
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

              <div className="py-4 border-t border-slate-800">
                <div className="bg-blue-500/5 rounded-xl p-4 border border-blue-500/10">
                   <div className="flex items-center gap-2 text-blue-400 mb-2">
                      <Zap className="w-4 h-4 fill-current" />
                      <span className="text-[11px] font-bold uppercase tracking-wider">Live Mode Active</span>
                   </div>
                   <p className="text-[10px] text-slate-400 leading-relaxed">
                     Previews are generated automatically as you edit. Use the download button for the final chart bundle.
                   </p>
                </div>
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
                  <ChevronRight className="w-5 h-5" />
                )}
                {isGenerating ? 'Processing...' : 'Download Full Chart'}
              </button>
            </div>

            <div className="mt-8 pt-6 border-t border-slate-800 space-y-4">
               <div className="flex items-start gap-3">
                  <CheckCircle2 className="w-5 h-5 text-green-500 mt-0.5" />
                  <div className="text-xs text-slate-400 leading-relaxed">
                     <span className="text-slate-200 block font-medium mb-1 uppercase tracking-tight">Standard Compliant</span>
                     Tiered probes and global config inheritance are applied live.
                  </div>
               </div>
               <div className="flex items-start gap-3">
                  <AlertCircle className="w-5 h-5 text-amber-500 mt-0.5" />
                  <div className="text-xs text-slate-400 leading-relaxed">
                     <span className="text-slate-200 block font-medium mb-1 uppercase tracking-tight">Zero-Default</span>
                     Operational choices stay in values.yaml. Check them on the left.
                  </div>
               </div>
            </div>
          </div>
        </aside>
      </main>

      <footer className="max-w-[1600px] mx-auto px-6 py-12 text-center text-slate-500 text-sm">
        <p>© 2026 Helmify Pro — Built for the Advanced Agentic Coding environment.</p>
      </footer>
    </div>
  );
}
