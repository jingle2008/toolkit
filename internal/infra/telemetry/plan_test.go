package telemetry

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/pkg/models"
)

func TestCapabilityFor(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		model *models.BaseModel
		want  Capability
	}{
		{"nil", nil, CapabilityChat},
		{"finetune", &models.BaseModel{Type: "Fine-tuning", Capabilities: []string{"TEXT_RERANK"}}, CapabilityChat},
		{"chat", &models.BaseModel{Capabilities: []string{"CHAT"}}, CapabilityChat},
		{"rerank", &models.BaseModel{Capabilities: []string{"TEXT_RERANK"}}, CapabilityTextRerank},
		{"embed", &models.BaseModel{Capabilities: []string{"TEXT_EMBEDDINGS"}}, CapabilityTextEmbeddings},
		// Precedence CHAT > RERANK > EMBED when several are present.
		{"multi", &models.BaseModel{Capabilities: []string{"TEXT_EMBEDDINGS", "CHAT"}}, CapabilityChat},
		{"unknown", &models.BaseModel{Capabilities: []string{"SOMETHING_ELSE"}}, CapabilityChat},
		{"classification", &models.BaseModel{Capabilities: []string{"TEXT_CLASSIFICATION"}}, CapabilityTextClassification},
		{"imagemod", &models.BaseModel{Capabilities: []string{"IMAGE_CONTENT_MODERATION"}}, CapabilityImageContentModeration},
		{"chat-over-classification", &models.BaseModel{Capabilities: []string{"TEXT_CLASSIFICATION", "CHAT"}}, CapabilityChat},
		{"embed-over-classification", &models.BaseModel{Capabilities: []string{"TEXT_CLASSIFICATION", "TEXT_EMBEDDINGS"}}, CapabilityTextEmbeddings},
		{"classification-over-imagemod", &models.BaseModel{Capabilities: []string{"IMAGE_CONTENT_MODERATION", "TEXT_CLASSIFICATION"}}, CapabilityTextClassification},
		{"text_to_text", &models.BaseModel{Capabilities: []string{"TEXT_TO_TEXT"}}, CapabilityChat},
		{"image_text_to_text", &models.BaseModel{Capabilities: []string{"IMAGE_TEXT_TO_TEXT"}}, CapabilityChat},
		{"chat-over-t2t", &models.BaseModel{Capabilities: []string{"TEXT_TO_TEXT", "CHAT"}}, CapabilityChat},
		{"embedding-synonym", &models.BaseModel{Capabilities: []string{"EMBEDDING"}}, CapabilityTextEmbeddings},
		{"content-moderation-synonym", &models.BaseModel{Capabilities: []string{"CONTENT_MODERATION"}}, CapabilityTextClassification},
		{"text_to_image", &models.BaseModel{Capabilities: []string{"TEXT_TO_IMAGE"}}, CapabilityTextToImage},
		{"image_text_to_image", &models.BaseModel{Capabilities: []string{"IMAGE_TEXT_TO_IMAGE"}}, CapabilityImageTextToImage},
		{"text_to_audio", &models.BaseModel{Capabilities: []string{"TEXT_TO_AUDIO"}}, CapabilityTextToAudio},
		{"audio_to_text", &models.BaseModel{Capabilities: []string{"AUDIO_TO_TEXT"}}, CapabilityAudioToText},
		{"unsupported-only", &models.BaseModel{Capabilities: []string{"REALTIME"}}, CapabilityUnsupported},
		{"supported-wins", &models.BaseModel{Capabilities: []string{"TEXT_GENERATION", "CHAT"}}, CapabilityChat},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, CapabilityFor(tc.model))
		})
	}
}

func TestCapability_String(t *testing.T) {
	t.Parallel()
	for c := CapabilityChat; c <= CapabilityUnsupported; c++ {
		assert.NotContains(t, c.String(), "Capability(", "capability %d has no name", int(c))
	}
	assert.Equal(t, "text_rerank", CapabilityTextRerank.String())
}

// An imported model several DACs serve has no plan until one is named;
// naming one it isn't on is an error.
func TestPlanOn_SeveralDACs(t *testing.T) {
	t.Parallel()
	env := models.Environment{Realm: "oc1", Region: "me-abudhabi-1", Type: "prod"}
	im := &models.ImportedModel{
		BaseModel: models.BaseModel{Name: "amaaaaaaim", Capabilities: []string{"TEXT_RERANK"}},
		Namespace: "default", Copies: []string{"amaaaaaadac1", "amaaaaaadac2"},
	}
	_, err := PlanOn(im, env, nil, "")
	var several *SeveralDACsError
	require.ErrorAs(t, err, &several)
	assert.Equal(t, []string{"amaaaaaadac1", "amaaaaaadac2"}, several.DACs)

	plan, err := PlanOn(im, env, nil, "amaaaaaadac2")
	require.NoError(t, err)
	assert.Equal(t, Plan{
		Filter:     Filter{Key: FilterDacID, Value: "ocid1.generativeaidedicatedaicluster.oc1.me-abudhabi-1.amaaaaaadac2"},
		Capability: CapabilityTextRerank,
	}, plan)

	_, err = PlanOn(im, env, nil, "amaaaaaadac9")
	require.ErrorContains(t, err, "is not on DAC amaaaaaadac9; it is on: amaaaaaadac1, amaaaaaadac2")
	_, err = PlanOn(&models.DedicatedAICluster{Name: "d"}, env, nil, "amaaaaaadac1")
	require.ErrorContains(t, err, "only for an imported model")
}

func TestPlanFor_NoMetrics(t *testing.T) {
	t.Parallel()
	_, err := PlanFor(&models.GPUNode{}, models.Environment{}, nil)
	require.ErrorIs(t, err, ErrNoMetrics)
	_, err = PlanFor((*models.DedicatedAICluster)(nil), models.Environment{}, nil)
	require.ErrorIs(t, err, ErrNoMetrics)
}
