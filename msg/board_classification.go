package msg

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

// Board classification: a kanban-store board keeps a taxonomy and a policy,
// and a classifier publishes one immutable decision per card it classified.
// kanban-store owns and stores these records; the types live here because
// llm-bridge-server reads the taxonomy and policy and writes the decisions,
// and neither side may keep its own copy of the shape.
//
// The digests below are versioned encodings, not hashes of whatever JSON a
// caller happened to send: each names its version in its prefix, fixes the
// fields it covers in its own struct, and is pinned by a fixture in testdata.
// A change to what a digest covers is a new version, never an edit to v1.

// hasSelectableValue reports whether an axis has a value a classifier may
// still choose.
func hasSelectableValue(axis ClassificationAxis) bool {
	for _, value := range axis.Values {
		if !value.Archived {
			return true
		}
	}
	return false
}

// Selectable is the taxonomy as a classifier is given it: archived axes and
// archived values left out. The archived ones stay in the stored taxonomy so
// that history resolves; nothing new may be labelled with them.
func (taxonomy ClassificationTaxonomy) Selectable() ClassificationTaxonomy {
	out := ClassificationTaxonomy{Name: taxonomy.Name, Domain: taxonomy.Domain, Axes: []ClassificationAxis{}}
	for _, axis := range taxonomy.Axes {
		if axis.Archived {
			continue
		}
		kept := axis
		kept.Values = []ClassificationValue{}
		for _, value := range axis.Values {
			if !value.Archived {
				kept.Values = append(kept.Values, value)
			}
		}
		out.Axes = append(out.Axes, kept)
	}
	return out
}

// ValidateStableIdentity refuses a taxonomy whose axes and values do not all
// carry an id, or that uses one id twice. A board's taxonomy must pass it; an
// inline taxonomy on a classification.run need not.
func (taxonomy *ClassificationTaxonomy) ValidateStableIdentity() error {
	seen := map[string]string{}
	claim := func(id, what string) error {
		if strings.TrimSpace(id) == "" || id != strings.TrimSpace(id) {
			return fmt.Errorf("%s has a blank or untrimmed id %q", what, id)
		}
		if earlier, taken := seen[id]; taken {
			return fmt.Errorf("%s and %s share the id %s", earlier, what, id)
		}
		seen[id] = what
		return nil
	}
	for _, axis := range taxonomy.Axes {
		if err := claim(axis.ID, fmt.Sprintf("axis %q", axis.Name)); err != nil {
			return err
		}
		for _, value := range axis.Values {
			if err := claim(value.ID, fmt.Sprintf("axis %q value %q", axis.Name, value.Name)); err != nil {
				return err
			}
		}
	}
	return nil
}

// AxisByID and ValueByID find an entry by its stable id, archived or not.
func (taxonomy ClassificationTaxonomy) AxisByID(axisID string) (ClassificationAxis, bool) {
	for _, axis := range taxonomy.Axes {
		if axis.ID == axisID {
			return axis, true
		}
	}
	return ClassificationAxis{}, false
}

func (axis ClassificationAxis) ValueByID(valueID string) (ClassificationValue, bool) {
	for _, value := range axis.Values {
		if value.ID == valueID {
			return value, true
		}
	}
	return ClassificationValue{}, false
}

// ClassificationTaxonomyDigestVersion prefixes every taxonomy digest.
const ClassificationTaxonomyDigestVersion = "classification-taxonomy-v1"

