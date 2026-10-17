package telemetry

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/pkg/models"
)

// Window is how far back a metrics dashboard looks.
const Window = 7 * 24 * time.Hour

// Plan is what a metrics dashboard for one row needs: the MQL filter that
// scopes it and the capability that picks its metrics.
type Plan struct {
	Filter     Filter
	Capability Capability
}

// Catalog finds the model a row serves; *models.Dataset is one, with the
// catalog CatalogFor names loaded.
type Catalog interface {
	FindModelByName(name string) *models.BaseModel
	FindBaseModelByName(name string) *models.BaseModel
}

// ErrNoMetrics is PlanFor's error for a row with no dashboard at all: a
// category without metrics, or no row. Callers treat it as a no-op where
// asking was implicit (a TUI key on any row).
var ErrNoMetrics = errors.New("no metrics for this row")

// SeveralDACsError is PlanFor's error for an imported model several DACs
// serve: each has its own dashboard, so the caller picks one (TUI: a
// picker; CLI/MCP: a dac argument) and narrows the model with OnDAC.
type SeveralDACsError struct {
	Model string
	DACs  []string // the DAC namespaces serving the model
}

func (e *SeveralDACsError) Error() string {
	return fmt.Sprintf("imported model %s is served by several DACs: %s", e.Model, strings.Join(e.DACs, ", "))
}

// OnDAC narrows an imported model to its copy on DAC dac, so its plan is
// that DAC's dashboard.
func OnDAC(im models.ImportedModel, dac string) (models.ImportedModel, error) {
	if !slices.Contains(im.DACNamespaces(), dac) {
		return models.ImportedModel{}, fmt.Errorf("imported model %s is not on DAC %s; it is on: %s",
			im.Name, dac, strings.Join(im.DACNamespaces(), ", "))
	}
	im.Namespace, im.Copies = dac, nil
	return im, nil
}

// MetricsCategories are the categories whose rows have a dashboard.
var MetricsCategories = []domain.Category{
	domain.DedicatedAICluster, domain.ImportedModel, domain.GPUWorkload, domain.InferenceService,
}

// CatalogFor reports which model catalog PlanFor needs loaded for item:
// need is false when none is (a DAC without a model, an imported model,
// whose capabilities are inline, or a row without metrics).
func CatalogFor(item any) (domain.Category, bool) {
	switch it := item.(type) {
	case *models.DedicatedAICluster:
		if it == nil || it.ModelName == "" {
			return domain.BaseModel, false
		}
		return modelCatalog(it.ModelName), true
	case *models.GPUWorkload:
		if it == nil {
			return domain.BaseModel, false
		}
		return servingCatalog(models.IsDACName(it.Namespace), it.Model)
	case *models.InferenceService:
		if it == nil {
			return domain.BaseModel, false
		}
		return servingCatalog(it.IsDedicated(), it.Model)
	default: // *models.ImportedModel (capabilities inline) or unknown
		return domain.BaseModel, false
	}
}

// servingCatalog is CatalogFor for a serving pod or endpoint: a dedicated
// one resolves its model by name, an on-demand one against the base
// catalog. No model means no metrics, so no catalog.
func servingCatalog(dedicated bool, model string) (domain.Category, bool) {
	switch {
	case model == "":
		return domain.BaseModel, false
	case dedicated:
		return modelCatalog(model), true
	default:
		return domain.BaseModel, true
	}
}

// modelCatalog routes a model NAME to the catalog that holds it: imported/
// finetune names carry the imported-model prefix, everything else is base.
func modelCatalog(modelName string) domain.Category {
	if models.IsImportedModelName(modelName) {
		return domain.ImportedModel
	}
	return domain.BaseModel
}

