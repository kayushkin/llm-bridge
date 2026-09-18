package servicesettings

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/kayushkin/llm-bridge/msg"
)

func sampleDefinitions() []Definition {
	return []Definition{
		{Key: "listen_address", EnvironmentVariable: "SAMPLE_LISTEN_ADDR", Kind: msg.ServiceSettingKindWiring, ValueType: msg.ServiceSettingValueTypeString, Description: "where it listens", Default: ":9000"},
		{Key: "owner_url", EnvironmentVariable: "SAMPLE_OWNER_URL", Kind: msg.ServiceSettingKindWiring, ValueType: msg.ServiceSettingValueTypeString, Description: "the store every caller is read from", Required: true},
		{Key: "service_token", EnvironmentVariable: "SAMPLE_SERVICE_TOKEN", Kind: msg.ServiceSettingKindSecret, ValueType: msg.ServiceSettingValueTypeString, Description: "what internal callers present"},
		{Key: "classifier.model", EnvironmentVariable: "SAMPLE_CLASSIFIER_MODEL", Kind: msg.ServiceSettingKindBehaviour, ValueType: msg.ServiceSettingValueTypeString, Description: "the model the classifier asks for", Default: "small-model", Editable: true},
		{Key: "classifier.timeout", EnvironmentVariable: "SAMPLE_CLASSIFIER_TIMEOUT", Kind: msg.ServiceSettingKindBehaviour, ValueType: msg.ServiceSettingValueTypeDuration, Description: "how long one call may take", Default: "20s", Editable: true},
		{Key: "classifier.max_characters", EnvironmentVariable: "SAMPLE_CLASSIFIER_MAX_CHARS", Kind: msg.ServiceSettingKindBehaviour, ValueType: msg.ServiceSettingValueTypeInteger, Description: "how much text is sent", Default: "6000"},
	}
}

type memoryStoredValues struct {
	held      map[string]string
	saveError error
}

func (m *memoryStoredValues) Load() (map[string]string, error) {
	copied := make(map[string]string, len(m.held))
	for key, value := range m.held {
		copied[key] = value
	}
	return copied, nil
}

func (m *memoryStoredValues) Save(key, value string) error {
	if m.saveError != nil {
		return m.saveError
	}
	if m.held == nil {
		m.held = make(map[string]string)
	}
	m.held[key] = value
	return nil
}

func mustRegistry(t *testing.T, variables map[string]string) *Registry {
	t.Helper()
	registry, err := New("sample", []string{"SAMPLE_"}, sampleDefinitions(), MapEnvironment(variables))
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

func settingOf(t *testing.T, described msg.ServiceSettings, key string) msg.ServiceSetting {
	t.Helper()
	for _, setting := range described.Settings {
		if setting.Key == key {
			return setting
		}
	}
	t.Fatalf("no setting %s in the description", key)
	return msg.ServiceSetting{}
}

func TestAValueComesFromTheEnvironmentThenTheDefaultAndSaysWhich(t *testing.T) {
	registry := mustRegistry(t, map[string]string{"SAMPLE_OWNER_URL": "http://owner", "SAMPLE_CLASSIFIER_TIMEOUT": "45s"})
	if got := registry.String("listen_address"); got != ":9000" {
		t.Errorf("listen_address = %q, want the default", got)
	}
	if got := registry.Duration("classifier.timeout"); got != 45*time.Second {
		t.Errorf("classifier.timeout = %v, want the environment's 45s", got)
	}
	if got := registry.Integer("classifier.max_characters"); got != 6000 {
		t.Errorf("classifier.max_characters = %d", got)
	}
	described := registry.Describe()
	if source := settingOf(t, described, "listen_address").Source; source != msg.ServiceSettingSourceDefault {
		t.Errorf("listen_address source = %s", source)
	}
	if source := settingOf(t, described, "owner_url").Source; source != msg.ServiceSettingSourceEnvironment {
		t.Errorf("owner_url source = %s", source)
	}
	if source := settingOf(t, described, "service_token").Source; source != msg.ServiceSettingSourceUnset {
		t.Errorf("service_token source = %s", source)
	}
}

func TestStartupNamesEveryFaultAtOnce(t *testing.T) {
	_, err := New("sample", []string{"SAMPLE_"}, sampleDefinitions(), MapEnvironment(map[string]string{
		"SAMPLE_CLASSIFIER_TIMEOUT": "soon",
		"SAMPLE_CLASIFIER_MODEL":    "misspelled-variable",
		"UNRELATED_VARIABLE":        "left alone",
	}))
	if err == nil {
		t.Fatal("a registry was built from a malformed duration and a misspelled variable")
	}
	for _, want := range []string{"SAMPLE_CLASSIFIER_TIMEOUT", "not a duration", "SAMPLE_CLASIFIER_MODEL is set and sample declares no such setting"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the startup error does not mention %q: %v", want, err)
		}
	}
	if strings.Contains(err.Error(), "UNRELATED_VARIABLE") {
		t.Errorf("a variable outside the service's prefixes was reported: %v", err)
	}
}