// SemanticDigest names what a classifier is given by this taxonomy: the
// selectable axes and values, in order, with their ids, names, descriptions,
// cardinality and requiredness, and the domain line. The taxonomy's own Name
// is left out: no classifier is shown it, so renaming the taxonomy does not
// make earlier decisions stale. Archiving an entry changes the digest, because
// it changes what a classifier may answer. Every entry must carry an id.
func (taxonomy ClassificationTaxonomy) SemanticDigest() (string, error) {
	if err := taxonomy.ValidateStableIdentity(); err != nil {
		return "", err
	}
	type valueV1 struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	type axisV1 struct {
		ID            string    `json:"id"`
		Name          string    `json:"name"`
		Description   string    `json:"description"`
		AllowMultiple bool      `json:"allow_multiple"`
		Required      bool      `json:"required"`
		Values        []valueV1 `json:"values"`
	}
	type taxonomyV1 struct {
		Domain string   `json:"domain"`
		Axes   []axisV1 `json:"axes"`
	}
	selectable := taxonomy.Selectable()
	encoding := taxonomyV1{Domain: selectable.Domain, Axes: []axisV1{}}
	for _, axis := range selectable.Axes {
		encoded := axisV1{ID: axis.ID, Name: axis.Name, Description: axis.Description,
			AllowMultiple: axis.AllowMultiple, Required: axis.Required, Values: []valueV1{}}
		for _, value := range axis.Values {
			encoded.Values = append(encoded.Values, valueV1{ID: value.ID, Name: value.Name, Description: value.Description})
		}
		encoding.Axes = append(encoding.Axes, encoded)
	}
	return versionedDigest(ClassificationTaxonomyDigestVersion, encoding)
}

// versionedDigest is "<version>:sha256:<hex>" of the JSON encoding of a struct
// whose field order the caller fixes.
func versionedDigest(version string, encoding any) (string, error) {
	encoded, err := json.Marshal(encoding)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(encoded)
	return version + ":sha256:" + hex.EncodeToString(sum[:]), nil
}

// ClassificationAxisMode is what a board does with a classifier's proposals on
// one axis. GET /api/boards/{id}/classification serves the modes the board
// accepts, so no client keeps its own list.
type ClassificationAxisMode string

const (
	// ClassificationAxisModeOff: the axis is not classified.
	ClassificationAxisModeOff ClassificationAxisMode = "off"
	// ClassificationAxisModeDiscovery: proposals are recorded and shown, and
	// nothing is reviewed or applied from them.
	ClassificationAxisModeDiscovery ClassificationAxisMode = "discovery"
	// ClassificationAxisModeAssisted: proposals wait for a person, and only a
	// person's accept or correction sets the card's label.
	ClassificationAxisModeAssisted ClassificationAxisMode = "assisted"
)

// ClassificationAxisModes is every mode a board may set. There is no "auto":
// applying a proposal with nobody looking needs its own allowlist, revocation
// and recovery rules first.
var ClassificationAxisModes = []ClassificationAxisMode{ClassificationAxisModeOff, ClassificationAxisModeDiscovery, ClassificationAxisModeAssisted}

// ClassificationAxisPolicy is a board's rule for one axis.
type ClassificationAxisPolicy struct {
	Mode ClassificationAxisMode `json:"mode"`
}

// BoardClassificationPolicy is what a board does with classification, per
// axis, keyed by the axis's stable id. An axis with no entry is unconfigured
// and is treated as off; it never inherits another board's rule. Revision
// counts the board's policies from 1; a board that never set one has none.
type BoardClassificationPolicy struct {
	BoardID      string                              `json:"board_id"`
	Revision     int64                               `json:"revision"`
	AxisPolicies map[string]ClassificationAxisPolicy `json:"axis_policies"`
	UpdatedAt    time.Time                           `json:"updated_at"`
	UpdatedBy    string                              `json:"updated_by"`
}

// ValidateAxisPolicies refuses a policy naming an axis the taxonomy does not
// have, or a mode that is not in ClassificationAxisModes. An archived axis
// may only be off.
func ValidateAxisPolicies(axisPolicies map[string]ClassificationAxisPolicy, taxonomy ClassificationTaxonomy) error {
	for axisID, policy := range axisPolicies {
		axis, found := taxonomy.AxisByID(axisID)
		if !found {
			return fmt.Errorf("axis_policies names axis %s, which the board's taxonomy does not have", axisID)
		}
		known := false
		for _, mode := range ClassificationAxisModes {
			if policy.Mode == mode {
				known = true
			}
		}
		if !known {
			return fmt.Errorf("axis %s has mode %q; the modes are %v", axisID, policy.Mode, ClassificationAxisModes)
		}
		if axis.Archived && policy.Mode != ClassificationAxisModeOff {
			return fmt.Errorf("axis %s is archived and may only be off", axisID)
		}
	}
	return nil
}

