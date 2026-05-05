package deployment

import (
	"fmt"
	"io"
	"regexp"
	"strings"
	"text/template"

	"github.com/arttor/helmify/pkg/processor/pod"

	"github.com/arttor/helmify/pkg/helmify"
	"github.com/arttor/helmify/pkg/processor"
	yamlformat "github.com/arttor/helmify/pkg/yaml"
	"github.com/iancoleman/strcase"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"
)

var deploymentGVC = schema.GroupVersionKind{
	Group:   "apps",
	Version: "v1",
	Kind:    "Deployment",
}

var deploymentTempl, _ = template.New("deployment").Parse(
	`{{- .Meta }}
spec:
{{- .Replicas }}
{{- .RevisionHistoryLimit }}
{{- .Strategy }}
  selector:
{{ .Selector }}
  template:
    metadata:
      labels:
{{ .PodLabels }}
{{- .PodAnnotations }}
    spec:
{{ .Spec }}`)

const selectorTempl = `%[1]s
{{- include "%[2]s.selectorLabels" . | nindent 6 }}
%[3]s`

// New creates processor for k8s Deployment resource.
func New() helmify.Processor {
	return &deployment{}
}

type deployment struct{}

// Process k8s Deployment object into template. Returns false if not capable of processing given resource type.
func (d deployment) Process(appMeta helmify.AppMetadata, obj *unstructured.Unstructured) (bool, helmify.Template, error) {
	if obj.GroupVersionKind() != deploymentGVC {
		return false, nil, nil
	}
	depl := appsv1.Deployment{}

	err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &depl)
	if err != nil {
		return true, nil, fmt.Errorf("%w: unable to cast to deployment", err)
	}
	meta, err := processor.ProcessObjMeta(appMeta, obj)
	if err != nil {
		return true, nil, err
	}

	values := helmify.Values{}

	name := processor.ObjectValueName(appMeta, obj)
	replicas, err := processReplicas(name, &depl, &values)
	if err != nil {
		return true, nil, err
	}

	revisionHistoryLimit, err := processRevisionHistoryLimit(name, &depl, &values)
	if err != nil {
		return true, nil, err
	}

	strategy, err := processStrategy(name, &depl, &values)
	if err != nil {
		return true, nil, err
	}

	matchLabels, err := yamlformat.Marshal(map[string]interface{}{"matchLabels": depl.Spec.Selector.MatchLabels}, 0)
	if err != nil {
		return true, nil, err
	}
	matchExpr := ""
	if depl.Spec.Selector.MatchExpressions != nil {
		matchExpr, err = yamlformat.Marshal(map[string]interface{}{"matchExpressions": depl.Spec.Selector.MatchExpressions}, 0)
		if err != nil {
			return true, nil, err
		}
	}
	selector := fmt.Sprintf(selectorTempl, matchLabels, appMeta.ChartName(), matchExpr)
	selector = strings.Trim(selector, " \n")
	selector = string(yamlformat.Indent([]byte(selector), 4))

	podLabels, err := yamlformat.Marshal(depl.Spec.Template.ObjectMeta.Labels, 8)
	if err != nil {
		return true, nil, err
	}
	podLabels += fmt.Sprintf("\n      {{- include \"%s.selectorLabels\" . | nindent 8 }}", appMeta.ChartName())

	podAnnotations := ""
	annotations := depl.Spec.Template.ObjectMeta.Annotations
	annotations = pod.AddReloadingAnnotations(appMeta, annotations, &depl.Spec.Template.Spec)
	depl.Spec.Template.ObjectMeta.Annotations = annotations

	if len(depl.Spec.Template.ObjectMeta.Annotations) != 0 {
		podAnnotations, err = yamlformat.Marshal(map[string]interface{}{"annotations": depl.Spec.Template.ObjectMeta.Annotations}, 6)
		if err != nil {
			return true, nil, err
		}

		podAnnotations = "\n" + podAnnotations
	}

	nameCamel := strcase.ToLowerCamel(name)
	specMap, podValues, err := pod.ProcessSpec(nameCamel, appMeta, depl.Spec.Template.Spec, 0)
	if err != nil {
		return true, nil, err
	}
	err = values.Merge(podValues)
	if err != nil {
		return true, nil, err
	}

	spec, err := yamlformat.Marshal(specMap, 6)
	if err != nil {
		return true, nil, err
	}
	if appMeta.Config().AddWebhookOption {
		spec = addWebhookOption(spec)
	}

	spec = replaceSingleQuotes(spec)

	return true, &result{
		name:   name,
		values: values,
		data: struct {
			Meta                 string
			Replicas             string
			RevisionHistoryLimit string
			Strategy             string
			Selector             string
			PodLabels            string
			PodAnnotations       string
			Spec                 string
		}{
			Meta:                 meta,
			Replicas:             replicas,
			RevisionHistoryLimit: revisionHistoryLimit,
			Strategy:             strategy,
			Selector:             selector,
			PodLabels:            podLabels,
			PodAnnotations:       podAnnotations,
			Spec:                 spec,
		},
	}, nil
}

