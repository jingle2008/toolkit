package k8s

import (
	"context"
	"fmt"

	"github.com/jingle2008/toolkit/pkg/models"
	"gopkg.in/yaml.v3"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

// LoadBaseModels returns all ClusterBaseModel CRs as a slice.
func LoadBaseModels(ctx context.Context, client dynamic.Interface) ([]models.BaseModel, error) {
	gvr := schema.GroupVersionResource{
		Group:    "ome.io",
		Version:  "v1beta1",
		Resource: "clusterbasemodels",
	}
	list, err := client.Resource(gvr).List(ctx, v1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list ClusterBaseModel: %w", err)
	}
	result := make([]models.BaseModel, 0, len(list.Items))
	for _, item := range list.Items {
		bm, err := parseBaseModel(&item)
		if err != nil {
			return nil, fmt.Errorf("parse ClusterBaseModel %q: %w", item.GetName(), err)
		}
		result = append(result, bm)
	}
	return result, nil
}

func parseBaseModel(obj *unstructured.Unstructured) (models.BaseModel, error) {
	spec, _, _ := unstructured.NestedMap(obj.Object, "spec")
	status, _, _ := unstructured.NestedMap(obj.Object, "status")
	capabilities, _, _ := unstructured.NestedStringSlice(spec, "modelCapabilities")
	metadata, _, _ := unstructured.NestedStringMap(spec, "additionalMetadata")

	var dacShapeConfigs *models.DacShapeConfigs
	if dac, ok := metadata["dacShapeConfigs"]; ok && dac != "" {
		dacShapeConfigs = unmarshalYaml[models.DacShapeConfigs](dac)
	}

	labels := getLabels(obj)
	annotations := getAnnotations(obj)

	var runtime string
	var runtimeType string
	if value, ok := annotations["models.ome.io/runtime"]; ok {
		runtime = value
		runtimeType = "Serving"
	} else if value, ok := annotations["models.ome.io/training-runtime"]; ok {
		runtime = value
		runtimeType = "Fine-tuning"
	}

	name := obj.GetName()
	var internalName string
	if value, ok := metadata["internalName"]; ok {
		internalName = value
	} else {
		internalName = name
	}

	displayName, _, _ := unstructured.NestedString(spec, "displayName")
	version, _, _ := unstructured.NestedString(spec, "version")
	vendor, _, _ := unstructured.NestedString(spec, "vendor")
	maxTokens, _, _ := unstructured.NestedInt64(spec, "maxTokens")
	state, _, _ := unstructured.NestedString(status, "state")
	parameterSize, _, _ := unstructured.NestedString(spec, "modelParameterSize")

	bm := models.BaseModel{
		InternalName:         internalName,
		Name:                 name,
		DisplayName:          displayName,
		Type:                 runtimeType,
		Version:              version,
		Vendor:               vendor,
		MaxTokens:            int(maxTokens),
		VaultKey:             annotations["ome.io/base-model-decryption-key-name"],
		LifeCyclePhase:       annotations["models.ome.io/lifecycle-phase"],
		IsExperimental:       annotations["models.ome.io/experimental"] == "true",
		IsInternal:           annotations["models.ome.io/internal"] == "true",
		Capabilities:         capabilities,
		Runtime:              runtime,
		Replicas:             0,
		DacShapeConfigs:      dacShapeConfigs,
		DeprecatedDate:       labels["genai-model-deprecated-date"],
		OnDemandRetiredDate:  labels["genai-model-on-demand-retired-date"],
		DedicatedRetiredDate: labels["genai-model-dedicated-retired-date"],
		IsImageTextToText:    metadata["image-text-to-text"] == "true",
		ParameterSize:        parameterSize,
		Status:               state,
	}

	return bm, nil
}

func unmarshalYaml[T any](text string) *T {
	var result T
	if err := yaml.Unmarshal([]byte(text), &result); err != nil {
		return nil
	}
	return &result
}