// ModeOf is an axis's mode under the policy: off when the policy is absent or
// has no entry for it.
func (policy *BoardClassificationPolicy) ModeOf(axisID string) ClassificationAxisMode {
	if policy == nil {
		return ClassificationAxisModeOff
	}
	axisPolicy, configured := policy.AxisPolicies[axisID]
	if !configured {
		return ClassificationAxisModeOff
	}
	return axisPolicy.Mode
}

// BoardClassificationAction is one thing a caller may do with a board's
// classification. Each is judged by kanban-store on its own.
type BoardClassificationAction string

const (
	// Read decisions, reviews and labels: can_view.
	BoardClassificationActionRead BoardClassificationAction = "read"
	// Ask for cards to be classified: can_edit.
	BoardClassificationActionClassify BoardClassificationAction = "classify"
	// Accept, correct or reject a decision, which sets the card's label: can_edit.
	BoardClassificationActionReview BoardClassificationAction = "review"
	// Change the taxonomy or the policy: can_administer.
	BoardClassificationActionManage BoardClassificationAction = "manage"
)

// BoardClassification is a board's classification settings as
// GET /api/boards/{id}/classification serves them.
type BoardClassification struct {
	BoardID string `json:"board_id"`
	// OrganizationID is the principal-store group whose operations budget and
	// grants classifying this board runs under. Empty means none is set, and
	// a board classification refuses to start.
	OrganizationID string `json:"organization_id,omitempty"`
	// Taxonomy is the current revision, archived entries included. Nil when
	// the board has none; a classification then fails rather than borrowing one.
	Taxonomy *ClassificationTaxonomy `json:"taxonomy"`
	// TaxonomyRevision counts the board's taxonomies from 1; 0 means the board
	// never had one. It is what a PUT sends back in If-Match.
	TaxonomyRevision  int64      `json:"taxonomy_revision"`
	TaxonomyDigest    string     `json:"taxonomy_digest,omitempty"`
	TaxonomyUpdatedAt *time.Time `json:"taxonomy_updated_at,omitempty"`
	TaxonomyUpdatedBy string     `json:"taxonomy_updated_by,omitempty"`
	// Policy is nil when the board has none: unconfigured, not defaulted.
	Policy         *BoardClassificationPolicy  `json:"policy"`
	SupportedModes []ClassificationAxisMode    `json:"supported_modes"`
	CallerActions  []BoardClassificationAction `json:"caller_actions"`
}

// ClassificationSourceSnapshot is everything a classifier was given about one
// card, exactly as given. Nothing in it is cut or rewritten; an item too long
// to send is refused, not shortened.
type ClassificationSourceSnapshot struct {
	// SourceRevision is the owning record's version when it was read
	// (noteboard's updated_at for a card). It says where the text came from;
	// it is not part of the digest, which covers the text itself.
	SourceRevision string     `json:"source_revision,omitempty"`
	Subject        string     `json:"subject"`
	Body           string     `json:"body"`
	Sender         string     `json:"sender,omitempty"`
	Recipients     []string   `json:"recipients,omitempty"`
	ReceivedAt     *time.Time `json:"received_at,omitempty"`
	// ModelContext is any other text the classifier was shown with the item.
	ModelContext string `json:"model_context,omitempty"`
}

// ClassificationSourceDigestVersion prefixes every source digest.
const ClassificationSourceDigestVersion = "classification-source-v1"

// Digest covers every field the classifier was shown. Recipients keep their
// order, since that is how they were shown.
func (snapshot ClassificationSourceSnapshot) Digest() (string, error) {
	type sourceV1 struct {
		Subject      string   `json:"subject"`
		Body         string   `json:"body"`
		Sender       string   `json:"sender"`
		Recipients   []string `json:"recipients"`
		ReceivedAt   string   `json:"received_at"`
		ModelContext string   `json:"model_context"`
	}
	encoding := sourceV1{Subject: snapshot.Subject, Body: snapshot.Body, Sender: snapshot.Sender,
		Recipients: append([]string{}, snapshot.Recipients...), ModelContext: snapshot.ModelContext}
	if snapshot.ReceivedAt != nil {
		encoding.ReceivedAt = snapshot.ReceivedAt.UTC().Format(time.RFC3339Nano)
	}
	return versionedDigest(ClassificationSourceDigestVersion, encoding)
}