func replaceSingleQuotes(s string) string {
	r := regexp.MustCompile(`'({{((.*|.*\n.*))}}.*)'`)
	return r.ReplaceAllString(s, "${1}")
}

func addWebhookOption(manifest string) string {
	webhookOptionHeader := "      {{- if .Values.webhook.enabled }}"
	webhookOptionFooter := "      {{- end }}"
	volumes := `      - name: cert
        secret:
          defaultMode: 420
          secretName: webhook-server-cert`
	volumeMounts := `        - mountPath: /tmp/k8s-webhook-server/serving-certs
          name: cert
          readOnly: true`
	manifest = strings.ReplaceAll(manifest, volumes, fmt.Sprintf("%s\n%s\n%s",
		webhookOptionHeader, volumes, webhookOptionFooter))
	manifest = strings.ReplaceAll(manifest, volumeMounts, fmt.Sprintf("%s\n%s\n%s",
		webhookOptionHeader, volumeMounts, webhookOptionFooter))

	re := regexp.MustCompile(`        - containerPort: \d+
          name: webhook-server
          protocol: TCP`)

	manifest = re.ReplaceAllString(manifest, fmt.Sprintf("%s\n%s\n%s", webhookOptionHeader,
		re.FindString(manifest), webhookOptionFooter))
	return manifest
}

func processReplicas(name string, deployment *appsv1.Deployment, values *helmify.Values) (string, error) {
	if deployment.Spec.Replicas == nil {
		return "", nil
	}
	nameCamel := strcase.ToLowerCamel(name)
	_, err := values.Add(int64(*deployment.Spec.Replicas), name, "replicas")
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("{{- if not (kindIs \"nil\" .Values.%s.replicas) }}\n  replicas: {{ .Values.%s.replicas }}\n{{- end }}", nameCamel, nameCamel), nil
}

func processRevisionHistoryLimit(name string, deployment *appsv1.Deployment, values *helmify.Values) (string, error) {
	if deployment.Spec.RevisionHistoryLimit == nil {
		return "", nil
	}
	nameCamel := strcase.ToLowerCamel(name)
	_, err := values.Add(int64(*deployment.Spec.RevisionHistoryLimit), name, "revisionHistoryLimit")
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("{{- if not (kindIs \"nil\" .Values.%s.revisionHistoryLimit) }}\n  revisionHistoryLimit: {{ .Values.%s.revisionHistoryLimit }}\n{{- end }}", nameCamel, nameCamel), nil
}

func processStrategy(name string, deployment *appsv1.Deployment, values *helmify.Values) (string, error) {
	if deployment.Spec.Strategy.Type == "" {
		return "", nil
	}
	nameCamel := strcase.ToLowerCamel(name)
	strategyMap := map[string]interface{}{
		"type": string(deployment.Spec.Strategy.Type),
	}
	// ... (rest of strategyMap logic is fine as it populates values)
	if deployment.Spec.Strategy.RollingUpdate != nil {
		ru := deployment.Spec.Strategy.RollingUpdate
		ruMap := map[string]interface{}{}
		if ru.MaxSurge != nil {
			if ru.MaxSurge.Type == intstr.Int {
				ruMap["maxSurge"] = int64(ru.MaxSurge.IntVal)
			} else {
				ruMap["maxSurge"] = ru.MaxSurge.StrVal
			}
		}
		if ru.MaxUnavailable != nil {
			if ru.MaxUnavailable.Type == intstr.Int {
				ruMap["maxUnavailable"] = int64(ru.MaxUnavailable.IntVal)
			} else {
				ruMap["maxUnavailable"] = ru.MaxUnavailable.StrVal
			}
		}
		strategyMap["rollingUpdate"] = ruMap
	}
	_ = unstructured.SetNestedField(*values, strategyMap, nameCamel, "strategy")
	return fmt.Sprintf("{{- with .Values.%s.strategy }}\n  strategy:\n    {{- toYaml . | nindent 4 }}\n{{- end }}", nameCamel), nil
}

type result struct {
	name string
	data struct {
		Meta                 string
		Replicas             string
		RevisionHistoryLimit string
		Strategy             string
		Selector             string
		PodLabels            string
		PodAnnotations       string
		Spec                 string
	}
	values helmify.Values
}

func (r *result) Filename() string {
	return fmt.Sprintf("%s-deployment.yaml", r.name)
}

func (r *result) Values() helmify.Values {
	return r.values
}

func (r *result) Write(writer io.Writer) error {
	return deploymentTempl.Execute(writer, r.data)
}
