package msg

// ServiceSettings is a service describing its own configuration: every setting
// it reads, what each one is for, the value in force and where that value came
// from. A service answers it at GET /settings, and one page draws any service
// from the answer.
//
// Two kinds of setting are kept apart on purpose. Deploy configuration —
// addresses, paths, secrets — belongs to the environment a process is started
// in: it is shown and never edited through the service. Behaviour settings are
// stored by the service that owns them, read when they are used, and changed
// with PUT /settings/{key}, with no restart.
type ServiceSettings struct {
	// Service is the service's own name, as healthcheck and the repo know it.
	Service  string           `json:"service"`
	Settings []ServiceSetting `json:"settings"`
	// Kinds and Sources are the vocabularies ServiceSetting.Kind and
	// ServiceSetting.Source draw from, with what each means, so a page draws
	// its legend from the answer rather than from a copy.
	Kinds   []ServiceSettingKindDefinition   `json:"kinds"`
	Sources []ServiceSettingSourceDefinition `json:"sources"`
}

// ServiceSettingKind says what sort of thing a setting is, which decides how
// it may be shown and whether it may be edited.
type ServiceSettingKind string

const (
	// ServiceSettingKindWiring is an address of something else: a URL, a
	// listen address, a port.
	ServiceSettingKindWiring ServiceSettingKind = "wiring"
	// ServiceSettingKindPath is a file or directory on the host.
	ServiceSettingKindPath ServiceSettingKind = "path"
	// ServiceSettingKindSecret is a credential. Its value is never served:
	// only whether it is set.
	ServiceSettingKindSecret ServiceSettingKind = "secret"
	// ServiceSettingKindBehaviour changes what the service does: a model, an
	// instance, a timeout, a limit.
	ServiceSettingKindBehaviour ServiceSettingKind = "behaviour"
)

// ServiceSettingSource says where the value in force came from.
type ServiceSettingSource string

const (
	// ServiceSettingSourceEnvironment: the process environment set it.
	ServiceSettingSourceEnvironment ServiceSettingSource = "environment"
	// ServiceSettingSourceStored: the service's own record holds it. The
	// environment variable, if set, is ignored.
	ServiceSettingSourceStored ServiceSettingSource = "stored"
	// ServiceSettingSourceDefault: nothing set it, and the value built into
	// the service is in force.
	ServiceSettingSourceDefault ServiceSettingSource = "default"
	// ServiceSettingSourceUnset: nothing set it and there is no default.
	ServiceSettingSourceUnset ServiceSettingSource = "unset"
)

// ServiceSettingValueType says how Value is written.
type ServiceSettingValueType string

const (
	ServiceSettingValueTypeString  ServiceSettingValueType = "string"
	ServiceSettingValueTypeInteger ServiceSettingValueType = "integer"
	ServiceSettingValueTypeBoolean ServiceSettingValueType = "boolean"
	// A Go duration: "15m", "20s", "1h30m".
	ServiceSettingValueTypeDuration ServiceSettingValueType = "duration"
	// Comma-separated items: "codex,aider".
	ServiceSettingValueTypeStringList ServiceSettingValueType = "string_list"
	// Comma-separated key:value pairs: "herald:Reminders,dispatcher:Work".
	ServiceSettingValueTypeStringMap ServiceSettingValueType = "string_map"
)

// ServiceSetting is one setting and the value in force.
type ServiceSetting struct {
	// Key names the setting within the service, and is what PUT
	// /settings/{key} takes: "signal_classifier.model".
	Key string `json:"key"`
	// EnvironmentVariable is the variable the process reads for it.
	EnvironmentVariable string                  `json:"environment_variable"`
	Kind                ServiceSettingKind      `json:"kind"`
	ValueType           ServiceSettingValueType `json:"value_type"`
	// Description says what the setting is and what changing it changes.
	Description string `json:"description"`
	// Value is the value in force, written as its ValueType says. Always
	// empty for a secret.
	Value string `json:"value"`
	// IsSet reports whether any value is in force. It is how a secret says
	// it is present without showing itself.
	IsSet  bool                 `json:"is_set"`
	Source ServiceSettingSource `json:"source"`
	// DefaultValue is the value built into the service, empty when there is
	// none. Never filled for a secret.
	DefaultValue string `json:"default_value"`
	// Required settings stop the service from starting when unset.
	Required bool `json:"required"`
	// Editable settings take PUT /settings/{key} and apply without a
	// restart. Everything else changes only where the process is started.
	Editable bool `json:"editable"`
	// Notes are things a reader should know about this value: an environment
	// variable that is set and ignored because the stored value wins.
	Notes []string `json:"notes"`
}

type ServiceSettingKindDefinition struct {
	Kind        ServiceSettingKind `json:"kind"`
	Description string             `json:"description"`
}

type ServiceSettingSourceDefinition struct {
	Source      ServiceSettingSource `json:"source"`
	Description string               `json:"description"`
}

// ServiceSettingKinds is the served kind vocabulary.
func ServiceSettingKinds() []ServiceSettingKindDefinition {
	return []ServiceSettingKindDefinition{
		{ServiceSettingKindBehaviour, "Changes what the service does. Stored by the service when editable, and applied without a restart."},
		{ServiceSettingKindWiring, "The address of something else. Set where the process is started; shown here, never edited here."},
		{ServiceSettingKindPath, "A file or directory on the host. Set where the process is started."},
		{ServiceSettingKindSecret, "A credential. Only whether it is set is shown; the value never leaves the process."},
	}
}

// ServiceSettingSources is the served source vocabulary.
func ServiceSettingSources() []ServiceSettingSourceDefinition {
	return []ServiceSettingSourceDefinition{
		{ServiceSettingSourceStored, "The service's own record holds the value. An environment variable for the same setting is ignored."},
		{ServiceSettingSourceEnvironment, "The process environment set the value."},
		{ServiceSettingSourceDefault, "Nothing set it; the value built into the service is in force."},
		{ServiceSettingSourceUnset, "Nothing set it and there is no default."},
	}
}

// ServiceSettingUpdate is the body of PUT /settings/{key}.
type ServiceSettingUpdate struct {
	Value string `json:"value"`
}