// PlanFor maps a row (*models.DedicatedAICluster, *models.ImportedModel,
// *models.GPUWorkload or *models.InferenceService) to its dashboard plan,
// in env, with catalog holding what CatalogFor names (nil when nothing).
// The TUI's metrics key, `toolkit metrics` and MCP's metrics_url all plan
// through it. An error other than ErrNoMetrics and *SeveralDACsError says
// why this row has no dashboard, for the user.
//
//nolint:cyclop // per-row-type resolution; the switch is the routing surface
func PlanFor(item any, env models.Environment, catalog Catalog) (Plan, error) {
	if catalog == nil {
		catalog = &models.Dataset{} // no catalog loaded: no model is found
	}
	realm, region := env.Realm, env.Region
	switch it := item.(type) {
	case *models.DedicatedAICluster:
		if it == nil {
			return Plan{}, ErrNoMetrics
		}
		filter := Filter{Key: FilterDacID, Value: it.OCID(realm, region)}
		if it.ModelName == "" {
			return Plan{Filter: filter, Capability: CapabilityChat}, nil
		}
		return dedicatedPlan(filter, catalog.FindModelByName(it.ModelName))
	case *models.ImportedModel:
		if it == nil {
			return Plan{}, ErrNoMetrics
		}
		// A DAC serving the model holds a copy of it in its namespace; the
		// primary copy usually sits in `default`.
		dacs := it.DACNamespaces()
		switch len(dacs) {
		case 0:
			return Plan{}, errors.New("imported model is not tied to a dedicated AI cluster")
		case 1:
		default:
			return Plan{}, &SeveralDACsError{Model: it.Name, DACs: dacs}
		}
		ocid := models.DedicatedAICluster{Name: dacs[0]}.OCID(realm, region)
		return dedicatedPlan(Filter{Key: FilterDacID, Value: ocid}, &it.BaseModel)
	case *models.GPUWorkload:
		if it == nil {
			return Plan{}, ErrNoMetrics
		}
		if it.Model == "" {
			return Plan{}, errors.New("workload has no model")
		}
		return servingPlan(env, catalog, models.IsDACName(it.Namespace), it.Namespace, it.Model)
	case *models.InferenceService:
		if it == nil {
			return Plan{}, ErrNoMetrics
		}
		if it.Model == "" {
			return Plan{}, errors.New("inference service has no model")
		}
		return servingPlan(env, catalog, it.IsDedicated(), it.Namespace, it.Model)
	default:
		return Plan{}, ErrNoMetrics
	}
}

// servingPlan is PlanFor for a serving pod or endpoint with a model. A
// dedicated one lives in its DAC's namespace, so it is DacId-scoped; an
// on-demand one is scoped by its base model's display name.
func servingPlan(env models.Environment, catalog Catalog, dedicated bool, namespace, model string) (Plan, error) {
	if dedicated {
		ocid := models.DedicatedAICluster{Name: namespace}.OCID(env.Realm, env.Region)
		return dedicatedPlan(Filter{Key: FilterDacID, Value: ocid}, catalog.FindModelByName(model))
	}
	bm := catalog.FindBaseModelByName(model)
	if bm == nil {
		return Plan{}, errors.New("model not found in base catalog")
	}
	capability := CapabilityFor(bm)
	if !capability.Supported() {
		return Plan{}, errors.New("metrics not supported for this model")
	}
	return Plan{Filter: Filter{Key: FilterResourceID, Value: bm.DisplayName}, Capability: capability}, nil
}

// dedicatedPlan finalizes a DacId-filtered (dedicated-mode) plan. A
// capability with no dashboard at all is unsupported; an unfilterable one
// (the fixed, unfiltered moderation sets) cannot be DacId-scoped, so it
// has no dedicated-mode dashboard.
func dedicatedPlan(filter Filter, model *models.BaseModel) (Plan, error) {
	capability := CapabilityFor(model)
	switch {
	case !capability.Supported():
		return Plan{}, errors.New("metrics not supported for this model")
	case !capability.Filterable():
		return Plan{}, errors.New("metrics not available for this model in dedicated mode")
	default:
		return Plan{Filter: filter, Capability: capability}, nil
	}
}

