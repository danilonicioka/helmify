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
  Download,
  Edit3,
  Folder,
  FolderOpen,
  File
} from 'lucide-react';
import { motion, AnimatePresence } from 'framer-motion';

export default function HelmifyUI() {
  const [manifest, setManifest] = useState('');
  const [chartName, setChartName] = useState('my-chart');
  const [isGenerating, setIsGenerating] = useState(false);
  const [isPreviewLoading, setIsPreviewLoading] = useState(false);
  const [previewFiles, setPreviewFiles] = useState<Record<string, string>>({});
  const [activeFile, setActiveFile] = useState<string>('source.yaml');
  const [isTemplatesOpen, setIsTemplatesOpen] = useState<boolean>(true);
  const [isSidebarOpen, setIsSidebarOpen] = useState(true);
  const [options, setOptions] = useState({
    crd: false,
    certManager: false,
    webhook: false,
    optionalCrds: false,
  });

  // Debounced preview update
  const updatePreview = useCallback(async (currentManifest: string, name: string, currentOptions: typeof options) => {
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
          'X-Crd': String(currentOptions.crd),
          'X-Cert-Manager-Subchart': String(currentOptions.certManager),
          'X-Add-Webhook-Option': String(currentOptions.webhook),
          'X-Optional-Crds': String(currentOptions.optionalCrds),
        },
        body: currentManifest,
      });

      if (response.ok) {
        const data = await response.json();
        setPreviewFiles(data);
        if (activeFile !== 'source.yaml' && !data[activeFile]) {
          const files = Object.keys(data);
          if (files.includes('values.yaml')) setActiveFile('values.yaml');
          else if (files.length > 0) setActiveFile(files[0]);
        }
      }
    } catch (err) {
      console.error('Preview failed:', err);
    } finally {
      setIsPreviewLoading(false);
    }
  }, [activeFile]);

  useEffect(() => {
    const timer = setTimeout(() => {
      updatePreview(manifest, chartName, options);
    }, 800); // Slightly longer debounce to allow for more complex manifests
    return () => clearTimeout(timer);
  }, [manifest, chartName, options, updatePreview]);

  const handleDownload = async () => {
    if (Object.keys(previewFiles).length === 0) return;
    setIsGenerating(true);
    
    try {
      const response = await fetch('/v1/download', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          chartName: chartName,
          files: previewFiles
        }),
      });

      if (!response.ok) throw new Error('Download failed');

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
      alert('Failed to download chart.');
    } finally {
      setIsGenerating(false);
    }
  };

  const handlePreviewEdit = (newValue: string | undefined) => {
    if (newValue === undefined || activeFile === 'source.yaml') return;
    setPreviewFiles(prev => ({
      ...prev,
      [activeFile]: newValue
    }));
  };

  const handleEditorChange = (newValue: string | undefined) => {
    if (newValue === undefined) return;
    if (activeFile === 'source.yaml') {
      setManifest(newValue);
    } else {
      handlePreviewEdit(newValue);
    }
  };

  const rootFiles: string[] = [];
  const templateFiles: string[] = [];
  Object.keys(previewFiles).forEach(file => {
    if (file.startsWith('templates/')) {
      templateFiles.push(file);
    } else {
      rootFiles.push(file);
    }
  });

  rootFiles.sort((a, b) => {
    if (a === 'values.yaml') return -1;
    if (b === 'values.yaml') return 1;
    if (a === 'Chart.yaml') return -1;
    if (b === 'Chart.yaml') return 1;
    return a.localeCompare(b);
  });
  templateFiles.sort();

  return (
    <div className="h-screen bg-[#020617] text-slate-100 font-sans selection:bg-blue-500/30 overflow-hidden flex flex-col">

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
              <h1 className="text-lg font-bold tracking-tight hidden sm:block">Helmify</h1>
            </div>
          </div>

          <div className="flex items-center gap-4">
             <a 
                href="/" 
                className="text-xs font-semibold text-slate-300 hover:text-white px-3 py-1.5 hover:bg-slate-800 rounded-lg transition-all flex items-center gap-1.5 border border-slate-800"
             >
                <CheckCircle2 size={14} />
                <span>Portal Home</span>
             </a>
             <a 
                href="/wizard" 
                className="text-xs font-semibold text-slate-300 hover:text-white px-3 py-1.5 hover:bg-slate-800 rounded-lg transition-all flex items-center gap-1.5 border border-slate-800"
             >
                <Layers size={14} />
                <span>Switch to Wizard</span>
             </a>
             <div className="hidden lg:flex items-center gap-2 px-3 py-1 bg-blue-500/10 text-blue-400 rounded-full border border-blue-500/20 text-xs">
                <ShieldCheck size={14} />
                <span>Production Ready</span>
             </div>
             <button
                onClick={handleDownload}
                disabled={Object.keys(previewFiles).length === 0 || isGenerating}
                className={`px-4 h-9 rounded-lg font-semibold text-xs flex items-center gap-2 transition-all shadow-lg ${
                  Object.keys(previewFiles).length === 0 || isGenerating 
                    ? 'bg-slate-800 text-slate-500 cursor-not-allowed' 
                    : 'bg-blue-600 hover:bg-blue-500 text-white shadow-blue-600/10 active:scale-[0.95]'
                }`}
              >
                {isGenerating ? <Loader2 size={14} className="animate-spin" /> : <Download size={14} />}
                {isGenerating ? 'Building...' : 'Download Chart'}
              </button>
          </div>
        </div>
      </nav>

      <div className="flex flex-col lg:flex-row flex-1 overflow-hidden relative z-10">
        {/* Sidebar */}
        <aside 
          className={`border-b lg:border-b-0 lg:border-r border-slate-800/50 bg-slate-950/30 backdrop-blur-sm overflow-y-auto flex-shrink-0 transition-all duration-300 ease-in-out ${
            isSidebarOpen ? 'w-full lg:w-[50vw] opacity-100 p-6 space-y-8' : 'w-0 opacity-0 p-0 pointer-events-none'
          }`}
        >
          <div className="w-full space-y-8 min-w-[280px]">
            <div className="space-y-4">
              <div className="flex items-center gap-3 border-b border-slate-800/50 pb-3 mb-6">
                <Settings2 size={20} className="text-blue-500" />
                <h2 className="text-base font-bold tracking-tight text-slate-100">Chart Settings</h2>
              </div>
              
              <div className="space-y-2">
                <label className="block text-xs font-semibold text-slate-400 uppercase tracking-wider">Chart Name</label>
                <input 
                  type="text" 
                  value={chartName}
                  onChange={(e) => setChartName(e.target.value)}
                  placeholder="e.g. portal-certidao"
                  className="w-full bg-slate-900 border border-slate-800 rounded-lg px-4 py-3 text-sm focus:outline-none focus:ring-1 focus:ring-blue-500/50 transition-all text-slate-200"
                />
              </div>
            </div>

            {/* Generation Options */}
            <div className="space-y-4">
              <div className="text-xs font-bold text-slate-400 uppercase tracking-wider border-b border-slate-800/50 pb-2">
                Generation Options
              </div>

              {/* Toggle 1: Separate CRD */}
              <div className="bg-slate-900/40 border border-slate-800/80 p-4 rounded-xl space-y-1">
                <div className="flex items-center justify-between">
                  <div className="pr-4">
                    <span className="text-xs font-semibold text-slate-300">Separate CRDs Folder</span>
                    <p className="text-[10px] text-slate-500 leading-relaxed">
                      Places Custom Resource Definitions in a dedicated /crds directory.
                    </p>
                  </div>
                  <label className="relative inline-flex items-center cursor-pointer select-none flex-shrink-0">
                    <input 
                      type="checkbox" 
                      checked={options.crd}
                      onChange={(e) => setOptions(prev => ({ ...prev, crd: e.target.checked }))}
                      className="sr-only peer" 
                    />
                    <div className="w-9 h-5 bg-slate-800 peer-focus:outline-none rounded-full peer peer-checked:after:translate-x-4 after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:rounded-full after:h-4 after:w-4 after:transition-all peer-checked:bg-blue-600"></div>
                  </label>
                </div>
              </div>

              {/* Toggle 2: Cert Manager */}
              <div className="bg-slate-900/40 border border-slate-800/80 p-4 rounded-xl space-y-1">
                <div className="flex items-center justify-between">
                  <div className="pr-4">
                    <span className="text-xs font-semibold text-slate-300">Cert-Manager Subchart</span>
                    <p className="text-[10px] text-slate-500 leading-relaxed">
                      Include cert-manager subchart settings for automated TLS management.
                    </p>
                  </div>
                  <label className="relative inline-flex items-center cursor-pointer select-none flex-shrink-0">
                    <input 
                      type="checkbox" 
                      checked={options.certManager}
                      onChange={(e) => setOptions(prev => ({ ...prev, certManager: e.target.checked }))}
                      className="sr-only peer" 
                    />
                    <div className="w-9 h-5 bg-slate-800 peer-focus:outline-none rounded-full peer peer-checked:after:translate-x-4 after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:rounded-full after:h-4 after:w-4 after:transition-all peer-checked:bg-blue-600"></div>
                  </label>
                </div>
              </div>

              {/* Toggle 3: Webhook */}
              <div className="bg-slate-900/40 border border-slate-800/80 p-4 rounded-xl space-y-1">
                <div className="flex items-center justify-between">
                  <div className="pr-4">
                    <span className="text-xs font-semibold text-slate-300">Webhook Support</span>
                    <p className="text-[10px] text-slate-500 leading-relaxed">
                      Inject webhook enable/disable validation logic in the templates.
                    </p>
                  </div>
                  <label className="relative inline-flex items-center cursor-pointer select-none flex-shrink-0">
                    <input 
                      type="checkbox" 
                      checked={options.webhook}
                      onChange={(e) => setOptions(prev => ({ ...prev, webhook: e.target.checked }))}
                      className="sr-only peer" 
                    />
                    <div className="w-9 h-5 bg-slate-800 peer-focus:outline-none rounded-full peer peer-checked:after:translate-x-4 after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:rounded-full after:h-4 after:w-4 after:transition-all peer-checked:bg-blue-600"></div>
                  </label>
                </div>
              </div>

              {/* Toggle 4: Optional CRDs */}
              <div className="bg-slate-900/40 border border-slate-800/80 p-4 rounded-xl space-y-1">
                <div className="flex items-center justify-between">
                  <div className="pr-4">
                    <span className="text-xs font-semibold text-slate-300">Optional CRDs</span>
                    <p className="text-[10px] text-slate-500 leading-relaxed">
                      Allow toggling CRDs installation dynamically via values.yaml.
                    </p>
                  </div>
                  <label className="relative inline-flex items-center cursor-pointer select-none flex-shrink-0">
                    <input 
                      type="checkbox" 
                      checked={options.optionalCrds}
                      onChange={(e) => setOptions(prev => ({ ...prev, optionalCrds: e.target.checked }))}
                      className="sr-only peer" 
                    />
                    <div className="w-9 h-5 bg-slate-800 peer-focus:outline-none rounded-full peer peer-checked:after:translate-x-4 after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:rounded-full after:h-4 after:w-4 after:transition-all peer-checked:bg-blue-600"></div>
                  </label>
                </div>
              </div>
            </div>

            <div className="pt-6 border-t border-slate-800/50 space-y-4">
               <div className="flex items-start gap-3">
                  <CheckCircle2 size={16} className="text-green-500 mt-0.5 flex-shrink-0" />
                  <div className="text-[11px] text-slate-400 leading-relaxed">
                     <span className="text-slate-200 block font-semibold mb-1 uppercase tracking-tight">TJPA Compliant</span>
                     Probes and annotations are generated live.
                  </div>
               </div>
               <div className="flex items-start gap-3">
                  <Edit3 size={16} className="text-blue-400 mt-0.5 flex-shrink-0" />
                  <div className="text-[11px] text-slate-400 leading-relaxed">
                     <span className="text-slate-200 block font-semibold mb-1 uppercase tracking-tight">Editable Preview</span>
                     You can edit the generated files directly before downloading.
                  </div>
               </div>
            </div>

            <div className="bg-blue-500/5 rounded-xl p-4 border border-blue-500/10">
               <div className="flex items-center gap-2 text-blue-400 mb-2">
                  <Zap size={14} className="fill-current" />
                  <span className="text-[10px] font-bold uppercase tracking-wider">Live Updates Active</span>
               </div>
               <p className="text-[10px] text-slate-500 leading-relaxed italic">
                 The preview updates as you refine your source manifest. Manual edits are preserved.
               </p>
            </div>
          </div>
        </aside>

        {/* File Explorer Sidebar */}
        <div className="w-60 bg-slate-950/50 border-r border-slate-800/50 flex flex-col flex-shrink-0 overflow-y-auto">
          {/* Section: INPUT */}
          <div className="flex items-center gap-2 px-4 py-3 text-slate-500 font-bold text-[10px] tracking-wider uppercase border-b border-slate-900/40">
            <span>Input Source</span>
          </div>
          <div className="py-2">
            <div 
              onClick={() => setActiveFile('source.yaml')}
              className={`flex items-center gap-2 px-4 py-2 text-xs font-medium cursor-pointer transition-all hover:bg-slate-800/40 ${
                activeFile === 'source.yaml' ? 'bg-blue-500/10 text-blue-400 border-l-2 border-blue-500 font-semibold' : 'text-slate-400'
              }`}
            >
              <FileCode size={14} className="text-blue-500" />
              <span>source.yaml</span>
            </div>
          </div>

          {/* Section: GENERATED HELM CHART */}
          <div className="flex items-center gap-2 px-4 py-3 text-slate-500 font-bold text-[10px] tracking-wider uppercase border-t border-b border-slate-900/40">
            <span>Generated Chart</span>
          </div>
          <div className="py-2 flex-1 flex flex-col gap-0.5">
            {Object.keys(previewFiles).length === 0 ? (
              <div className="px-4 py-3 text-[10px] text-slate-600 italic">No output generated</div>
            ) : (
              <>
                {/* Root level files */}
                {rootFiles.map(file => {
                  let icon = <FileCode size={14} className="text-slate-400" />;
                  if (file === '.helmignore') icon = <AlertCircle size={14} className="text-slate-500" />;
                  
                  return (
                    <div 
                      key={file}
                      onClick={() => setActiveFile(file)}
                      className={`flex items-center gap-2 px-4 py-2 text-xs font-medium cursor-pointer transition-all hover:bg-slate-800/40 ${
                        activeFile === file ? 'bg-blue-500/10 text-blue-400 border-l-2 border-blue-500 font-semibold' : 'text-slate-400'
                      }`}
                    >
                      {icon}
                      <span>{file}</span>
                    </div>
                  );
                })}

                {/* Templates Folder */}
                {templateFiles.length > 0 && (
                  <div>
                    <div 
                      onClick={() => setIsTemplatesOpen(!isTemplatesOpen)}
                      className="flex items-center justify-between px-4 py-2 text-xs font-medium text-slate-400 cursor-pointer hover:bg-slate-800/40"
                    >
                      <div className="flex items-center gap-2">
                        {isTemplatesOpen ? <FolderOpen size={14} className="text-blue-400" /> : <Folder size={14} className="text-blue-400" />}
                        <span>templates</span>
                      </div>
                      <ChevronRight 
                        size={12} 
                        className={`text-slate-500 transition-transform ${isTemplatesOpen ? 'rotate-90' : ''}`} 
                      />
                    </div>

                    {isTemplatesOpen && (
                      <div className="pl-4 flex flex-col gap-0.5">
                        {templateFiles.map(file => {
                          const shortName = file.substring('templates/'.length);
                          return (
                            <div 
                              key={file}
                              onClick={() => setActiveFile(file)}
                              className={`flex items-center gap-2 px-4 py-2 text-xs font-medium cursor-pointer transition-all hover:bg-slate-800/40 ${
                                activeFile === file ? 'bg-blue-500/10 text-blue-400 border-l-2 border-blue-500 font-semibold' : 'text-slate-400'
                              }`}
                            >
                              <File size={14} className="text-slate-500" />
                              <span>{shortName}</span>
                            </div>
                          );
                        })}
                      </div>
                    )}
                  </div>
                )}
              </>
            )}
          </div>
        </div>

        {/* Main Content (Single Active Editor) */}
        <main className="flex-1 flex flex-col overflow-hidden bg-[#020617]">
          <div className="h-10 border-b border-slate-800/50 bg-slate-900/30 flex items-center justify-between px-4 flex-shrink-0">
            <div className="flex items-center gap-2">
              <Package size={14} className="text-blue-400" />
              <span className="text-[10px] font-bold uppercase tracking-widest text-slate-400">
                {activeFile === 'source.yaml' ? 'Kubernetes Source Editor' : `Generated File: ${activeFile}`}
              </span>
            </div>
            {activeFile !== 'source.yaml' && (
              <span className="text-[9px] bg-slate-800 text-slate-400 px-2 py-0.5 rounded font-mono border border-slate-700/50">
                EDITABLE PREVIEW
              </span>
            )}
          </div>
          <div className="flex-1 relative overflow-hidden">
            <AnimatePresence mode="wait">
              <motion.div
                key={activeFile}
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
                  value={activeFile === 'source.yaml' ? manifest : (previewFiles[activeFile] || '')}
                  onChange={handleEditorChange}
                  options={{
                    readOnly: false,
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
              </motion.div>
            </AnimatePresence>
            {isPreviewLoading && (
              <div className="absolute inset-0 bg-slate-950/20 backdrop-blur-[1px] flex items-center justify-center z-20">
                <Loader2 size={24} className="text-blue-500 animate-spin" />
              </div>
            )}
          </div>
        </main>
      </div>
      
      {/* Minimal Footer */}
      <footer className="h-8 border-t border-slate-800/50 bg-slate-950/80 backdrop-blur-md flex items-center px-4 justify-between flex-shrink-0 text-[10px] text-slate-600">
        <div>© 2026 Helmify — Advanced Agentic Coding</div>
        <div className="flex items-center gap-4">
          <span className="flex items-center gap-1"><div className="w-1.5 h-1.5 bg-green-500 rounded-full" /> API Online</span>
          <span>v1.3.0</span>
        </div>
      </footer>
    </div>
  );
}
