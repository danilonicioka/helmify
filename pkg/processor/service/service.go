package service

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	"github.com/arttor/helmify/pkg/processor"

	"github.com/arttor/helmify/pkg/helmify"
	yamlformat "github.com/arttor/helmify/pkg/yaml"
	"github.com/iancoleman/strcase"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/yaml"
)

const (
	svcTempSpec = `
spec:
  type: {{ .Values.%[1]s.service.type }}
  selector:%[2]s
    {{- include "%[3]s" . | nindent 4 }}%[4]s
  ports:
  {{- .Values.%[1]s.service.ports | toYaml | nindent 2 }}`
)

const (
	lbSourceRangesTempSpec = `
  loadBalancerSourceRanges:
  {{- .Values.%[1]s.service.loadBalancerSourceRanges | toYaml | nindent 2 }}`
)

const (
	ipFamilyTempSpec = `
  {{- if .Values.%[1]s.service.ipFamilyPolicy }}
  ipFamilyPolicy: {{ .Values.%[1]s.service.ipFamilyPolicy }}
  {{- end }}
  {{- if .Values.%[1]s.service.ipFamilies }}
  ipFamilies:
  {{- .Values.%[1]s.service.ipFamilies | toYaml | nindent 2 }}
  {{- end }}`
)

var svcGVC = schema.GroupVersionKind{
	Group:   "",
	Version: "v1",
	Kind:    "Service",
}

// New creates processor for k8s Service resource.
func New() helmify.Processor {
	return &svc{}
}

type svc struct{}

// Process k8s Service object into template. Returns false if not capable of processing given resource type.
func (r svc) Process(appMeta helmify.AppMetadata, obj *unstructured.Unstructured) (bool, helmify.Template, error) {
	if obj.GroupVersionKind() != svcGVC {
		return false, nil, nil
	}
	service := corev1.Service{}
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &service)
	if err != nil {
		return true, nil, fmt.Errorf("%w: unable to cast to service", err)
	}

	name := processor.ObjectValueName(appMeta, obj)
	shortName := strings.TrimPrefix(name, "controller-manager-")
	shortNameCamel := strcase.ToLowerCamel(processor.GetComponent(obj))

	suffix := processor.GetDynamicSuffix(appMeta, obj, "svc")

	meta, err := processor.ProcessObjMeta(appMeta, obj, processor.WithSuffix(suffix))
	if err != nil {
		return true, nil, err
	}

	cleanSelector := map[string]string{}
	for k, v := range service.Spec.Selector {
		if k == "app.kubernetes.io/name" || k == "app.kubernetes.io/instance" ||
			k == "app.kubernetes.io/version" || k == "app.kubernetes.io/managed-by" ||
			k == "app.kubernetes.io/component" || k == "app" ||
			k == "helm.sh/chart" || k == "deployment" {
			continue
		}
		cleanSelector[k] = v
	}

	var selector string
	if len(cleanSelector) > 0 {
		selectorBytes, _ := yaml.Marshal(cleanSelector)
		selectorBytes = yamlformat.Indent(selectorBytes, 4)
		selector = "\n" + string(bytes.TrimRight(selectorBytes, "\n "))
	}

	values := helmify.Values{}
	svcType := service.Spec.Type
	if svcType == "" {
		svcType = corev1.ServiceTypeClusterIP
	}
	_ = unstructured.SetNestedField(values, string(svcType), shortNameCamel, "service", "type")
	ports := make([]interface{}, len(service.Spec.Ports))
	for i, p := range service.Spec.Ports {
		pMap := map[string]interface{}{
			"port": int64(p.Port),
		}
		if p.Name != "" {
			pMap["name"] = p.Name
		}
		if p.NodePort != 0 {
			pMap["nodePort"] = int64(p.NodePort)
		}
		if p.Protocol != "" {
			pMap["protocol"] = string(p.Protocol)
		}
		if p.TargetPort.Type == intstr.Int {
			pMap["targetPort"] = int64(p.TargetPort.IntVal)
		} else {
			pMap["targetPort"] = p.TargetPort.StrVal
		}
		ports[i] = pMap
	}

	_ = unstructured.SetNestedSlice(values, ports, shortNameCamel, "service", "ports")

	comp := processor.GetComponent(obj)
	labelHelper := appMeta.ChartName() + ".selectorLabels"
	if comp == "api" || comp == "app" {
		labelHelper = fmt.Sprintf("%s.%s.selectorLabels", appMeta.ChartName(), comp)
	}
	ipFamilySpec := parseIPFamily(values, service, shortNameCamel)
	res := meta + fmt.Sprintf(svcTempSpec, shortNameCamel, selector, labelHelper, ipFamilySpec)

	res += parseLoadBalancerSourceRanges(values, service, shortNameCamel)

	if shortNameCamel == "webhookService" && appMeta.Config().AddWebhookOption {
		res = fmt.Sprintf("{{- if .Values.webhook.enabled }}\n%s\n{{- end }}", res)
	}

	resultName := shortName
	if shortName == appMeta.ChartName() || !processor.IsMultiDeployment(appMeta) {
		resultName = ""
	}

	return true, &result{
		name:   resultName,
		data:   res,
		values: values,
	}, nil
}

func parseIPFamily(values helmify.Values, service corev1.Service, shortNameCamel string) string {
	hasIPFamilyPolicy := service.Spec.IPFamilyPolicy != nil
	hasIPFamilies := len(service.Spec.IPFamilies) > 0

	if !hasIPFamilyPolicy && !hasIPFamilies {
		return ""
	}

	if hasIPFamilyPolicy {
		_ = unstructured.SetNestedField(values, string(*service.Spec.IPFamilyPolicy), shortNameCamel, "service", "ipFamilyPolicy")
	}

	if hasIPFamilies {
		ipFamilies := make([]interface{}, len(service.Spec.IPFamilies))
		for i, fam := range service.Spec.IPFamilies {
			ipFamilies[i] = string(fam)
		}
		_ = unstructured.SetNestedSlice(values, ipFamilies, shortNameCamel, "service", "ipFamilies")
	}

	return fmt.Sprintf(ipFamilyTempSpec, shortNameCamel)
}

func parseLoadBalancerSourceRanges(values helmify.Values, service corev1.Service, shortNameCamel string) string {
	if len(service.Spec.LoadBalancerSourceRanges) < 1 {
		return ""
	}
	lbSourceRanges := make([]interface{}, len(service.Spec.LoadBalancerSourceRanges))
	for i, ip := range service.Spec.LoadBalancerSourceRanges {
		lbSourceRanges[i] = ip
	}
	_ = unstructured.SetNestedSlice(values, lbSourceRanges, shortNameCamel, "service", "loadBalancerSourceRanges")
	return fmt.Sprintf(lbSourceRangesTempSpec, shortNameCamel)
}

type result struct {
	name   string
	data   string
	values helmify.Values
}

func (r *result) Filename() string {
	if r.name == "chart" || r.name == "" {
		return "svc.yaml"
	}
	return fmt.Sprintf("svc-%s.yaml", r.name)
}

func (r *result) Values() helmify.Values {
	return r.values
}

func (r *result) Write(writer io.Writer) error {
	_, err := writer.Write([]byte(r.data))
	return err
}