// DashboardURL is plan's OCI Telemetry MQL dashboard in env (region and
// fleet), over the Window ending at now.
func DashboardURL(env models.Environment, plan Plan, now time.Time) string {
	fleet := "generative-ai-service-api-" + env.Type
	return MetricsURL(plan.Filter, plan.Capability, env.Region, Project, fleet, now.Add(-Window), now)
}

// capabilityPrecedence maps a model-capability string to its telemetry
// capability, in priority order: the first capability the model declares
// wins. CHAT is highest; the four unsupported capabilities are last, so any
// supported capability outranks them (a purely-unsupported model resolves to
// CapabilityUnsupported). Synonyms (EMBEDDING, CONTENT_MODERATION,
// IMAGE_TEXT_TO_TEXT) map to the same telemetry capability as their primary.
var capabilityPrecedence = []struct {
	flag string
	cap  Capability
}{
	{models.CapabilityChat, CapabilityChat},
	{models.CapabilityTextToText, CapabilityChat},
	{models.CapabilityImageTextToText, CapabilityChat},
	{models.CapabilityTextRerank, CapabilityTextRerank},
	{models.CapabilityTextEmbeddings, CapabilityTextEmbeddings},
	{models.CapabilityEmbedding, CapabilityTextEmbeddings},
	{models.CapabilityTextToImage, CapabilityTextToImage},
	{models.CapabilityImageTextToImage, CapabilityImageTextToImage},
	{models.CapabilityTextToAudio, CapabilityTextToAudio},
	{models.CapabilityAudioToText, CapabilityAudioToText},
	{models.CapabilityTextClassification, CapabilityTextClassification},
	{models.CapabilityContentModeration, CapabilityTextClassification},
	{models.CapabilityImageContentModeration, CapabilityImageContentModeration},
	{models.CapabilityTextGeneration, CapabilityUnsupported},
	{models.CapabilityAudioToAudio, CapabilityUnsupported},
	{models.CapabilityRealtime, CapabilityUnsupported},
	{models.CapabilityPromptInjectionProtection, CapabilityUnsupported},
}

// CapabilityFor maps a resolved model to its metric capability via
// capabilityPrecedence (first declared match wins). nil / finetune / a model
// declaring no recognized capability fall back to CapabilityChat.
func CapabilityFor(model *models.BaseModel) Capability {
	if model == nil || model.Type == "Fine-tuning" {
		return CapabilityChat
	}
	for _, p := range capabilityPrecedence {
		if model.HasCapability(p.flag) {
			return p.cap
		}
	}
	return CapabilityChat
}

// capabilityNames are the capabilities as `toolkit metrics` and MCP's
// metrics_url name them.
var capabilityNames = map[Capability]string{
	CapabilityChat:                   "chat",
	CapabilityTextRerank:             "text_rerank",
	CapabilityTextEmbeddings:         "text_embeddings",
	CapabilityTextClassification:     "text_classification",
	CapabilityImageContentModeration: "image_content_moderation",
	CapabilityTextToImage:            "text_to_image",
	CapabilityImageTextToImage:       "image_text_to_image",
	CapabilityTextToAudio:            "text_to_audio",
	CapabilityAudioToText:            "audio_to_text",
	CapabilityUnsupported:            "unsupported",
}

// String names c, e.g. "text_rerank".
func (c Capability) String() string {
	if n, ok := capabilityNames[c]; ok {
		return n
	}
	return fmt.Sprintf("Capability(%d)", int(c))
}

// PlanOn is PlanFor where the caller names, as dac, which DAC an imported
// model several DACs serve is viewed on ("" when it isn't choosing): the
// CLI's --dac and MCP's dac, where the TUI shows a picker.
func PlanOn(item any, env models.Environment, catalog Catalog, dac string) (Plan, error) {
	if dac != "" {
		im, ok := item.(*models.ImportedModel)
		if !ok || im == nil {
			return Plan{}, errors.New("a DAC is chosen only for an imported model")
		}
		one, err := OnDAC(*im, dac)
		if err != nil {
			return Plan{}, err
		}
		item = &one
	}
	return PlanFor(item, env, catalog)
}
