package msg

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"
)

// digestFixture pins the v1 encodings. A digest that moves here means stored
// decisions no longer match their own records: make a v2, never edit v1.
type digestFixture struct {
	Taxonomy               ClassificationTaxonomy                 `json:"taxonomy"`
	TaxonomyDigest         string                                 `json:"taxonomy_digest"`
	Source                 ClassificationSourceSnapshot           `json:"source"`
	SourceDigest           string                                 `json:"source_digest"`
	Publication            BoardClassificationDecisionPublication `json:"publication"`
	PublicationKey         string                                 `json:"publication_key"`
	ArchivedTaxonomyDigest string                                 `json:"archived_taxonomy_digest"`
}

func readDigestFixture(t *testing.T) digestFixture {
	t.Helper()
	raw, err := os.ReadFile("testdata/board_classification_digests_v1.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture digestFixture
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	return fixture
}

func TestDigestsMatchTheV1Fixture(t *testing.T) {
	fixture := readDigestFixture(t)
	taxonomyDigest, err := fixture.Taxonomy.SemanticDigest()
	if err != nil {
		t.Fatal(err)
	}
	if taxonomyDigest != fixture.TaxonomyDigest {
		t.Errorf("taxonomy digest moved: fixture %s, now %s", fixture.TaxonomyDigest, taxonomyDigest)
	}
	sourceDigest, err := fixture.Source.Digest()
	if err != nil {
		t.Fatal(err)
	}
	if sourceDigest != fixture.SourceDigest {
		t.Errorf("source digest moved: fixture %s, now %s", fixture.SourceDigest, sourceDigest)
	}
	key, err := fixture.Publication.PublicationKey()
	if err != nil {
		t.Fatal(err)
	}
	if key != fixture.PublicationKey {
		t.Errorf("publication key moved: fixture %s, now %s", fixture.PublicationKey, key)
	}
	archived := fixture.Taxonomy
	archived.Axes = append([]ClassificationAxis(nil), fixture.Taxonomy.Axes...)
	archived.Axes[0].Values = append([]ClassificationValue(nil), fixture.Taxonomy.Axes[0].Values...)
	archived.Axes[0].Values[1].Archived = true
	archivedDigest, err := archived.SemanticDigest()
	if err != nil {
		t.Fatal(err)
	}
	if archivedDigest != fixture.ArchivedTaxonomyDigest {
		t.Errorf("archived taxonomy digest moved: fixture %s, now %s", fixture.ArchivedTaxonomyDigest, archivedDigest)
	}
	for _, digest := range []string{taxonomyDigest, sourceDigest, key} {
		if !strings.Contains(digest, "-v1:sha256:") {
			t.Errorf("digest %s does not name its version", digest)
		}
	}
}

func TestTaxonomyDigestMovesOnlyWithWhatAClassifierIsShown(t *testing.T) {
	base := readDigestFixture(t).Taxonomy
	baseDigest, err := base.SemanticDigest()
	if err != nil {
		t.Fatal(err)
	}
	change := func(edit func(*ClassificationTaxonomy)) string {
		raw, _ := json.Marshal(base)
		var copied ClassificationTaxonomy
		json.Unmarshal(raw, &copied)
		edit(&copied)
		digest, err := copied.SemanticDigest()
		if err != nil {
			t.Fatal(err)
		}
		return digest
	}
	if change(func(taxonomy *ClassificationTaxonomy) { taxonomy.Name = "renamed" }) != baseDigest {
		t.Error("renaming the taxonomy itself, which no classifier sees, changed the digest")
	}
	for what, edit := range map[string]func(*ClassificationTaxonomy){
		"a value description": func(taxonomy *ClassificationTaxonomy) { taxonomy.Axes[0].Values[0].Description = "changed" },
		"a value name":        func(taxonomy *ClassificationTaxonomy) { taxonomy.Axes[0].Values[0].Name = "invoices" },
		"the domain":          func(taxonomy *ClassificationTaxonomy) { taxonomy.Domain = "changed" },
		"requiredness":        func(taxonomy *ClassificationTaxonomy) { taxonomy.Axes[0].Required = !taxonomy.Axes[0].Required },
		"archiving an axis":   func(taxonomy *ClassificationTaxonomy) { taxonomy.Axes[1].Archived = true },
		"an id":               func(taxonomy *ClassificationTaxonomy) { taxonomy.Axes[0].ID = "classification_axis_999999" },
		"the order of values": func(taxonomy *ClassificationTaxonomy) {
			taxonomy.Axes[0].Values[0], taxonomy.Axes[0].Values[1] = taxonomy.Axes[0].Values[1], taxonomy.Axes[0].Values[0]
		},
	} {
		if change(edit) == baseDigest {
			t.Errorf("changing %s left the digest where it was", what)
		}
	}
}

func TestSemanticDigestNeedsStableIDs(t *testing.T) {
	taxonomy := ClassificationTaxonomy{Name: "inline", Axes: []ClassificationAxis{{Name: "category", Values: []ClassificationValue{{Name: "billing"}}}}}
	if _, err := taxonomy.SemanticDigest(); err == nil {
		t.Fatal("a taxonomy with no ids got a digest")
	}
	taxonomy.Axes[0].ID, taxonomy.Axes[0].Values[0].ID = "same", "same"
	if err := taxonomy.ValidateStableIdentity(); err == nil {
		t.Fatal("an id used by an axis and a value was accepted")
	}
}

func TestSelectableLeavesArchivedEntriesOut(t *testing.T) {
	taxonomy := readDigestFixture(t).Taxonomy
	taxonomy.Axes[0].Values[1].Archived = true
	taxonomy.Axes[1].Archived = true
	selectable := taxonomy.Selectable()
	if len(selectable.Axes) != 1 || len(selectable.Axes[0].Values) != len(taxonomy.Axes[0].Values)-1 {
		t.Fatalf("selectable kept archived entries: %+v", selectable)
	}
	if len(taxonomy.Axes) != 2 {
		t.Fatal("Selectable changed the taxonomy it was called on")
	}
}

func TestPublicationValidation(t *testing.T) {
	fixture := readDigestFixture(t)
	if err := fixture.Publication.Validate(fixture.Taxonomy); err != nil {
		t.Fatalf("the fixture's publication is refused: %v", err)
	}
	category, urgency := fixture.Taxonomy.Axes[0], fixture.Taxonomy.Axes[1]
	for what, spoil := range map[string]func(*BoardClassificationDecisionPublication){
		"a source edited after digesting": func(publication *BoardClassificationDecisionPublication) { publication.Source.Body += " edited" },
		"an unknown axis": func(publication *BoardClassificationDecisionPublication) {
			publication.Selections["classification_axis_999999"] = []string{}
		},
		"a value of another axis": func(publication *BoardClassificationDecisionPublication) {
			publication.Selections[category.ID] = []string{urgency.Values[0].ID}
		},
		"two values on a one-value axis": func(publication *BoardClassificationDecisionPublication) {
			publication.Selections[urgency.ID] = []string{urgency.Values[0].ID, urgency.Values[1].ID}
		},
		"no policy revision": func(publication *BoardClassificationDecisionPublication) { publication.PolicyRevision = 0 },
		"an unknown origin":  func(publication *BoardClassificationDecisionPublication) { publication.Origin = "rules" },
		"confidence above one": func(publication *BoardClassificationDecisionPublication) {
			high := 1.5
			publication.ModelReportedConfidence = &high
		},
		"no attempt":        func(publication *BoardClassificationDecisionPublication) { publication.AttemptNumber = 0 },
		"no resolved model": func(publication *BoardClassificationDecisionPublication) { publication.Model.ResolvedModelID = "" },
	} {
		raw, _ := json.Marshal(fixture.Publication)
		var copied BoardClassificationDecisionPublication
		json.Unmarshal(raw, &copied)
		spoil(&copied)
		if err := copied.Validate(fixture.Taxonomy); err == nil {
			t.Errorf("%s: accepted", what)
		}
	}
	archived := fixture.Taxonomy
	archived.Axes = append([]ClassificationAxis(nil), fixture.Taxonomy.Axes...)
	archived.Axes[0].Values = append([]ClassificationValue(nil), fixture.Taxonomy.Axes[0].Values...)
	for index := range archived.Axes[0].Values {
		archived.Axes[0].Values[index].Archived = archived.Axes[0].Values[index].ID == fixture.Publication.Selections[category.ID][0]
	}
	if err := fixture.Publication.Validate(archived); err == nil {
		t.Error("a publication choosing an archived value was accepted")
	}
}

func TestPublicationKeyIgnoresTheAnswer(t *testing.T) {
	fixture := readDigestFixture(t)
	first, _ := fixture.Publication.PublicationKey()
	changed := fixture.Publication
	changed.Rationale = "a different answer"
	changed.AttemptNumber = 2
	changed.CompletedAt = changed.CompletedAt.Add(time.Minute)
	second, _ := changed.PublicationKey()
	if first != second {
		t.Fatal("the key moved with the answer; a second answer to the same question would not be seen as a conflict")
	}
	changed.PolicyRevision++
	third, _ := changed.PublicationKey()
	if third == first {
		t.Fatal("the key did not move with the policy revision")
	}
}

func TestAxisPolicyValidation(t *testing.T) {
	taxonomy := readDigestFixture(t).Taxonomy
	taxonomy.Axes[1].Archived = true
	good := map[string]ClassificationAxisPolicy{taxonomy.Axes[0].ID: {Mode: ClassificationAxisModeAssisted}, taxonomy.Axes[1].ID: {Mode: ClassificationAxisModeOff}}
	if err := ValidateAxisPolicies(good, taxonomy); err != nil {
		t.Fatalf("good policy refused: %v", err)
	}
	for what, policies := range map[string]map[string]ClassificationAxisPolicy{
		"auto":                         {taxonomy.Axes[0].ID: {Mode: "auto"}},
		"an unknown axis":              {"classification_axis_999999": {Mode: ClassificationAxisModeOff}},
		"an archived axis switched on": {taxonomy.Axes[1].ID: {Mode: ClassificationAxisModeDiscovery}},
	} {
		if err := ValidateAxisPolicies(policies, taxonomy); err == nil {
			t.Errorf("%s: accepted", what)
		}
	}
	var unconfigured *BoardClassificationPolicy
	if unconfigured.ModeOf(taxonomy.Axes[0].ID) != ClassificationAxisModeOff {
		t.Error("a board with no policy treats an axis as something other than off")
	}
}