// ClassificationDecisionOrigin is what produced a decision.
type ClassificationDecisionOrigin string

// ClassificationDecisionOriginModel: a model answered, through a bridge
// operation. Rules or trained models will need their own origin and evidence.
const ClassificationDecisionOriginModel ClassificationDecisionOrigin = "model"

// ClassificationModelEvidence is which model was asked for and which answered.
type ClassificationModelEvidence struct {
	// RequestedModel is what the operation asked for: a model id, alias or
	// role ("efficient"). Empty means the bridge's configured default.
	RequestedModel string `json:"requested_model,omitempty"`
	// ResolvedModelID is the model-store id the request resolved to.
	ResolvedModelID string `json:"resolved_model_id"`
	Provider        string `json:"provider,omitempty"`
	// AnsweredByModel is the model the harness reported answering. Empty means
	// it did not say, and nothing may fill it in from a display label.
	AnsweredByModel string `json:"answered_by_model,omitempty"`
}

// BoardClassificationDecisionPublication is what a classifier sends
// kanban-store for one card: POST /api/boards/{id}/classification/decisions,
// with the service token. The store derives the publication key from it (see
// PublicationKey), so sending it twice returns the first decision, and sending
// a different answer under the same key is a conflict.
type BoardClassificationDecisionPublication struct {
	BoardID string `json:"board_id"`
	CardID  string `json:"card_id"`
	// OperationID is the bridge operation that ran the classifier;
	// CheckpointID and AttemptNumber name the durable batch and the attempt
	// whose answer this is.
	OperationID   string `json:"operation_id"`
	CheckpointID  string `json:"checkpoint_id"`
	AttemptNumber int    `json:"attempt_number"`
	// InitiatingPrincipalID asked for the classification. The store checks,
	// when it publishes, that this principal may still classify the board.
	InitiatingPrincipalID string                       `json:"initiating_principal_id"`
	Source                ClassificationSourceSnapshot `json:"source"`
	// SourceDigest must equal Source.Digest(); the store recomputes it.
	SourceDigest string `json:"source_digest"`
	// TaxonomyRevision is the board's taxonomy revision the classifier was
	// given, and TaxonomyDigest its SemanticDigest; the store refuses a pair
	// that does not match its own record.
	TaxonomyRevision int64  `json:"taxonomy_revision"`
	TaxonomyDigest   string `json:"taxonomy_digest"`
	PolicyRevision   int64  `json:"policy_revision"`
	// PromptRevision names the classifier's instructions and answer schema.
	PromptRevision string                       `json:"prompt_revision"`
	Model          ClassificationModelEvidence  `json:"model"`
	Origin         ClassificationDecisionOrigin `json:"origin"`
	// Selections maps each axis id the classifier was asked about to the
	// value ids it chose. An axis it was asked about and answered nothing on
	// is present with an empty list.
	Selections map[string][]string `json:"selections"`
	Rationale  string              `json:"rationale,omitempty"`
	// ModelReportedConfidence is the model's own number for the whole item,
	// between 0 and 1: not per axis and not a measured accuracy. Nil when the
	// model gave none.
	ModelReportedConfidence *float64 `json:"model_reported_confidence,omitempty"`
	// NeedsReview is set when the answer left a required axis empty or broke
	// the taxonomy, and ReviewReasons say how.
	NeedsReview   bool                `json:"needs_review,omitempty"`
	ReviewReasons []OperationEvidence `json:"review_reasons,omitempty"`
	CompletedAt   time.Time           `json:"completed_at"`
}

// ClassificationPublicationKeyVersion prefixes every publication key.
const ClassificationPublicationKeyVersion = "classification-publication-v1"

// PublicationKey is the identity of one answer: this operation, this card,
// this source text, under this taxonomy, policy and prompt. The same key twice
// is the same decision.
func (publication BoardClassificationDecisionPublication) PublicationKey() (string, error) {
	type keyV1 struct {
		OperationID      string `json:"operation_id"`
		BoardID          string `json:"board_id"`
		CardID           string `json:"card_id"`
		SourceDigest     string `json:"source_digest"`
		TaxonomyRevision int64  `json:"taxonomy_revision"`
		PolicyRevision   int64  `json:"policy_revision"`
		PromptRevision   string `json:"prompt_revision"`
	}
	return versionedDigest(ClassificationPublicationKeyVersion, keyV1{
		OperationID: publication.OperationID, BoardID: publication.BoardID, CardID: publication.CardID,
		SourceDigest: publication.SourceDigest, TaxonomyRevision: publication.TaxonomyRevision,
		PolicyRevision: publication.PolicyRevision, PromptRevision: publication.PromptRevision,
	})
}

