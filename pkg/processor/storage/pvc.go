package storage

import (
	"fmt"
	"github.com/arttor/helmify/pkg/helmify"
	"github.com/arttor/helmify/pkg/processor"
	yamlformat "github.com/arttor/helmify/pkg/yaml"
	"github.com/iancoleman/strcase"
	"io"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"strings"
	"text/template"
)

var pvcTempl, _ = template.New("pvc").Parse(
	`{{ .Meta }}
{{ .Spec }}`)

var pvcGVC = schema.GroupVersionKind{
	Group:   "",
	Version: "v1",
	Kind:    "PersistentVolumeClaim",
}

// New creates processor for k8s PVC resource.
func New() helmify.Processor {
	return &pvc{}
}

type pvc struct{}

// Process k8s PVC object into template. Returns false if not capable of processing given resource type.
func (p pvc) Process(appMeta helmify.AppMetadata, obj *unstructured.Unstructured) (bool, helmify.Template, error) {
	if obj.GroupVersionKind() != pvcGVC {
		return false, nil, nil
	}


	name := processor.ObjectValueName(appMeta, obj)
	nameCamelCase := strcase.ToLowerCamel(name)
	values := helmify.Values{}
	var err error

	// NEW LOGIC: Scan workloads to see if they mount this PVC
	targetComponent := ""
	for _, o := range appMeta.Objects() {
		kind := strings.ToLower(o.GetKind())
		if kind == "deployment" || kind == "statefulset" || kind == "daemonset" || kind == "job" {
			volumes, found, _ := unstructured.NestedSlice(o.Object, "spec", "template", "spec", "volumes")
			if found {
				for _, vRaw := range volumes {
					if v, ok := vRaw.(map[string]interface{}); ok {
						if pvc, hasPvc := v["persistentVolumeClaim"].(map[string]interface{}); hasPvc {
							if claimName, _ := pvc["claimName"].(string); claimName == obj.GetName() {
								targetComponent = processor.GetComponent(o)
								break
							}
						}
					}
				}
			}
		} else if kind == "pod" {
			volumes, found, _ := unstructured.NestedSlice(o.Object, "spec", "volumes")
			if found {
				for _, vRaw := range volumes {
					if v, ok := vRaw.(map[string]interface{}); ok {
						if pvc, hasPvc := v["persistentVolumeClaim"].(map[string]interface{}); hasPvc {
							if claimName, _ := pvc["claimName"].(string); claimName == obj.GetName() {
								targetComponent = processor.GetComponent(o)
								break
							}
						}
					}
				}
			}
		}
		if targetComponent != "" {
			break
		}
	}

	targetRoot := "pvc"
	targetKey := nameCamelCase
	if targetComponent != "" {
		targetRoot = strcase.ToLowerCamel(targetComponent)
		if targetRoot == "" {
			targetRoot = strcase.ToLowerCamel(processor.ObjectValueName(appMeta, obj))
		}
		targetKey = "persistence"
		err := unstructured.SetNestedField(values, true, targetRoot, "persistence", "enabled")
		if err != nil {
			return true, nil, err
		}
	}

	suffix := processor.GetDynamicSuffix(appMeta, obj, "persistentvolumeclaim")
	var meta string
	if targetComponent != "" {
		meta, err = processor.ProcessObjMeta(appMeta, obj, processor.WithSuffix(suffix), processor.WithComponent(targetComponent))
	} else {
		meta, err = processor.ProcessObjMeta(appMeta, obj, processor.WithSuffix(suffix))
	}
	if err != nil {
		return true, nil, err
	}

	claim := corev1.PersistentVolumeClaim{}
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &claim)
	if err != nil {
		return true, nil, fmt.Errorf("%w: unable to cast to PVC", err)
	}

	// template storage class name
	if claim.Spec.StorageClassName != nil {
		templatedSC, err := values.Add(*claim.Spec.StorageClassName, targetRoot, targetKey, "storageClass")
		if err != nil {
			return true, nil, err
		}
		claim.Spec.StorageClassName = &templatedSC
	}

	// template resources
	specMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&claim.Spec)
	if err != nil {
		return true, nil, err
	}

	storageReq, ok, _ := unstructured.NestedString(specMap, "resources", "requests", "storage")
	if ok {
		templatedStorageReq, err := values.Add(storageReq, targetRoot, targetKey, "storageRequest")
		if err != nil {
			return true, nil, err
		}
		err = unstructured.SetNestedField(specMap, templatedStorageReq, "resources", "requests", "storage")
		if err != nil {
			return true, nil, err
		}
	}

	storageLim, ok, _ := unstructured.NestedString(specMap, "resources", "limits", "storage")
	if ok {
		templatedStorageLim, err := values.Add(storageLim, targetRoot, targetKey, "storageLimit")
		if err != nil {
			return true, nil, err
		}
		err = unstructured.SetNestedField(specMap, templatedStorageLim, "resources", "limits", "storage")
		if err != nil {
			return true, nil, err
		}
	}

	spec, err := yamlformat.Marshal(map[string]interface{}{"spec": specMap}, 0)
	if err != nil {
		return true, nil, err
	}
	spec = strings.ReplaceAll(spec, "'", "")

	return true, &result{
		name: name,
		data: struct {
			Meta string
			Spec string
		}{Meta: meta, Spec: spec},
		values:          values,
		targetComponent: targetRoot,
		isStandalone:    targetComponent == "",
	}, nil
}

type result struct {
	name string
	data struct {
		Meta string
		Spec string
	}
	values          helmify.Values
	targetComponent string
	isStandalone    bool
}

func (r *result) Filename() string {
	if r.targetComponent != "" {
		return fmt.Sprintf("pvc-%s.yaml", r.targetComponent)
	}
	return fmt.Sprintf("pvc-%s.yaml", r.name)
}

func (r *result) Values() helmify.Values {
	return r.values
}

func (r *result) Write(writer io.Writer) error {
	var err error
	if !r.isStandalone {
		_, err = fmt.Fprintf(writer, "{{- if .Values.%s.persistence.enabled -}}\n", r.targetComponent)
		if err != nil {
			return err
		}
	}
	
	err = pvcTempl.Execute(writer, r.data)
	if err != nil {
		return err
	}
	
	if !r.isStandalone {
		_, err = fmt.Fprint(writer, "{{- end -}}\n")
		if err != nil {
			return err
		}
	}
	return nil
}
