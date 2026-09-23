function toggleWorkloadTypeFields() {
            const workloadType = document.getElementById('compWorkloadType').value;
            const scheduleField = document.getElementById('scheduleField');
            if (workloadType === 'CronJob') {
                scheduleField.style.display = 'block';
            } else {
                scheduleField.style.display = 'none';
            }
        }
        
        // Watch chart name changes for single deployment key renaming
        document.getElementById('chartName').addEventListener('input', (e) => {
            chartNameManuallyEdited = true;
            const newName = e.target.value.trim() || 'chart';
            updatePreview();
        });

        // Render Tabs
        function renderTabs() {
            const tabsContainer = document.getElementById('components-tabs');
            tabsContainer.innerHTML = '';

            Object.keys(components).forEach(name => {
                const tab = document.createElement('button');
                tab.type = 'button';
                tab.className = `tab-btn ${activeComponent === name ? 'active' : ''}`;
                tab.innerHTML = `<span>${name}</span>`;

                // Allow removing tabs in multi type
                if (Object.keys(components).length > 1) {
                    const removeBtn = document.createElement('button');
                    removeBtn.type = 'button';
                    removeBtn.className = 'remove-comp-btn';
                    removeBtn.innerHTML = '&times;';
                    removeBtn.onclick = (e) => {
                        e.stopPropagation();
                        delete components[name];
                        if (activeComponent === name) {
                            activeComponent = Object.keys(components)[0];
                        }
                        renderTabs();
                        renderActiveComponent();
                        updatePreview();
                    };
                    tab.appendChild(removeBtn);
                }

                tab.addEventListener('click', () => {
                    saveActiveComponentState();
                    activeComponent = name;
                    renderTabs();
                    renderActiveComponent();
                });

                tabsContainer.appendChild(tab);
            });
        }

        // Add component in multi mode - Custom Modal
        const addModal = document.getElementById('add-deployment-modal');
        const modalInput = document.getElementById('modal-comp-name');
        const modalError = document.getElementById('modal-comp-error');
        const modalConfirmBtn = document.getElementById('modal-btn-confirm');
        const modalCancelBtn = document.getElementById('modal-btn-cancel');

        let modalPreset = 'api';
        const presetBackendBtn = document.getElementById('modal-preset-api');
        const presetFrontendBtn = document.getElementById('modal-preset-app');

        presetBackendBtn.addEventListener('click', () => {
            modalPreset = 'api';
            presetBackendBtn.classList.add('active');
            presetFrontendBtn.classList.remove('active');
        });

        presetFrontendBtn.addEventListener('click', () => {
            modalPreset = 'app';
            presetFrontendBtn.classList.add('active');
            presetBackendBtn.classList.remove('active');
        });

        function showAddDeploymentModal() {
            modalInput.value = '';
            modalError.style.display = 'none';
            modalError.textContent = '';
            modalPreset = 'api';
            presetBackendBtn.classList.add('active');
            presetFrontendBtn.classList.remove('active');
            addModal.classList.add('show');
            setTimeout(() => modalInput.focus(), 150);
        }

        function hideAddDeploymentModal() {
            addModal.classList.remove('show');
        }

        document.getElementById('btn-add-comp').addEventListener('click', () => {
            showAddDeploymentModal();
        });

        modalCancelBtn.addEventListener('click', () => {
            hideAddDeploymentModal();
        });

        // Close on backdrop click
        addModal.addEventListener('click', (e) => {
            if (e.target === addModal) {
                hideAddDeploymentModal();
            }
        });

        // Close on ESC key
        window.addEventListener('keydown', (e) => {
            if (e.key === 'Escape' && addModal.classList.contains('show')) {
                hideAddDeploymentModal();
            }
        });

        function handleModalConfirm() {
            const compName = modalInput.value.trim();
            if (!compName) {
                modalError.textContent = 'Deployment name is required.';
                modalError.style.display = 'block';
                return;
            }
            const cleanName = compName.toLowerCase().replace(/[^a-z0-9-]/g, '');
            if (!cleanName) {
                modalError.textContent = 'Name must contain only letters, numbers, or dashes.';
                modalError.style.display = 'block';
                return;
            }
            if (components[cleanName]) {
                modalError.textContent = 'A deployment with this name already exists.';
                modalError.style.display = 'block';
                return;
            }

            saveActiveComponentState();
            components[cleanName] = createDefaultComponentConfig(cleanName, modalPreset);
            activeComponent = cleanName;

            renderTabs();
            renderActiveComponent();
            updatePreview();
            hideAddDeploymentModal();
        }

        modalConfirmBtn.addEventListener('click', handleModalConfirm);

        modalInput.addEventListener('keydown', (e) => {
            if (e.key === 'Enter') {
                e.preventDefault();
                handleModalConfirm();
            }
        });

        // Save active component state from inputs to memory
        function saveActiveComponentState() {
            if (!activeComponent || !components[activeComponent]) return;

            const config = components[activeComponent];
            config.workloadType = document.getElementById('compWorkloadType').value;
            config.schedule = document.getElementById('compSchedule').value;
            config.replicas = 1;
            let svcPort = parseInt(document.getElementById('compPort').value, 10) || 8080;
            if (!config.service.ports) config.service.ports = { http: { protocol: 'TCP' } };
            if (!config.service.ports.http) config.service.ports.http = { protocol: 'TCP' };
            config.service.ports.http.port = svcPort;
            delete config.service.port;
            config.image.repository = document.getElementById('compRepo').value;
            config.image.tag = document.getElementById('compTag').value;
            config.route.path = document.getElementById('compRoutePath').value;

            const connectsToStr = document.getElementById('compConnectsTo').value.trim();
            config.connectsTo = connectsToStr ? connectsToStr.split(',').map(s => s.trim()).filter(Boolean) : [];
            config.runtime = document.getElementById('compRuntime').value.trim();

            if (!config.persistence) config.persistence = { enabled: false, ephemeral: false, mountPath: '/var/lib/data' };
            config.persistence.enabled = document.getElementById('persistenceEnabled').checked;
            config.persistence.ephemeral = document.getElementById('persistenceEphemeral').checked;
            config.persistence.mountPath = document.getElementById('persistenceMountPath').value.trim() || '/var/lib/data';
            config.persistence.storageRequest = document.getElementById('persistenceStorage').value.trim() || '1Gi';

            if (document.getElementById('truststoreEnabled').checked) {
                if (!config.initContainers) config.initContainers = {};
                const mountPath = document.getElementById('truststorePath').value.trim() || '/custom-truststore';
                const cert = document.getElementById('truststoreCert').value.trim();
                
                config.initContainers['build-truststore'] = {
                    image: {
                        repository: "tjpa-registry-quay-quay-enterprise.apps.ocp-hub.i.tj.pa.gov.br/ca/devops/third-party/pje-light/pje-binarios-light",
                        tag: "latest"
                    },
                    command: [
                        "sh",
                        "-c",
                        "DEFAULT_CACERTS=$(find / -name cacerts -type f 2>/dev/null | head -n 1)\nif [ -z \"$DEFAULT_CACERTS\" ]; then\n  echo \"Error: Could not find default cacerts file!\"\n  exit 1\nfi\necho \"Copying default cacerts from $DEFAULT_CACERTS to /custom-truststore/cacerts\"\ncp \"$DEFAULT_CACERTS\" /custom-truststore/cacerts\nchmod +w /custom-truststore/cacerts\n\nfor cert in /certs/*.pem; do\n  if [ -f \"$cert\" ]; then\n    alias_name=$(basename \"$cert\" .pem)\n    echo \"Importing certificate $cert with alias $alias_name...\"\n    keytool -importcert -trustcacerts -keystore /custom-truststore/cacerts \\\n      -storepass changeit -noprompt -alias \"$alias_name\" -file \"$cert\"\n  fi\ndone\necho \"All certificates imported successfully!\""
                    ],
                    persistence: {
                        enabled: true,
                        ephemeral: false,
                        mountPath: mountPath,
                        storageRequest: ""
                    }
                };
                
                if (cert) {
                    if (!config.files) config.files = { cm: {}, secret: {} };
                    if (!config.files.secret) config.files.secret = {};
                    config.files.secret['cert.pem'] = {
                        mountPath: "/certs",
                        content: cert
                    };
                } else if (config.files && config.files.secret && config.files.secret['cert.pem']) {
                    delete config.files.secret['cert.pem'];
                }
            } else {
                if (config.initContainers && config.initContainers['build-truststore']) {
                    delete config.initContainers['build-truststore'];
                }
                if (config.files && config.files.secret && config.files.secret['cert.pem']) {
                    delete config.files.secret['cert.pem'];
                }
            }

            if (!config.autoscaling) config.autoscaling = createDefaultComponentConfig('temp', 'app').autoscaling;
            config.autoscaling.enabled = document.getElementById('autoscalingEnabled').checked;
            config.autoscaling.engine = document.getElementById('autoscalingEngine').value;
            
            config.autoscaling.hpa.minReplicas = parseInt(document.getElementById('hpaMinReplicas').value.trim()) || 1;
            config.autoscaling.hpa.maxReplicas = parseInt(document.getElementById('hpaMaxReplicas').value.trim()) || 2;
            if (config.autoscaling.hpa.metrics && config.autoscaling.hpa.metrics.length > 0 && config.autoscaling.hpa.metrics[0].resource && config.autoscaling.hpa.metrics[0].resource.target) {
                config.autoscaling.hpa.metrics[0].resource.target.averageUtilization = parseInt(document.getElementById('hpaUtilization').value.trim()) || 180;
            }
            
            config.autoscaling.keda.minReplicas = parseInt(document.getElementById('kedaMinReplicas').value.trim()) || 3;
            config.autoscaling.keda.maxReplicas = parseInt(document.getElementById('kedaMaxReplicas').value.trim()) || 20;

            if (!config.vso) config.vso = { enabled: false };
            config.vso.enabled = document.getElementById('vsoEnabled').checked;

            config.route.default.enabled = document.getElementById('routeDefaultEnabled').checked;
            config.route.default.host = document.getElementById('routeDefaultHost').value.trim();

            config.route.internal.enabled = document.getElementById('routeIntEnabled').checked;
            config.route.internal.host = document.getElementById('routeIntHost').value.trim();

            config.route.external.enabled = document.getElementById('routeExtEnabled').checked;
            config.route.external.host = document.getElementById('routeExtHost').value.trim();

            config.route.additional = {};
            const additionalRouteDivs = document.getElementById('additional-routes-container').children;
            for (let i = 0; i < additionalRouteDivs.length; i++) {
                const name = additionalRouteDivs[i].querySelector('.additional-route-name').value.trim();
                const host = additionalRouteDivs[i].querySelector('.additional-route-host').value.trim();
                const enabled = additionalRouteDivs[i].querySelector('.additional-route-enabled').checked;
                if (name && host) {
                    config.route.additional[name] = { enabled: enabled, host: host };
                }
            }

            config.config = { env: {}, files: {} };
            config.secrets = { env: {}, files: {} };
            const fileDivs = document.getElementById('custom-files-container').children;
            for (let i = 0; i < fileDivs.length; i++) {
                const type = fileDivs[i].dataset.type;
                const name = fileDivs[i].querySelector('.file-name').value.trim();
                const path = fileDivs[i].querySelector('.file-mount').value.trim();
                const content = fileDivs[i].querySelector('.file-content').value;
                const isInline = fileDivs[i].querySelector('.file-content-toggle').checked;
                if (name && path) {
                    if (type === 'cm') {
                        config.config.files[name] = { mountPath: path };
                        if (isInline) config.config.files[name].content = content;
                    } else {
                        const b64enc = fileDivs[i].querySelector('.file-b64enc') ? fileDivs[i].querySelector('.file-b64enc').checked : false;
                        config.secrets.files[name] = { mountPath: path, b64enc: b64enc };
                        if (isInline) config.secrets.files[name].content = content;
                    }
                }
            }

            // Collect component ConfigMap
            document.getElementById('comp-cm-text').value.split('\n').forEach(line => {
                const idx = line.indexOf('=');
                const idxColon = line.indexOf(':');
                if (idx > 0 && (idxColon === -1 || idx < idxColon)) {
                    const k = line.substring(0, idx).trim();
                    const v = line.substring(idx + 1).trim();
                    if (k) config.config.env[k] = v;
                } else if (idxColon > 0) {
                    const k = line.substring(0, idxColon).trim();
                    const v = line.substring(idxColon + 1).trim();
                    if (k) config.config.env[k] = v;
                }
            });

            // Collect component Secret
            document.getElementById('comp-secret-text').value.split('\n').forEach(line => {
                const idx = line.indexOf('=');
                const idxColon = line.indexOf(':');
                if (idx > 0 && (idxColon === -1 || idx < idxColon)) {
                    const k = line.substring(0, idx).trim();
                    const v = line.substring(idx + 1).trim();
                    if (k) config.secrets.env[k] = v;
                } else if (idxColon > 0) {
                    const k = line.substring(0, idxColon).trim();
                    const v = line.substring(idxColon + 1).trim();
                    if (k) config.secrets.env[k] = v;
                }
            });
        }
        function toggleTruststoreUI() {
            const enabled = document.getElementById('truststoreEnabled').checked;
            const optionsDiv = document.getElementById('truststore-options');
            if (enabled) {
                optionsDiv.style.display = 'block';
            } else {
                optionsDiv.style.display = 'none';
            }
        }

        function togglePersistenceUI() {
            const enabled = document.getElementById('persistenceEnabled').checked;
            const optionsDiv = document.getElementById('persistence-options');
            if (enabled) {
                optionsDiv.style.display = 'block';
                const isEphemeral = document.getElementById('persistenceEphemeral').checked;
                document.getElementById('persistenceStorage').disabled = isEphemeral;
            } else {
                optionsDiv.style.display = 'none';
            }
        }
        function toggleAutoscalingUI() {
            const enabled = document.getElementById('autoscalingEnabled').checked;
            document.getElementById('autoscaling-options').style.display = enabled ? 'block' : 'none';
        }

        function toggleAutoscalingEngineUI() {
            const engine = document.getElementById('autoscalingEngine').value;
            if (engine === 'hpa') {
                document.getElementById('hpa-specific-options').style.display = 'block';
                document.getElementById('keda-specific-options').style.display = 'none';
            } else {
                document.getElementById('hpa-specific-options').style.display = 'none';
                document.getElementById('keda-specific-options').style.display = 'block';
            }
        }



        function toggleEnvVar(id) {
            document.getElementById(id + '-container').style.display = 'block';
            document.getElementById('btn-add-' + id).style.display = 'none';
        }

        function removeEnvVar(id) {
            document.getElementById(id + '-container').style.display = 'none';
            document.getElementById('btn-add-' + id).style.display = 'inline-block';
            document.getElementById(id + '-text').value = '';
            if (id.startsWith('global')) {
                saveGlobalConfig();
            } else {
                saveAndPreview();
            }
        }

        function addAdditionalRoute(name = '', host = '', enabled = true) {
            const container = document.getElementById('additional-routes-container');
            const routeId = 'route-' + Date.now() + Math.floor(Math.random() * 1000);

            const routeDiv = document.createElement('div');
            routeDiv.className = 'route-card';
            routeDiv.id = routeId;
            routeDiv.style.marginTop = '1rem';

            routeDiv.innerHTML = `
                <div class="route-card-header">
                    <div style="display: flex; gap: 0.5rem; align-items: center; width: 60%;">
                        <input type="text" class="additional-route-name" placeholder="Route Key (e.g. custom-api)" value="${name}" oninput="saveAndPreview()" style="margin: 0; padding: 0.2rem 0.5rem; font-size: 0.9rem;" />
                    </div>
                    <div style="display: flex; gap: 1rem; align-items: center;">
                        <button type="button" class="btn-remove-item" onclick="document.getElementById('${routeId}').remove(); saveAndPreview()">Remove</button>
                        <label class="route-switch">
                            <input type="checkbox" class="additional-route-enabled" onchange="saveAndPreview()" ${enabled ? 'checked' : ''}>
                            <span class="slider"></span>
                        </label>
                    </div>
                </div>
                <div class="form-group" style="margin-top: 1rem; margin-bottom: 0;">
                    <label>Route Hostname</label>
                    <input type="text" class="additional-route-host" placeholder="e.g. custom-api.domain.com" value="${host}" oninput="saveAndPreview()">
                </div>
            `;
            container.appendChild(routeDiv);
        }

        function addCustomFile(type, filename = '', mountPath = '', content = undefined, b64enc = false) {
            const container = document.getElementById('custom-files-container');
            const fileId = 'file-' + Date.now() + Math.floor(Math.random() * 1000);
            
            // If content is explicitly undefined and not empty string from typing, it defaults to inline true unless we are loading a state without it
            const isInline = content !== undefined;
            const actualContent = content || '';

            const fileDiv = document.createElement('div');
            fileDiv.className = 'form-group';
            fileDiv.id = fileId;
            fileDiv.style.border = '1px solid var(--border-color)';
            fileDiv.style.padding = '10px';
            fileDiv.style.borderRadius = 'var(--radius-md)';
            fileDiv.style.marginTop = '10px';
            fileDiv.dataset.type = type;

            fileDiv.innerHTML = `
                <div style="display: flex; justify-content: space-between; margin-bottom: 10px;">
                    <strong style="color: ${type === 'secret' ? '#f43f5e' : '#3b82f6'};">${type === 'secret' ? 'Secret File' : 'ConfigMap File'}</strong>
                    <button type="button" class="btn-remove-item" onclick="document.getElementById('${fileId}').remove(); saveAndPreview()">Remove</button>
                </div>
                <div class="form-group row" style="margin-bottom: 10px;">
                    <div>
                        <label>Filename (Key)</label>
                        <input type="text" class="file-name" value="${filename}" placeholder="e.g. nginx.conf" oninput="saveAndPreview()">
                    </div>
                    <div>
                        <label>Mount Path</label>
                        <input type="text" class="file-mount" value="${mountPath}" placeholder="e.g. /etc/nginx/nginx.conf" oninput="saveAndPreview()">
                    </div>
                </div>
                <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 5px;">
                    <div style="display: flex; align-items: center;">
                        <label class="route-switch" style="margin-bottom: 0;">
                            <input type="checkbox" class="file-content-toggle" ${isInline ? 'checked' : ''} onchange="this.parentElement.parentElement.parentElement.nextElementSibling.style.display = this.checked ? 'block' : 'none'; this.parentElement.parentElement.parentElement.nextElementSibling.nextElementSibling.style.display = this.checked ? 'none' : 'block'; saveAndPreview()">
                            <span class="slider"></span>
                        </label>
                        <span style="margin-left: 8px; font-size: 13px; font-weight: bold;">Provide Content Inline</span>
                    </div>
                    ${type === 'secret' ? `
                    <div style="display: flex; align-items: center;">
                        <label class="route-switch" style="margin-bottom: 0;">
                            <input type="checkbox" class="file-b64enc" ${b64enc ? 'checked' : ''} onchange="saveAndPreview()">
                            <span class="slider"></span>
                        </label>
                        <span style="margin-left: 8px; font-size: 13px;">Base64 Encoded</span>
                    </div>` : ''}
                </div>
                <textarea class="file-content" rows="4" placeholder="Paste file content here..." oninput="saveAndPreview()" style="display: ${isInline ? 'block' : 'none'};">${actualContent}</textarea>
                <div class="file-external-notice" style="display: ${isInline ? 'none' : 'block'}; padding: 10px; background-color: var(--surface-bg); border-radius: var(--radius-sm); border: 1px dashed var(--border-color); color: var(--text-muted); font-size: 13px; text-align: center; margin-top: 5px;">
                    Chart will automatically load this file from <code>files/&lt;filename&gt;</code>
                </div>
            `;
            container.appendChild(fileDiv);
        }

        function toggleRouteUI() {
            document.getElementById('routeDefaultHostWrapper').style.display = document.getElementById('routeDefaultEnabled').checked ? 'block' : 'none';
            document.getElementById('routeIntHostWrapper').style.display = document.getElementById('routeIntEnabled').checked ? 'block' : 'none';
            document.getElementById('routeExtHostWrapper').style.display = document.getElementById('routeExtEnabled').checked ? 'block' : 'none';
        }

        // Add listeners for route toggles
        document.getElementById('routeDefaultEnabled').addEventListener('change', toggleRouteUI);
        document.getElementById('routeIntEnabled').addEventListener('change', toggleRouteUI);
        document.getElementById('routeExtEnabled').addEventListener('change', toggleRouteUI);

        function renderActiveComponent() {
            if (!activeComponent || !components[activeComponent]) return;

            const config = components[activeComponent];
            
            document.getElementById('compWorkloadType').value = config.workloadType || 'Deployment';
            document.getElementById('compSchedule').value = config.schedule || '';
            toggleWorkloadTypeFields();

            let displayPort = 8080;
            if (config.service.ports && config.service.ports.http && config.service.ports.http.port) {
                displayPort = config.service.ports.http.port;
            } else if (config.service.port) {
                displayPort = config.service.port;
            }
            document.getElementById('compPort').value = displayPort;
            document.getElementById('compRepo').value = config.image.repository;
            document.getElementById('compTag').value = config.image.tag || '';
            document.getElementById('compRoutePath').value = config.route.path || '';
            document.getElementById('compConnectsTo').value = (config.connectsTo || []).join(', ');
            document.getElementById('compRuntime').value = config.runtime || '';

            if (config.persistence) {
                document.getElementById('persistenceEnabled').checked = config.persistence.enabled;
                document.getElementById('persistenceEphemeral').checked = !!config.persistence.ephemeral;
                document.getElementById('persistenceMountPath').value = config.persistence.mountPath || '/var/lib/data';
                document.getElementById('persistenceStorage').value = config.persistence.storageRequest || '1Gi';
            } else {
                document.getElementById('persistenceEnabled').checked = false;
                document.getElementById('persistenceEphemeral').checked = false;
                document.getElementById('persistenceMountPath').value = '/var/lib/data';
                document.getElementById('persistenceStorage').value = '1Gi';
            }

            if (config.initContainers && config.initContainers['build-truststore']) {
                const ts = config.initContainers['build-truststore'];
                document.getElementById('truststoreEnabled').checked = true;
                document.getElementById('truststorePath').value = (ts.persistence && ts.persistence.mountPath) ? ts.persistence.mountPath : '/custom-truststore';
                
                let cert = '';
                if (config.files && config.files.secret && config.files.secret['cert.pem']) {
                    cert = config.files.secret['cert.pem'].content || '';
                }
                document.getElementById('truststoreCert').value = cert;
            } else {
                document.getElementById('truststoreEnabled').checked = false;
                document.getElementById('truststorePath').value = '/custom-truststore';
                document.getElementById('truststoreCert').value = '';
            }

            if (config.autoscaling) {
                document.getElementById('autoscalingEnabled').checked = config.autoscaling.enabled;
                document.getElementById('autoscalingEngine').value = config.autoscaling.engine || 'hpa';
                document.getElementById('hpaMinReplicas').value = config.autoscaling.hpa.minReplicas || 1;
                document.getElementById('hpaMaxReplicas').value = config.autoscaling.hpa.maxReplicas || 2;
                document.getElementById('hpaUtilization').value = (config.autoscaling.hpa.metrics && config.autoscaling.hpa.metrics.length > 0 && config.autoscaling.hpa.metrics[0].resource && config.autoscaling.hpa.metrics[0].resource.target) ? config.autoscaling.hpa.metrics[0].resource.target.averageUtilization : 180;
                document.getElementById('kedaMinReplicas').value = config.autoscaling.keda.minReplicas || 3;
                document.getElementById('kedaMaxReplicas').value = config.autoscaling.keda.maxReplicas || 20;
            } else {
                document.getElementById('autoscalingEnabled').checked = false;
                document.getElementById('autoscalingEngine').value = 'hpa';
                document.getElementById('hpaMinReplicas').value = 1;
                document.getElementById('hpaMaxReplicas').value = 2;
                document.getElementById('hpaUtilization').value = 180;
                document.getElementById('kedaMinReplicas').value = 3;
                document.getElementById('kedaMaxReplicas').value = 20;
            }

            if (config.vso) {
                document.getElementById('vsoEnabled').checked = config.vso.enabled;
            } else {
                document.getElementById('vsoEnabled').checked = false;
            }

            togglePersistenceUI();
            toggleAutoscalingUI();
            toggleAutoscalingEngineUI();
            toggleTruststoreUI();

            document.getElementById('routeDefaultEnabled').checked = config.route.default.enabled;
            document.getElementById('routeDefaultHost').value = config.route.default.host || '';

            document.getElementById('routeIntEnabled').checked = config.route.internal.enabled;
            document.getElementById('routeIntHost').value = config.route.internal.host || '';

            document.getElementById('routeExtEnabled').checked = config.route.external.enabled;
            document.getElementById('routeExtHost').value = config.route.external.host || '';

            const additionalRoutesContainer = document.getElementById('additional-routes-container');
            additionalRoutesContainer.innerHTML = '';
            if (config.route.additional) {
                for (const [name, routeConfig] of Object.entries(config.route.additional)) {
                    addAdditionalRoute(name, routeConfig.host, routeConfig.enabled);
                }
            }

            const filesContainer = document.getElementById('custom-files-container');
            filesContainer.innerHTML = '';
            if (config.config && config.config.files) {
                for (const [name, fileConfig] of Object.entries(config.config.files)) {
                    addCustomFile('cm', name, fileConfig.mountPath, fileConfig.content);
                }
            }
            if (config.secrets && config.secrets.files) {
                for (const [name, fileConfig] of Object.entries(config.secrets.files)) {
                    addCustomFile('secret', name, fileConfig.mountPath, fileConfig.content, fileConfig.b64enc);
                }
            }

            toggleRouteUI();

            // Render ConfigMaps
            const cmText = Object.entries(config.config?.env || {}).map(([k, v]) => `${k}=${v}`).join('\n');
            document.getElementById('comp-cm-text').value = cmText;

            // Render Secrets
            const secretText = Object.entries(config.secrets?.env || {}).map(([k, v]) => `${k}=${v}`).join('\n');
            document.getElementById('comp-secret-text').value = secretText;

            if (cmText.trim().length > 0) toggleEnvVar('comp-cm');
            else { document.getElementById('comp-cm-container').style.display = 'none'; document.getElementById('btn-add-comp-cm').style.display = 'inline-block'; }

            if (secretText.trim().length > 0) toggleEnvVar('comp-secret');
            else { document.getElementById('comp-secret-container').style.display = 'none'; document.getElementById('btn-add-comp-secret').style.display = 'inline-block'; }
        }

        // Handle Global Config lists
        function renderGlobalConfigList() {
            const cmText = Object.entries(globalConfig || {}).map(([k, v]) => `${k}=${v}`).join('\n');
            document.getElementById('global-config-text').value = cmText;

            const secText = Object.entries(globalSecret || {}).map(([k, v]) => `${k}=${v}`).join('\n');
            document.getElementById('global-secret-text').value = secText;
            if (cmText.trim().length > 0) toggleEnvVar('global-config');
            if (secText.trim().length > 0) toggleEnvVar('global-secret');
        }

        function stripQuotes(str) {
            str = str.trim();
            if (str.length >= 2 && ((str.startsWith('"') && str.endsWith('"')) || (str.startsWith("'") && str.endsWith("'")))) {
                return str.substring(1, str.length - 1);
            }
            return str;
        }

        function parseTextAreaVars(text) {
            const parsed = {};
            text.split('\n').forEach(line => {
                const idx = line.indexOf('=');
                const idxColon = line.indexOf(':');
                if (idx > 0 && (idxColon === -1 || idx < idxColon)) {
                    const k = stripQuotes(line.substring(0, idx));
                    const v = stripQuotes(line.substring(idx + 1));
                    if (k) parsed[k] = v;
                } else if (idxColon > 0) {
                    const k = stripQuotes(line.substring(0, idxColon));
                    const v = stripQuotes(line.substring(idxColon + 1));
                    if (k) parsed[k] = v;
                }
            });
            return parsed;
        }

        function saveGlobalConfig() {
            globalConfig = parseTextAreaVars(document.getElementById('global-config-text').value);
            globalSecret = parseTextAreaVars(document.getElementById('global-secret-text').value);
            updatePreview();
        }

        function saveAndPreview() {
            saveActiveComponentState();
            updatePreview();
        }

        function renderFileTree() {
            const treeContainer = document.getElementById('ide-file-tree');
            treeContainer.innerHTML = '';

            const rootFiles = [];
            const templateFiles = [];

            Object.keys(generatedFiles).forEach(path => {
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
                treeContainer.appendChild(fileEl);
            });

            // Render templates folder if we have template files
            if (templateFiles.length > 0) {
                const folderEl = document.createElement('div');
                folderEl.className = 'tree-folder';

                const headerEl = document.createElement('div');
                headerEl.className = 'tree-folder-header';
                headerEl.innerHTML = `<span class="arrow">${templatesFolderOpen ? '&#9662;' : '&#9656;'}</span> <span style="font-size: 0.95rem; margin-right: 0.2rem;">📁</span> <span>templates</span>`;
                headerEl.addEventListener('click', () => {
                    templatesFolderOpen = !templatesFolderOpen;
                    renderFileTree();
                });
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
                treeContainer.appendChild(folderEl);
            }
        }

        function createFileNode(path, displayName) {
            const fileEl = document.createElement('div');
            fileEl.className = `tree-file ${activeFile === path ? 'active' : ''}`;

            // Icon
            let icon = '📄';
            if (path.endsWith('.yaml') || path.endsWith('.yml')) icon = '📝';
            if (path.endsWith('.tpl')) icon = '⚙️';
            if (path === '.helmignore') icon = '🚫';

            fileEl.innerHTML = `<span style="font-size: 0.95rem; margin-right: 0.2rem;">${icon}</span> <span>${displayName}</span>`;

            fileEl.addEventListener('click', () => {
                activeFile = path;
                // Highlight active item
                document.querySelectorAll('.tree-file').forEach(el => el.classList.remove('active'));
                fileEl.classList.add('active');
                renderActiveFileContent();
            });
            return fileEl;
        }

        function renderActiveFileContent() {
            const activeFilenameSpan = document.getElementById('active-filename');
            const editorBody = document.getElementById('ide-editor-body');

            let oldScrollTop = 0;
            const oldCodePre = document.getElementById('ide-code-pre');
            if (oldCodePre) {
                oldScrollTop = oldCodePre.scrollTop;
            }

            if (!activeFile || !generatedFiles[activeFile]) {
                activeFilenameSpan.textContent = 'Select a file';
                editorBody.innerHTML = `
                    <div class="ide-placeholder">
                        <svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><rect width="18" height="18" x="3" y="3" rx="2"></rect><path d="M9 3v18"></path><path d="m16 15-3-3 3-3"></path></svg>
                        <p>No file selected</p>
                    </div>
                `;
                return;
            }

            activeFilenameSpan.textContent = activeFile;

            const code = generatedFiles[activeFile];
            const lineCount = code.split('\n').length;

            // Generate line numbers column
            let lineNumsHTML = '';
            for (let i = 1; i <= lineCount; i++) {
                lineNumsHTML += `${i}\n`;
            }

            // Replace characters for HTML rendering
            const escapedCode = code
                .replace(/&/g, '&amp;')
                .replace(/</g, '&lt;')
                .replace(/>/g, '&gt;');

            editorBody.innerHTML = `
                <pre class="ide-line-numbers" id="ide-line-numbers">${lineNumsHTML}</pre>
                <pre class="ide-code-pre" id="ide-code-pre"><code>${escapedCode}</code></pre>
            `;

            const codePre = document.getElementById('ide-code-pre');
            const lineNums = document.getElementById('ide-line-numbers');
            if (codePre && lineNums) {
                codePre.onscroll = () => {
                    lineNums.scrollTop = codePre.scrollTop;
                };
                codePre.scrollTop = oldScrollTop;
                lineNums.scrollTop = oldScrollTop;
            }
        }

        // Input listeners for live updates
        document.querySelectorAll('input, select').forEach(element => {
            element.addEventListener('input', () => updatePreview());
            element.addEventListener('change', () => updatePreview());
        });

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

        function showToast() {
            const toast = document.getElementById('toast-message');
            toast.style.display = 'block';
            setTimeout(() => {
                toast.style.display = 'none';
            }, 3000);
        }

        // Add spinner CSS dynamically
        const style = document.createElement('style');
        style.innerHTML = `@keyframes spin { 0% { transform: rotate(0deg); } 100% { transform: rotate(360deg); } }`;
        document.head.appendChild(style);

        // Run on load removed (handled in main.js)
