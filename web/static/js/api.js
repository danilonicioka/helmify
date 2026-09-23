        // Initialize state
        async function init() {
            try {
                // Fetch available subcomponents
                const subRes = await fetch('/v1/subcomponents');
                if (subRes.ok) {
                    const subs = await subRes.json();
                    const container = document.getElementById('subcomponentsContainer');
                    if (container && subs) {
                        container.innerHTML = '';
                        subs.forEach(sub => {
                            const label = document.createElement('label');
                            label.style = 'display: flex; align-items: center; gap: 8px; font-size: 14px; font-weight: 400; cursor: pointer; text-transform: capitalize;';
                            label.innerHTML = `
                                <input type="checkbox" name="subcomponent" value="${sub}" onchange="saveAndPreview()" style="width: 16px; height: 16px; accent-color: var(--accent);">
                                Include ${sub}
                            `;
                            container.appendChild(label);
                        });
                    }
                }

                const response = await fetch(`/v1/defaults`);
                if (!response.ok) {
                    throw new Error("Failed to fetch defaults");
                }
                const defaults = await response.json();

                // Extract global configuration
                globalConfig = (defaults.global && defaults.global.config && defaults.global.config.env) || { 'TZ': 'America/Belem' };
                globalSecret = (defaults.global && defaults.global.secrets && defaults.global.secrets.env) || {};

                // Extract components
                components = {};
                let compKeys = [];
                const deploysSource = defaults.deploys || defaults;
                const cronjobsSource = defaults.cronjobs || {};
                const allComps = { ...deploysSource, ...cronjobsSource };
                
                Object.entries(allComps).forEach(([key, val]) => {
                    if (val && typeof val === 'object' && val.image) {
                        components[key] = parseComponentConfig(val);
                        if (cronjobsSource[key]) {
                            components[key].workloadType = 'CronJob';
                            components[key].schedule = val.schedule || '';
                        }
                        compKeys.push(key);
                    }
                });

                let oldKey = 'my-app';
                if (compKeys.length > 0) {
                    oldKey = compKeys[0];
                }

                if (!chartNameManuallyEdited) {
                    document.getElementById('chartName').value = oldKey;
                }

                activeComponent = compKeys.includes('api') ? 'api' : (compKeys[0] || 'api');
                document.getElementById('btn-add-comp').style.display = 'block';

            } catch (err) {
                console.error("Error loading defaults:", err);
                fallbackInit();
            }

            renderGlobalConfigList();
            renderTabs();
            renderActiveComponent();
            updateUIForChartType();
            updatePreview();
        }
        function fallbackInit() {
            if (!chartNameManuallyEdited) {
                document.getElementById('chartName').value = 'my-app';
            }
            components = {
                'api': createDefaultComponentConfig('api', 'api'),
                'app': createDefaultComponentConfig('app', 'app')
            };
            activeComponent = 'api';
            document.getElementById('btn-add-comp').style.display = 'block';
        }
        // Update real-time preview
        async function updatePreview() {
            const payload = buildPayload();

            try {
                const response = await fetch('/v1/preview-wizard', {
                    method: 'POST',
                    headers: {
                        'Content-Type': 'application/json'
                    },
                    body: JSON.stringify(payload)
                });

                if (response.ok) {
                    generatedFiles = await response.json();

                    // Render the file tree
                    renderFileTree();

                    // Render the currently selected file content
                    if (isFirstLoad) {
                        // Find first available file (prefer values.yaml)
                        if (generatedFiles['values.yaml']) {
                            activeFile = 'values.yaml';
                        } else {
                            const keys = Object.keys(generatedFiles);
                            if (keys.length > 0) activeFile = keys[0];
                        }
                        isFirstLoad = false;
                    }

                    renderActiveFileContent();
                } else {
                    const text = await response.text();
                    console.error("Preview failed:", text);
                }
            } catch (err) {
                console.error("Preview error:", err);
            }
        }
        document.getElementById('btn-generate-header').addEventListener('click', async () => {
            const payload = buildPayload();
            const btn = document.getElementById('btn-generate-header');
            const originalText = btn.innerHTML;

            btn.disabled = true;
            btn.innerHTML = `<span style="display:inline-block; border: 2px solid #fff; border-top: 2px solid transparent; border-radius:50%; width:12px; height:12px; animation: spin 1s linear infinite; margin-right: 6px;"></span> Downloading...`;

            try {
                const response = await fetch('/v1/generate-wizard', {
                    method: 'POST',
                    headers: {
                        'Content-Type': 'application/json'
                    },
                    body: JSON.stringify(payload)
                });

                if (!response.ok) {
                    const text = await response.text();
                    throw new Error(text || 'Failed to generate Helm chart.');
                }

                // Trigger file download
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



function createDefaultComponentConfig(name, type) {
    return {
        type: type,
        enabled: true,
        image: { repository: name, tag: "latest" },
        service: {
            ports: {
                http: { port: 8080 }
            }
        },
        route: { 
            path: "",
            default: { enabled: true, host: "" },
            internal: { enabled: false, host: "" },
            external: { enabled: false, host: "" }
        },
        resources: { limits: { cpu: "100m", memory: "128Mi" }, requests: { cpu: "10m", memory: "64Mi" } },
        autoscaling: {
            enabled: false,
            engine: "",
            keda: { minReplicas: 3, maxReplicas: 20 },
            hpa: { minReplicas: 1, maxReplicas: 2, metrics: [ { resource: { target: { averageUtilization: 180 } } } ] }
        },
        vso: { enabled: false },
        persistence: { enabled: false }
    };
}
