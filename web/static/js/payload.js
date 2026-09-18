        function parseComponentConfig(compData) {
            return {
                workloadType: compData.workloadType || 'Deployment',
                schedule: compData.schedule || '',
                replicas: compData.replicas !== undefined ? compData.replicas : 1,
                image: {
                    repository: (compData.image && compData.image.repository) || '',
                    tag: (compData.image && compData.image.tag) || ''
                },
                service: {
                    ports: {
                        http: {
                            port: (compData.service && compData.service.ports && compData.service.ports.http && compData.service.ports.http.port) || (compData.service && compData.service.port) || 8080,
                            protocol: 'TCP'
                        }
                    }
                },
                config: compData.config || { env: {}, files: {} },
                secrets: compData.secrets || { env: {}, files: {} },
                connectsTo: compData.connectsTo || [],
                runtime: compData.runtime || '',
                persistence: {
                    enabled: !!(compData.persistence && compData.persistence.enabled),
                    mountPath: (compData.persistence && compData.persistence.mountPath) || '/var/lib/data'
                },
                initContainers: compData.initContainers || {},
                extraContainers: compData.extraContainers || {},
                vso: {
                    enabled: !!(compData.vso && compData.vso.enabled)
                },
                route: {
                    path: (compData.route && compData.route.path) || '',
                    default: {
                        enabled: !!(compData.route && compData.route.default && compData.route.default.enabled),
                        host: (compData.route && compData.route.default && compData.route.default.host) || ''
                    },
                    internal: {
                        enabled: !!(compData.route && compData.route.internal && compData.route.internal.enabled),
                        host: (compData.route && compData.route.internal && compData.route.internal.host) || ''
                    },
                    external: {
                        enabled: !!(compData.route && compData.route.external && compData.route.external.enabled),
                        host: (compData.route && compData.route.external && compData.route.external.host) || ''
                    },
                    additional: (compData.route && compData.route.additional) || {}
                },
                hpa: compData.hpa || {
                    enabled: false,
                    minReplicas: 1,
                    maxReplicas: 2,
                    metrics: [
                        {
                            type: 'Resource',
                            resource: {
                                name: 'memory',
                                target: {
                                    type: 'Utilization',
                                    averageUtilization: 180
                                }
                            }
                        }
                    ],
                    behavior: {
                        scaleDown: { stabilizationWindowSeconds: 120 },
                        scaleUp: {
                            stabilizationWindowSeconds: 0,
                            policies: [
                                {
                                    type: 'Percent',
                                    value: 100,
                                    periodSeconds: 15
                                }
                            ]
                        }
                    }
                }
            };
        }

        function createDefaultComponentConfig(name, type) {
            const chartNameVal = document.getElementById('chartName').value.trim() || 'chart-model';
            const hostPrefix = chartType === 'single' ? name : `${chartNameVal}-${name}`;
            const isFrontend = type === 'app' || type === 'frontend' || type === 'web';
            const suffix = isFrontend ? 'app' : 'api';
            return {
                workloadType: 'Deployment',
                schedule: '',
                replicas: isFrontend ? 0 : 2,
                image: {
                    repository: `tjpa-registry-quay-quay-enterprise.apps.ocp-hub.i.tj.pa.gov.br/tjpa/${chartNameVal}-${suffix}`,
                    tag: 'latest'
                },
                service: {
                    ports: {
                        http: {
                            port: 8080,
                            protocol: 'TCP'
                        }
                    }
                },
                config: { env: {}, files: {} },
                secrets: { env: {}, files: {} },
                connectsTo: [],
                runtime: '',
                persistence: { enabled: false, mountPath: '/var/lib/data' },
                initContainers: {},
                extraContainers: {},
                vso: { enabled: false },
                route: {
                    path: '',
                    default: { enabled: true, host: `${hostPrefix}{{DEFAULT_DOMAIN}}` },
                    internal: { enabled: false, host: `${hostPrefix}{{INTERNAL_DOMAIN}}` },
                    external: { enabled: false, host: `${hostPrefix}{{EXTERNAL_DOMAIN}}` },
                    additional: {}
                },
                autoscaling: {
                    enabled: false,
                    engine: "",
                    keda: {
                        minReplicas: 3,
                        maxReplicas: 20,
                        pollingInterval: 30,
                        cooldownPeriod: 300,
                        triggers: [
                            {
                                type: 'rabbitmq',
                                metadata: {
                                    queueName: 'jurisprudencia.celery.knowledge',
                                    queueLength: '200'
                                }
                            }
                        ],
                        triggerAuth: {
                            secretTargetRef: [
                                {
                                    parameter: 'host',
                                    name: 'iande-global-secret',
                                    key: 'JURISPRUDENCIA_RABBITMQ_URL'
                                }
                            ]
                        }
                    },
                    hpa: {
                        minReplicas: 1,
                        maxReplicas: 2,
                        metrics: [
                            { type: 'Resource', resource: { name: 'memory', target: { type: 'Utilization', averageUtilization: 180 } } }
                        ],
                        behavior: {
                            scaleDown: { stabilizationWindowSeconds: 120 },
                            scaleUp: { stabilizationWindowSeconds: 0, policies: [ { type: 'Percent', value: 100, periodSeconds: 15 } ] }
                        }
                    }
                }
            };
        }

        // Combine inputs and return JSON payload
        function buildPayload() {
            saveActiveComponentState();

            const selectedSubcomponents = [];
            document.querySelectorAll('input[name="subcomponent"]:checked').forEach(cb => {
                selectedSubcomponents.push(cb.value);
            });

            const payload = {
                chartName: document.getElementById('chartName').value || 'chart-model',
                type: chartType,
                devRepoUrl: document.getElementById('devRepoUrl').value || (chartType === 'single' ? 'https://{{DEV_REPO}}/devops/my-app.git' : 'https://{{DEV_REPO}}/devops/my-app-multi.git'),
                globalConfig: globalConfig,
                globalSecret: globalSecret,
                deployments: components,
                subcomponents: selectedSubcomponents
            };
            return payload;
        }