func TestARequiredSettingIsCheckedByTheServerAndNotByEveryCommandOfTheBinary(t *testing.T) {
	registry, err := New("sample", []string{"SAMPLE_"}, sampleDefinitions(), MapEnvironment(nil))
	if err != nil {
		t.Fatalf("a command with no environment could not read its settings: %v", err)
	}
	if err := registry.CheckRequired(); err == nil || !strings.Contains(err.Error(), "SAMPLE_OWNER_URL is unset") {
		t.Errorf("CheckRequired = %v, want it to name SAMPLE_OWNER_URL", err)
	}
	if err := mustRegistry(t, map[string]string{"SAMPLE_OWNER_URL": "http://owner"}).CheckRequired(); err != nil {
		t.Errorf("CheckRequired with the setting present = %v", err)
	}
}

func TestADeclarationFaultIsRefused(t *testing.T) {
	cases := map[string]Definition{
		"only a behaviour setting is stored": {Key: "a", EnvironmentVariable: "SAMPLE_A", Kind: msg.ServiceSettingKindWiring, ValueType: msg.ServiceSettingValueTypeString, Editable: true},
		"is a secret and declares a default": {Key: "a", EnvironmentVariable: "SAMPLE_A", Kind: msg.ServiceSettingKindSecret, ValueType: msg.ServiceSettingValueTypeString, Default: "hunter2"},
		"its default":                        {Key: "a", EnvironmentVariable: "SAMPLE_A", Kind: msg.ServiceSettingKindBehaviour, ValueType: msg.ServiceSettingValueTypeInteger, Default: "many"},
	}
	for want, definition := range cases {
		_, err := New("sample", nil, []Definition{definition}, MapEnvironment(nil))
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("want a refusal mentioning %q, got %v", want, err)
		}
	}
	twice := []Definition{sampleDefinitions()[0], sampleDefinitions()[0]}
	if _, err := New("sample", nil, twice, MapEnvironment(nil)); err == nil || !strings.Contains(err.Error(), "declared twice") {
		t.Errorf("a key declared twice was accepted: %v", err)
	}
}

func TestASecretIsDescribedAsSetAndNeverShown(t *testing.T) {
	registry := mustRegistry(t, map[string]string{"SAMPLE_OWNER_URL": "http://owner", "SAMPLE_SERVICE_TOKEN": "do-not-print-me"})
	described, err := json.Marshal(registry.Describe())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(described), "do-not-print-me") {
		t.Fatal("the description carries the secret's value")
	}
	token := settingOf(t, registry.Describe(), "service_token")
	if !token.IsSet || token.Value != "" {
		t.Errorf("service_token: is_set=%v value=%q, want set and empty", token.IsSet, token.Value)
	}
	if got := registry.SecretEnvironmentVariables(); len(got) != 1 || got[0] != "SAMPLE_SERVICE_TOKEN" {
		t.Errorf("secret variables = %v", got)
	}
}

