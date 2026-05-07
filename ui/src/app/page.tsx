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
  ShieldCheck,
  PanelLeftClose,
  PanelLeftOpen,
  Download
} from 'lucide-react';
import { motion, AnimatePresence } from 'framer-motion';

export default function HelmifyUI() {
  const [manifest, setManifest] = useState('');
  const [chartName, setChartName] = useState('my-chart');
  const [isGenerating, setIsGenerating] = useState(false);
  const [isPreviewLoading, setIsPreviewLoading] = useState(false);
  const [previewFiles, setPreviewFiles] = useState<Record<string, string>>({});
  const [selectedFile, setSelectedFile] = useState<string>('values.yaml');
  const [isSidebarOpen, setIsSidebarOpen] = useState(true);

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
    <div className="h-screen bg-[#020617] text-slate-100 font-sans selection:bg-blue-500/30 overflow-hidden flex flex-col">
      {/* Background Orbs */}
      <div className="fixed inset-0 overflow-hidden pointer-events-none z-0">
        <div className="absolute top-[-10%] left-[-10%] w-[40%] h-[40%] bg-blue-600/5 blur-[120px] rounded-full" />
        <div className="absolute bottom-[-10%] right-[-10%] w-[40%] h-[40%] bg-purple-600/5 blur-[120px] rounded-full" />
      </div>

      {/* Header */}
      <nav className="border-b border-slate-800/50 bg-slate-950/80 backdrop-blur-md z-50 flex-shrink-0">
        <div className="max-w-[100%] mx-auto px-4 h-14 flex items-center justify-between">
          <div className="flex items-center gap-4">
            <button 
              onClick={() => setIsSidebarOpen(!isSidebarOpen)}
              className="p-2 hover:bg-slate-800 rounded-lg transition-colors text-slate-400 hover:text-white"
            >
              {isSidebarOpen ? <PanelLeftClose size={20} /> : <PanelLeftOpen size={20} />}
            </button>
            <div className="flex items-center gap-2">
              <div className="w-8 h-8 bg-gradient-to-br from-blue-500 to-indigo-600 rounded-lg flex items-center justify-center">
                <Rocket className="text-white w-5 h-5" />
              </div>
              <h1 className="text-lg font-bold tracking-tight hidden sm:block">Helmify <span className="text-blue-400 font-medium text-sm">Pro</span></h1>
            </div>
          </div>

          <div className="flex items-center gap-4">
             <div className="hidden lg:flex items-center gap-2 px-3 py-1 bg-blue-500/10 text-blue-400 rounded-full border border-blue-500/20 text-xs">
                <ShieldCheck size={14} />
                <span>Production Ready</span>
             </div>
             <button
                onClick={handleGenerate}
                disabled={!manifest || isGenerating}
                className={`px-4 h-9 rounded-lg font-semibold text-xs flex items-center gap-2 transition-all shadow-lg ${
                  !manifest || isGenerating 
                    ? 'bg-slate-800 text-slate-500 cursor-not-allowed' 
                    : 'bg-blue-600 hover:bg-blue-500 text-white shadow-blue-600/10 active:scale-[0.95]'
                }`}
              >
                {isGenerating ? <Loader2 size={14} className="animate-spin" /> : <Download size={14} />}
                {isGenerating ? 'Building...' : 'Download .tar.gz'}
              </button>
          </div>
        </div>
      </nav>

      <div className="flex flex-1 overflow-hidden relative z-10">
        {/* Sidebar */}
        <motion.aside 
          initial={false}
          animate={{ width: isSidebarOpen ? 280 : 0, opacity: isSidebarOpen ? 1 : 0 }}
          className="border-r border-slate-800/50 bg-slate-950/30 backdrop-blur-sm overflow-hidden flex-shrink-0"
        >
          <div className="w-[280px] p-6 space-y-8">
            <div className="space-y-4">
              <div className="flex items-center gap-2 text-slate-300">
                <Settings2 size={16} className="text-blue-400" />
                <span className="text-xs font-bold uppercase tracking-widest">Configuration</span>
              </div>
              
              <div className="space-y-2">
                <label className="text-[10px] font-bold text-slate-500 uppercase tracking-wider">Chart Name</label>
                <input 
                  type="text" 
                  value={chartName}
                  onChange={(e) => setChartName(e.target.value)}
                  className="w-full bg-slate-950/50 border border-slate-800 rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-1 focus:ring-blue-500/50 transition-all text-slate-200"
                />
              </div>
            </div>

            <div className="pt-6 border-t border-slate-800/50 space-y-4">
               <div className="flex items-start gap-3">
                  <CheckCircle2 size={16} className="text-green-500 mt-0.5 flex-shrink-0" />
                  <div className="text-[11px] text-slate-400 leading-relaxed">
                     <span className="text-slate-200 block font-semibold mb-1">TJPA STANDARD</span>
                     Tiered probes and global config inheritance are applied.
                  </div>
               </div>
               <div className="flex items-start gap-3">
                  <AlertCircle size={16} className="text-amber-500 mt-0.5 flex-shrink-0" />
                  <div className="text-[11px] text-slate-400 leading-relaxed">
                     <span className="text-slate-200 block font-semibold mb-1">BLUEPRINT MODE</span>
                     Manifests are clean blueprints. Operational choices stay in values.yaml.
                  </div>
               </div>
            </div>

            <div className="bg-blue-500/5 rounded-xl p-4 border border-blue-500/10">
               <div className="flex items-center gap-2 text-blue-400 mb-2">
                  <Zap size={14} className="fill-current" />
                  <span className="text-[10px] font-bold uppercase tracking-wider">Live Preview Active</span>
               </div>
               <p className="text-[10px] text-slate-500 leading-relaxed italic">
                 Updates happen in real-time as you refine your Kubernetes source.
               </p>
            </div>
          </div>
        </motion.aside>

        {/* Main Content (Editors) */}
        <main className="flex-1 flex overflow-hidden bg-[#020617]">
          {/* Left Editor */}
          <div className="flex-1 flex flex-col border-r border-slate-800/50">
            <div className="h-10 border-b border-slate-800/50 bg-slate-900/30 flex items-center px-4 justify-between">
              <div className="flex items-center gap-2">
                <Package size={14} className="text-blue-400" />
                <span className="text-[10px] font-bold uppercase tracking-widest text-slate-400">Source Manifest</span>
              </div>
            </div>
            <div className="flex-1 overflow-hidden">
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
                }}
              />
            </div>
          </div>

          {/* Right Editor */}
          <div className="flex-1 flex flex-col">
            <div className="h-10 border-b border-slate-800/50 bg-slate-900/30 flex items-center px-2 overflow-x-auto no-scrollbar">
              {sortedFiles.length === 0 ? (
                <div className="px-4 text-[10px] text-slate-500 italic uppercase">Preview Output</div>
              ) : (
                sortedFiles.map(file => (
                  <button
                    key={file}
                    onClick={() => setSelectedFile(file)}
                    className={`px-3 h-7 rounded-md text-[10px] font-bold transition-all whitespace-nowrap flex items-center gap-2 uppercase tracking-tighter ${
                      selectedFile === file 
                        ? 'bg-blue-600/20 text-blue-400 border border-blue-500/20' 
                        : 'text-slate-500 hover:bg-slate-800 hover:text-slate-300 border border-transparent'
                    }`}
                  >
                    <Layers size={12} />
                    {file}
                  </button>
                ))
              )}
            </div>
            <div className="flex-1 relative overflow-hidden">
              <AnimatePresence mode="wait">
                <motion.div
                  key={selectedFile}
                  initial={{ opacity: 0 }}
                  animate={{ opacity: 1 }}
                  exit={{ opacity: 0 }}
                  transition={{ duration: 0.1 }}
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
              {isPreviewLoading && (
                <div className="absolute inset-0 bg-slate-950/20 backdrop-blur-[1px] flex items-center justify-center z-20">
                  <Loader2 size={24} className="text-blue-500 animate-spin" />
                </div>
              )}
            </div>
          </div>
        </main>
      </div>
      
      {/* Minimal Footer */}
      <footer className="h-8 border-t border-slate-800/50 bg-slate-950/80 backdrop-blur-md flex items-center px-4 justify-between flex-shrink-0 text-[10px] text-slate-600">
        <div>© 2026 Helmify Pro — Advanced Agentic Coding</div>
        <div className="flex items-center gap-4">
          <span className="flex items-center gap-1"><div className="w-1.5 h-1.5 bg-green-500 rounded-full" /> API Online</span>
          <span>v1.2.0-pro</span>
        </div>
      </footer>
    </div>
  );
}
