        const defaultManifest = `apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-app
  labels:
    app: my-app
spec:
  replicas: 2
  selector:
    matchLabels:
      app: my-app
  template:
    metadata:
      labels:
        app: my-app
    spec:
      containers:
      - name: my-app
        image: "{{REGISTRY}}/my-app:1.0.0"
        ports:
        - containerPort: 8080
        env:
        - name: SPRING_PROFILES_ACTIVE
          value: prd
---
apiVersion: v1
kind: Service
metadata:
  name: my-app
  labels:
    app: my-app
spec:
  ports:
  - port: 8080
    targetPort: 8080
  selector:
    app: my-app
`;

        let activeFile = 'source.yaml';
        let isFirstLoad = true;
        let templatesFolderOpen = true;
        let generatedFiles = {
            'source.yaml': defaultManifest
        };
        let previewLoading = false;

        // Initialize state
        function init() {
            renderFileTree();
            updatePreview();
        }

        // Debounce updates
        let debounceTimer;
        function debouncedUpdatePreview() {
            clearTimeout(debounceTimer);
            debounceTimer = setTimeout(updatePreview, 800);
        }

        async function updatePreview() {
            const chartName = document.getElementById('chartName').value.trim() || 'my-chart';
            const body = generatedFiles['source.yaml'] || '';
            if (!body) {
                generatedFiles = { 'source.yaml': '' };
                renderFileTree();
                renderActiveFileContent();
                return;
            }

            previewLoading = true;
            const spinner = document.getElementById('preview-spinner');
            if (spinner) spinner.style.display = 'inline-block';

            const headers = {
                'X-Chart-Name': chartName,
                'X-Crd': 'false',
                'X-Cert-Manager-Subchart': 'false',
                'X-Add-Webhook-Option': 'false',
                'X-Optional-Crds': 'false',
                'X-Generate-All-Templates': document.getElementById('optGenerateAllTemplates').checked ? 'true' : 'false',
                'X-Dev-Repo-Url': document.getElementById('devRepoUrl').value.trim()
            };

            try {
                const response = await fetch('/v1/preview', {
                    method: 'POST',
                    headers: headers,
                    body: body
                });

                if (response.ok) {
                    const data = await response.json();
                    
                    const oldSource = generatedFiles['source.yaml'];
                    generatedFiles = data;
                    generatedFiles['source.yaml'] = oldSource;

                    renderFileTree();
                    
                    const previousActiveFile = activeFile;
                    let initialLoad = false;
                    if (isFirstLoad) {
                        activeFile = 'source.yaml';
                        isFirstLoad = false;
                        initialLoad = true;
                    } else if (activeFile !== 'source.yaml' && !generatedFiles[activeFile]) {
                        activeFile = 'source.yaml';
                    }
                    
                    if (initialLoad || activeFile !== 'source.yaml' || previousActiveFile !== activeFile) {
                        renderActiveFileContent();
                    }
                } else {
                    const text = await response.text();
                    console.error("Preview failed:", text);
                }
            } catch (err) {
                console.error("Preview error:", err);
            } finally {
                previewLoading = false;
                if (spinner) spinner.style.display = 'none';
                
                // If the very first load fails (e.g. invalid default manifest),
                // ensure we still render the textarea so the user isn't stuck on the placeholder.
                if (isFirstLoad) {
                    isFirstLoad = false;
                    activeFile = 'source.yaml';
                    renderActiveFileContent();
                }
            }
        }

        function renderFileTree() {
            // Render Input Source Tree
            const inputTree = document.getElementById('ide-input-tree');
            inputTree.innerHTML = '';
            
            const sourceFile = document.createElement('div');
            sourceFile.className = `tree-file ${activeFile === 'source.yaml' ? 'active' : ''}`;
            sourceFile.innerHTML = `<span style="font-size: 0.95rem; margin-right: 0.2rem;">📥</span> <span>source.yaml</span>`;
            sourceFile.onclick = () => selectFile('source.yaml');
            inputTree.appendChild(sourceFile);

            // Render Generated Chart Tree
            const generatedTree = document.getElementById('ide-generated-tree');
            generatedTree.innerHTML = '';

            const rootFiles = [];
            const templateFiles = [];

            Object.keys(generatedFiles).forEach(path => {
                if (path === 'source.yaml') return;
                if (path.startsWith('templates/')) {
                    templateFiles.push(path);
                } else {
                    rootFiles.push(path);
                }
            });

            // Sort lists
            rootFiles.sort((a, b) => {
                if (a === 'values.yaml') return -1;
                if (b === 'values.yaml') return 1;
                if (a === 'Chart.yaml') return -1;
                if (b === 'Chart.yaml') return 1;
                return a.localeCompare(b);
            });
            templateFiles.sort();

            // Render root files
            rootFiles.forEach(path => {
                const fileEl = createFileNode(path, path);
                generatedTree.appendChild(fileEl);
            });

            // Render templates folder
            if (templateFiles.length > 0) {
                const folderEl = document.createElement('div');
                folderEl.className = 'tree-folder';
                
                const headerEl = document.createElement('div');
                headerEl.className = 'tree-folder-header';
                headerEl.innerHTML = `<span class="arrow">${templatesFolderOpen ? '&#9662;' : '&#9656;'}</span> <span style="font-size: 0.95rem; margin-right: 0.2rem;">📁</span> <span>templates</span>`;
                headerEl.onclick = () => {
                    templatesFolderOpen = !templatesFolderOpen;
                    renderFileTree();
                };
                folderEl.appendChild(headerEl);

                const contentEl = document.createElement('div');
                contentEl.className = 'tree-folder-content';
                contentEl.style.display = templatesFolderOpen ? 'flex' : 'none';

                templateFiles.forEach(path => {
                    const shortName = path.substring('templates/'.length);
                    const fileEl = createFileNode(path, shortName);
                    contentEl.appendChild(fileEl);
                });

                folderEl.appendChild(contentEl);
                generatedTree.appendChild(folderEl);
            }
        }

        function createFileNode(path, displayName) {
            const fileEl = document.createElement('div');
            fileEl.className = `tree-file ${activeFile === path ? 'active' : ''}`;
            
            let icon = '📄';
            if (path.endsWith('.yaml') || path.endsWith('.yml')) icon = '📝';
            if (path.endsWith('.tpl')) icon = '⚙️';
            if (path === '.helmignore') icon = '🚫';

            fileEl.innerHTML = `<span style="font-size: 0.95rem; margin-right: 0.2rem;">${icon}</span> <span>${displayName}</span>`;
            fileEl.onclick = () => selectFile(path);
            return fileEl;
        }

        function selectFile(path) {
            activeFile = path;
            renderFileTree();
            renderActiveFileContent();
        }

        function renderActiveFileContent() {
            const activeFilenameSpan = document.getElementById('active-filename');
            const editorBadge = document.getElementById('editor-badge');
            const editorBody = document.getElementById('ide-editor-body');

            if (!activeFile) {
                activeFilenameSpan.textContent = 'Select a file';
                editorBadge.style.display = 'none';
                editorBody.innerHTML = `
                    <div class="ide-placeholder">
                        <svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><rect width="18" height="18" x="3" y="3" rx="2"></rect><path d="M9 3v18"></path><path d="m16 15-3-3 3-3"></path></svg>
                        <p>No file selected</p>
                    </div>
                `;
                return;
            }

            activeFilenameSpan.textContent = activeFile;
            editorBadge.style.display = activeFile !== 'source.yaml' ? 'inline-block' : 'none';

            const code = generatedFiles[activeFile] || '';

            editorBody.innerHTML = `
                <pre class="ide-line-numbers" id="line-numbers"></pre>
                <textarea id="code-editor" class="ide-code-textarea" spellcheck="false" wrap="off"></textarea>
            `;

            const textarea = document.getElementById('code-editor');
            textarea.value = code;

            // Sync scrolling
            textarea.onscroll = () => {
                const lineNums = document.getElementById('line-numbers');
                if (lineNums) lineNums.scrollTop = textarea.scrollTop;
            };

            // Sync line numbers
            const updateLineNums = () => {
                const lineNums = document.getElementById('line-numbers');
                if (lineNums) {
                    const lines = textarea.value.split('\n').length;
                    let html = '';
                    for (let i = 1; i <= lines; i++) {
                        html += `${i}\n`;
                    }
                    lineNums.textContent = html;
                }
            };
            updateLineNums();

            textarea.oninput = (e) => {
                const val = e.target.value;
                generatedFiles[activeFile] = val;
                updateLineNums();

                if (activeFile === 'source.yaml') {
                    debouncedUpdatePreview();
                }
            };

            // Support Tab key inside textarea
            textarea.onkeydown = (e) => {
                if (e.key === 'Tab') {
                    e.preventDefault();
                    const start = textarea.selectionStart;
                    const end = textarea.selectionEnd;
                    const val = textarea.value;
                    textarea.value = val.substring(0, start) + '  ' + val.substring(end);
                    textarea.selectionStart = textarea.selectionEnd = start + 2;
                    textarea.dispatchEvent(new Event('input'));
                }
            };
        }

        // Sidebar Toggle Script
        const btnToggleSidebar = document.getElementById('btn-toggle-sidebar');
        const settingsSidebar = document.getElementById('settings-sidebar');
        let isSidebarOpen = true;

        if (btnToggleSidebar && settingsSidebar) {
            btnToggleSidebar.addEventListener('click', () => {
                isSidebarOpen = !isSidebarOpen;
                if (isSidebarOpen) {
                    settingsSidebar.classList.remove('collapsed');
                    btnToggleSidebar.classList.remove('collapsed');
                } else {
                    settingsSidebar.classList.add('collapsed');
                    btnToggleSidebar.classList.add('collapsed');
                }
            });
        }

        // Event listeners for settings change
        document.getElementById('optGenerateAllTemplates').addEventListener('change', debouncedUpdatePreview);
        document.getElementById('chartName').addEventListener('input', debouncedUpdatePreview);
        document.getElementById('devRepoUrl').addEventListener('input', debouncedUpdatePreview);

        // Handle Download / Form Submission
        document.getElementById('btn-download').addEventListener('click', async () => {
            const btn = document.getElementById('btn-download');
            const originalText = btn.innerHTML;
            const chartName = document.getElementById('chartName').value.trim() || 'my-chart';

            // Filter out source.yaml from the download payload, only send generated files
            const filesPayload = {};
            Object.entries(generatedFiles).forEach(([name, content]) => {
                if (name !== 'source.yaml') {
                    filesPayload[name] = content;
                }
            });

            if (Object.keys(filesPayload).length === 0) {
                alert("Nothing to download yet.");
                return;
            }

            btn.disabled = true;
            btn.innerHTML = `<span style="display:inline-block; border: 2px solid #fff; border-top: 2px solid transparent; border-radius:50%; width:12px; height:12px; animation: spin 1s linear infinite; margin-right: 6px;"></span> Downloading...`;

            try {
                const response = await fetch('/v1/download', {
                    method: 'POST',
                    headers: {
                        'Content-Type': 'application/json'
                    },
                    body: JSON.stringify({
                        chartName: chartName,
                        files: filesPayload
                    })
                });

                if (!response.ok) {
                    const text = await response.text();
                    throw new Error(text || 'Failed to download Helm chart.');
                }

                const blob = await response.blob();
                const url = window.URL.createObjectURL(blob);
                const a = document.createElement('a');
                a.href = url;
                a.download = 'chart.tar.gz';
                document.body.appendChild(a);
                a.click();
                a.remove();
                window.URL.revokeObjectURL(url);

                showToast();

            } catch (err) {
                alert(`Error: ${err.message}`);
            } finally {
                btn.disabled = false;
                btn.innerHTML = originalText;
            }
        });

        function showToast() {
            const toast = document.getElementById('toast-message');
            toast.style.display = 'block';
            setTimeout(() => {
                toast.style.display = 'none';
            }, 3000);
        }

        // Run on load
        window.addEventListener('DOMContentLoaded', init);