func TestTheEnvironmentSeedsAnEditableSettingOnceAndThenTheRecordWins(t *testing.T) {
	stored := &memoryStoredValues{}
	first := mustRegistry(t, map[string]string{"SAMPLE_OWNER_URL": "http://owner", "SAMPLE_CLASSIFIER_MODEL": "model-from-environment"})
	if err := first.AttachStoredValues(stored, nil); err != nil {
		t.Fatal(err)
	}
	if stored.held["classifier.model"] != "model-from-environment" || stored.held["classifier.timeout"] != "20s" {
		t.Fatalf("the first start did not seed the record from the environment and the default: %v", stored.held)
	}
	if err := first.Set("classifier.model", "model-the-operator-chose"); err != nil {
		t.Fatal(err)
	}

	// The next start: the environment still names the old model, and loses.
	second := mustRegistry(t, map[string]string{"SAMPLE_OWNER_URL": "http://owner", "SAMPLE_CLASSIFIER_MODEL": "model-from-environment"})
	if err := second.AttachStoredValues(stored, nil); err != nil {
		t.Fatal(err)
	}
	if got := second.String("classifier.model"); got != "model-the-operator-chose" {
		t.Errorf("classifier.model = %q after a restart, want the stored value", got)
	}
	model := settingOf(t, second.Describe(), "classifier.model")
	if model.Source != msg.ServiceSettingSourceStored || !model.Editable {
		t.Errorf("classifier.model: source=%s editable=%v", model.Source, model.Editable)
	}
	if len(model.Notes) != 1 || !strings.Contains(model.Notes[0], "SAMPLE_CLASSIFIER_MODEL is also set") {
		t.Errorf("no note says the environment variable is ignored: %v", model.Notes)
	}
	if notes := settingOf(t, second.Describe(), "classifier.timeout").Notes; len(notes) != 0 {
		t.Errorf("classifier.timeout carries a note with no variable set: %v", notes)
	}
}

func TestASeedNamedByTheCallerWinsOverTheEnvironmentOnTheFirstStart(t *testing.T) {
	stored := &memoryStoredValues{}
	registry := mustRegistry(t, map[string]string{"SAMPLE_OWNER_URL": "http://owner"})
	if err := registry.AttachStoredValues(stored, map[string]string{"classifier.model": "model-from-a-config-literal"}); err != nil {
		t.Fatal(err)
	}
	if got := registry.String("classifier.model"); got != "model-from-a-config-literal" {
		t.Errorf("classifier.model = %q", got)
	}
}

func TestSetRefusesWhatItShouldAndStoresNothing(t *testing.T) {
	stored := &memoryStoredValues{}
	registry := mustRegistry(t, map[string]string{"SAMPLE_OWNER_URL": "http://owner"})
	if err := registry.AttachStoredValues(stored, nil); err != nil {
		t.Fatal(err)
	}
	registry.SetValidator("classifier.model", func(value string) error {
		switch value {
		case "model-nobody-serves":
			return fmt.Errorf("no such model")
		case "model-while-the-registry-is-down":
			return fmt.Errorf("%w: model registry", ErrSettingOwnerUnavailable)
		}
		return nil
	})
	var changed []string
	registry.OnChange(func(key string) { changed = append(changed, key) })

	refusals := []struct {
		key, value string
		want       error
	}{
		{"no.such.setting", "x", ErrUnknownSetting},
		{"listen_address", ":1", ErrSettingNotEditable},
		{"classifier.max_characters", "10", ErrSettingNotEditable},
		{"classifier.timeout", "soon", ErrInvalidSettingValue},
		{"classifier.model", "model-nobody-serves", ErrInvalidSettingValue},
		{"classifier.model", "model-while-the-registry-is-down", ErrSettingOwnerUnavailable},
	}
	for _, refusal := range refusals {
		if err := registry.Set(refusal.key, refusal.value); !errors.Is(err, refusal.want) {
			t.Errorf("Set(%s, %s) = %v, want %v", refusal.key, refusal.value, err, refusal.want)
		}
	}
	if stored.held["classifier.model"] != "small-model" || stored.held["classifier.timeout"] != "20s" || len(changed) != 0 {
		t.Fatalf("a refused write reached the record or the listeners: %v %v", stored.held, changed)
	}

	if err := registry.Set("classifier.timeout", " 90s "); err != nil {
		t.Fatal(err)
	}
	if registry.Duration("classifier.timeout") != 90*time.Second || stored.held["classifier.timeout"] != "90s" || len(changed) != 1 {
		t.Errorf("an accepted write is not in force, stored and announced: %v %v %v", registry.Duration("classifier.timeout"), stored.held, changed)
	}
	// Emptying a setting is a value too: it is how a model-less classifier is switched off.
	if err := registry.Set("classifier.model", ""); err != nil {
		t.Fatal(err)
	}
	if registry.String("classifier.model") != "" || settingOf(t, registry.Describe(), "classifier.model").IsSet {
		t.Error("an emptied setting still reads as set")
	}
}