// Validate checks what can be judged from the publication alone, against the
// taxonomy revision it names: required fields, the source digest, and that
// every selection is an axis and values of that taxonomy, within the axis's
// cardinality. Whether the store's own records agree is the store's check.
func (publication *BoardClassificationDecisionPublication) Validate(taxonomy ClassificationTaxonomy) error {
	for field, value := range map[string]string{
		"board_id": publication.BoardID, "card_id": publication.CardID, "operation_id": publication.OperationID,
		"checkpoint_id": publication.CheckpointID, "initiating_principal_id": publication.InitiatingPrincipalID,
		"prompt_revision": publication.PromptRevision, "model.resolved_model_id": publication.Model.ResolvedModelID,
	} {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s is required", field)
		}
	}
	if publication.AttemptNumber < 1 {
		return errors.New("attempt_number starts at 1")
	}
	if publication.TaxonomyRevision < 1 {
		return errors.New("taxonomy_revision is required")
	}
	if publication.PolicyRevision < 1 {
		return errors.New("policy_revision is required: a board with no policy is not classified")
	}
	if publication.Origin != ClassificationDecisionOriginModel {
		return fmt.Errorf("origin %q is not one this store records; the origins are [%s]", publication.Origin, ClassificationDecisionOriginModel)
	}
	if publication.CompletedAt.IsZero() {
		return errors.New("completed_at is required")
	}
	if confidence := publication.ModelReportedConfidence; confidence != nil && (*confidence < 0 || *confidence > 1) {
		return fmt.Errorf("model_reported_confidence %v is outside 0..1", *confidence)
	}
	digest, err := publication.Source.Digest()
	if err != nil {
		return err
	}
	if digest != publication.SourceDigest {
		return fmt.Errorf("source_digest %s does not match the source sent, whose digest is %s", publication.SourceDigest, digest)
	}
	if len(publication.Selections) == 0 {
		return errors.New("selections is empty: name every axis the classifier was asked about")
	}
	axisIDs := make([]string, 0, len(publication.Selections))
	for axisID := range publication.Selections {
		axisIDs = append(axisIDs, axisID)
	}
	sort.Strings(axisIDs)
	for _, axisID := range axisIDs {
		axis, found := taxonomy.AxisByID(axisID)
		if !found {
			return fmt.Errorf("selections names axis %s, which taxonomy revision %d does not have", axisID, publication.TaxonomyRevision)
		}
		if axis.Archived {
			return fmt.Errorf("selections names axis %s, which is archived in taxonomy revision %d", axisID, publication.TaxonomyRevision)
		}
		if err := ValidateAxisSelection(axis, publication.Selections[axisID]); err != nil {
			return err
		}
	}
	return nil
}

// ValidateAxisSelection refuses value ids the axis does not have or has
// archived, a value named twice, and more than one value on a one-value axis.
// An empty selection is allowed: a required axis left empty is recorded and
// marked for review, never filled in.
func ValidateAxisSelection(axis ClassificationAxis, valueIDs []string) error {
	seen := map[string]bool{}
	for _, valueID := range valueIDs {
		value, found := axis.ValueByID(valueID)
		if !found {
			return fmt.Errorf("axis %s has no value %s", axis.ID, valueID)
		}
		if value.Archived {
			return fmt.Errorf("axis %s value %s is archived and cannot be chosen", axis.ID, valueID)
		}
		if seen[valueID] {
			return fmt.Errorf("axis %s names value %s twice", axis.ID, valueID)
		}
		seen[valueID] = true
	}
	if !axis.AllowMultiple && len(valueIDs) > 1 {
		return fmt.Errorf("axis %s takes one value and was given %d", axis.ID, len(valueIDs))
	}
	return nil
}