func TestReadingAnUndeclaredKeyOrTheWrongTypePanics(t *testing.T) {
	registry := mustRegistry(t, map[string]string{"SAMPLE_OWNER_URL": "http://owner"})
	for name, read := range map[string]func(){
		"undeclared": func() { registry.String("never.declared") },
		"wrong type": func() { registry.Integer("classifier.timeout") },
	} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("%s: no panic", name)
				}
			}()
			read()
		}()
	}
}

func TestTheHandlerServesAndStoresWithTheRefusalsStatus(t *testing.T) {
	stored := &memoryStoredValues{}
	registry := mustRegistry(t, map[string]string{"SAMPLE_OWNER_URL": "http://owner"})
	if err := registry.AttachStoredValues(stored, nil); err != nil {
		t.Fatal(err)
	}
	registry.SetValidator("classifier.model", func(value string) error {
		if value == "down" {
			return fmt.Errorf("%w: model registry", ErrSettingOwnerUnavailable)
		}
		return nil
	})
	handler := Handler(registry, "/settings")
	call := func(method, path, body string) *httptest.ResponseRecorder {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(method, path, strings.NewReader(body)))
		return recorder
	}

	listed := call(http.MethodGet, "/settings", "")
	var described msg.ServiceSettings
	if err := json.Unmarshal(listed.Body.Bytes(), &described); err != nil || listed.Code != 200 || described.Service != "sample" || len(described.Kinds) == 0 {
		t.Fatalf("GET /settings: %d %v %s", listed.Code, err, listed.Body.String())
	}
	for path, want := range map[string]int{
		"/settings/no.such.setting":    http.StatusNotFound,
		"/settings/listen_address":     http.StatusConflict,
		"/settings/classifier.timeout": http.StatusBadRequest,
	} {
		if got := call(http.MethodPut, path, `{"value":"soon"}`); got.Code != want {
			t.Errorf("PUT %s = %d, want %d: %s", path, got.Code, want, got.Body.String())
		}
	}
	if got := call(http.MethodPut, "/settings/classifier.model", `{"value":"down"}`); got.Code != http.StatusBadGateway {
		t.Errorf("an owner that cannot be asked = %d, want 502", got.Code)
	}
	if got := call(http.MethodPut, "/settings/classifier.model", `{"valeu":"x"}`); got.Code != http.StatusBadRequest {
		t.Errorf("a misspelled body field = %d, want 400", got.Code)
	}
	accepted := call(http.MethodPut, "/settings/classifier.model", `{"value":"larger-model"}`)
	if accepted.Code != 200 || stored.held["classifier.model"] != "larger-model" {
		t.Errorf("an accepted PUT: %d, stored %v", accepted.Code, stored.held)
	}
	if got := call(http.MethodDelete, "/settings/classifier.model", ""); got.Code != http.StatusMethodNotAllowed {
		t.Errorf("DELETE = %d", got.Code)
	}
}
