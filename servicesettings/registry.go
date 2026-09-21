// Package servicesettings is how a service declares, reads and describes its
// own configuration, so that the same list serves three readers: the process
// (typed getters), the operator (GET /settings, msg.ServiceSettings) and the
// startup check that refuses a misspelled or malformed variable.
//
// A setting is declared once, as a Definition. Deploy configuration — wiring,
// paths, secrets — is read from the environment when the Registry is built and
// never changes. A behaviour setting declared Editable is stored by the service
// (AttachStoredValues): the environment seeds the record once, and from then on
// the record is the only source, changed with Set and read at the time of use.
package servicesettings

import (
	"errors"
	"fmt"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/kayushkin/llm-bridge/msg"
)

// Definition declares one setting.
type Definition struct {
	Key                 string
	EnvironmentVariable string
	Kind                msg.ServiceSettingKind
	ValueType           msg.ServiceSettingValueType
	// Description says what the setting is and what changing it changes.
	Description string
	// Default is the value built into the service, written as ValueType says.
	// Empty means there is none. A secret has no default.
	Default string
	// Required makes an unset setting with no default a startup error.
	Required bool
	// Editable marks a behaviour setting the service stores and changes at
	// runtime. Only a behaviour setting may be editable.
	Editable bool
}

// StoredValues is where a service keeps its editable settings: its own
// database. Load returns every stored key; Save writes one.
type StoredValues interface {
	Load() (map[string]string, error)
	Save(key, value string) error
}

var (
	// ErrUnknownSetting: the key names no declared setting.
	ErrUnknownSetting = errors.New("unknown setting")
	// ErrSettingNotEditable: the setting is declared, and is not one the
	// service stores. It changes where the process is started.
	ErrSettingNotEditable = errors.New("setting is not editable")
	// ErrInvalidSettingValue: the value does not parse as the setting's type,
	// or the setting's validator refused it.
	ErrInvalidSettingValue = errors.New("invalid setting value")
	// ErrSettingOwnerUnavailable is what a validator returns when it could not
	// ask whoever owns the thing the value names — an instance registry that
	// is down. Nothing is stored.
	ErrSettingOwnerUnavailable = errors.New("the owner of the named value could not be asked")
)

// Registry holds a service's declared settings and the values in force.
type Registry struct {
	service     string
	definitions []Definition
	byKey       map[string]Definition

	mutex sync.RWMutex
	// values holds the text in force per key; present only when some value is
	// in force.
	values  map[string]string
	sources map[string]msg.ServiceSettingSource
	// environmentAlsoSet marks an editable key whose environment variable is
	// set while the stored value is what is in force.
	environmentAlsoSet map[string]bool

	stored     StoredValues
	validators map[string]func(value string) error
	onChange   []func(key string)
}

// Environment is the two reads a Registry makes of a process environment.
type Environment struct {
	// Lookup reads one variable.
	Lookup func(name string) (string, bool)
	// Names lists every variable name that is set.
	Names func() []string
}

// ProcessEnvironment is the running process's own environment.
func ProcessEnvironment() Environment {
	return Environment{
		Lookup: os.LookupEnv,
		Names: func() []string {
			entries := os.Environ()
			names := make([]string, 0, len(entries))
			for _, entry := range entries {
				name, _, _ := strings.Cut(entry, "=")
				names = append(names, name)
			}
			return names
		},
	}
}

// MapEnvironment is an environment held in a map, for tests and for a service
// built from values rather than from a process.
func MapEnvironment(variables map[string]string) Environment {
	return Environment{
		Lookup: func(name string) (string, bool) { value, ok := variables[name]; return value, ok },
		Names: func() []string {
			names := make([]string, 0, len(variables))
			for name := range variables {
				names = append(names, name)
			}
			return names
		},
	}
}

// New builds a Registry from the declared settings and an environment. It
// returns one error naming every fault it found, so a bad unit file is fixed in
// one pass:
//
//   - a declaration fault (duplicate key or variable, an editable setting that
//     is not a behaviour setting, a secret with a default, a default that does
//     not parse);
//   - a set variable whose value does not parse as its type;
//   - a set variable that starts with one of ownedPrefixes and is declared by
//     nobody. That is a misspelling or a leftover, and either way the operator
//     believes it does something.
func New(service string, ownedPrefixes []string, definitions []Definition, environment Environment) (*Registry, error) {
	registry := &Registry{
		service:            service,
		definitions:        append([]Definition(nil), definitions...),
		byKey:              make(map[string]Definition, len(definitions)),
		values:             make(map[string]string),
		sources:            make(map[string]msg.ServiceSettingSource),
		environmentAlsoSet: make(map[string]bool),
		validators:         make(map[string]func(string) error),
	}
	var faults []string
	declaredVariables := make(map[string]string)
	for _, definition := range definitions {
		if definition.Key == "" || definition.EnvironmentVariable == "" {
			faults = append(faults, fmt.Sprintf("a setting is declared with an empty key or variable: %+v", definition))
			continue
		}
		if _, duplicate := registry.byKey[definition.Key]; duplicate {
			faults = append(faults, fmt.Sprintf("setting %s is declared twice", definition.Key))
			continue
		}
		if other, duplicate := declaredVariables[definition.EnvironmentVariable]; duplicate {
			faults = append(faults, fmt.Sprintf("%s is declared for both %s and %s", definition.EnvironmentVariable, other, definition.Key))
			continue
		}
		registry.byKey[definition.Key] = definition
		declaredVariables[definition.EnvironmentVariable] = definition.Key
		if definition.Editable && definition.Kind != msg.ServiceSettingKindBehaviour {
			faults = append(faults, fmt.Sprintf("setting %s is declared editable and is a %s setting; only a behaviour setting is stored by the service", definition.Key, definition.Kind))
		}
		if definition.Kind == msg.ServiceSettingKindSecret && definition.Default != "" {
			faults = append(faults, fmt.Sprintf("setting %s is a secret and declares a default", definition.Key))
		}
		if definition.Default != "" {
			if err := checkValue(definition.ValueType, definition.Default); err != nil {
				faults = append(faults, fmt.Sprintf("setting %s: its default %q: %v", definition.Key, definition.Default, err))
			}
		}

		value, isSet := environment.Lookup(definition.EnvironmentVariable)
		switch {
		case isSet && value != "":
			if err := checkValue(definition.ValueType, value); err != nil {
				faults = append(faults, fmt.Sprintf("%s: %v", definition.EnvironmentVariable, err))
				continue
			}
			registry.values[definition.Key] = value
			registry.sources[definition.Key] = msg.ServiceSettingSourceEnvironment
		case definition.Default != "":
			registry.values[definition.Key] = definition.Default
			registry.sources[definition.Key] = msg.ServiceSettingSourceDefault
		default:
			registry.sources[definition.Key] = msg.ServiceSettingSourceUnset
		}
	}

	names := environment.Names()
	sort.Strings(names)
	for _, name := range names {
		if _, declared := declaredVariables[name]; declared {
			continue
		}
		for _, prefix := range ownedPrefixes {
			if prefix != "" && strings.HasPrefix(name, prefix) {
				faults = append(faults, fmt.Sprintf("%s is set and %s declares no such setting: a misspelling, or a leftover that does nothing", name, service))
				break
			}
		}
	}
	if len(faults) > 0 {
		return nil, fmt.Errorf("%s settings: %s", service, strings.Join(faults, "; "))
	}
	return registry, nil
}

// CheckRequired names every Required setting with no value in force. It is a
// step of its own, not part of New, because one binary is often both a server
// and a handful of operator commands: the server cannot run without its
// required settings, and a command run from a shell has no use for them. The
// server's main calls this and refuses to start on an error.
func (r *Registry) CheckRequired() error {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	var missing []string
	for _, definition := range r.definitions {
		if _, inForce := r.values[definition.Key]; definition.Required && !inForce {
			missing = append(missing, fmt.Sprintf("%s is unset: %s", definition.EnvironmentVariable, definition.Description))
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("%s settings: %s", r.service, strings.Join(missing, "; "))
	}
	return nil
}

// AttachStoredValues makes the service's own record the source of every
// editable setting. A key the record already holds is served from it. A key it
// does not hold is seeded once — from seeds when the caller names a value for
// it, otherwise from what the environment or the default gave — and saved, so
// the first start after a setting becomes editable changes nothing. After this
// call the environment no longer decides an editable setting.
func (r *Registry) AttachStoredValues(stored StoredValues, seeds map[string]string) error {
	held, err := stored.Load()
	if err != nil {
		return fmt.Errorf("%s settings: read stored values: %w", r.service, err)
	}
	r.mutex.Lock()
	defer r.mutex.Unlock()
	for _, definition := range r.definitions {
		if !definition.Editable {
			continue
		}
		if r.sources[definition.Key] == msg.ServiceSettingSourceEnvironment {
			r.environmentAlsoSet[definition.Key] = true
		}
		value, isHeld := held[definition.Key]
		if !isHeld {
			value = r.values[definition.Key]
			if seed, named := seeds[definition.Key]; named {
				value = seed
			}
			if err := stored.Save(definition.Key, value); err != nil {
				return fmt.Errorf("%s settings: seed %s: %w", r.service, definition.Key, err)
			}
		}
		if value != "" {
			if err := checkValue(definition.ValueType, value); err != nil {
				return fmt.Errorf("%s settings: stored %s = %q: %w", r.service, definition.Key, value, err)
			}
		}
		r.setLocked(definition.Key, value, msg.ServiceSettingSourceStored)
	}
	r.stored = stored
	return nil
}

func (r *Registry) setLocked(key, value string, source msg.ServiceSettingSource) {
	if value == "" {
		delete(r.values, key)
	} else {
		r.values[key] = value
	}
	r.sources[key] = source
}

// SetValidator adds a check Set runs on a new value for key after it has
// parsed: whether a named instance exists, say. Return an error wrapping
// ErrSettingOwnerUnavailable when the owner could not be asked.
func (r *Registry) SetValidator(key string, validator func(value string) error) {
	r.mustDefinition(key)
	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.validators[key] = validator
}

// OnChange registers a function called after Set stores a new value.
func (r *Registry) OnChange(callback func(key string)) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.onChange = append(r.onChange, callback)
}

// Set stores a new value for an editable setting and puts it in force. An
// empty value is allowed and means "off" or "none" for the settings that read
// it so; a required setting refuses it.
func (r *Registry) Set(key, value string) error {
	definition, declared := r.byKey[key]
	if !declared {
		return fmt.Errorf("%w: %s", ErrUnknownSetting, key)
	}
	if !definition.Editable {
		return fmt.Errorf("%w: %s is read from %s where the process is started", ErrSettingNotEditable, key, definition.EnvironmentVariable)
	}
	value = strings.TrimSpace(value)
	if value == "" && definition.Required {
		return fmt.Errorf("%w: %s is required and cannot be emptied", ErrInvalidSettingValue, key)
	}
	if value != "" {
		if err := checkValue(definition.ValueType, value); err != nil {
			return fmt.Errorf("%w: %s: %v", ErrInvalidSettingValue, key, err)
		}
	}
	r.mutex.RLock()
	validator, stored := r.validators[key], r.stored
	r.mutex.RUnlock()
	if stored == nil {
		return fmt.Errorf("%s settings: %s cannot be set: no stored values are attached", r.service, key)
	}
	if validator != nil && value != "" {
		if err := validator(value); err != nil {
			if errors.Is(err, ErrSettingOwnerUnavailable) {
				return err
			}
			return fmt.Errorf("%w: %s: %v", ErrInvalidSettingValue, key, err)
		}
	}
	if err := stored.Save(key, value); err != nil {
		return fmt.Errorf("%s settings: store %s: %w", r.service, key, err)
	}
	r.mutex.Lock()
	r.setLocked(key, value, msg.ServiceSettingSourceStored)
	callbacks := append([]func(string){}, r.onChange...)
	r.mutex.Unlock()
	for _, callback := range callbacks {
		callback(key)
	}
	return nil
}

// Describe is the GET /settings answer. A secret's value is never in it.
func (r *Registry) Describe() msg.ServiceSettings {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	settings := make([]msg.ServiceSetting, 0, len(r.definitions))
	for _, definition := range r.definitions {
		value, isSet := r.values[definition.Key]
		setting := msg.ServiceSetting{
			Key:                 definition.Key,
			EnvironmentVariable: definition.EnvironmentVariable,
			Kind:                definition.Kind,
			ValueType:           definition.ValueType,
			Description:         definition.Description,
			IsSet:               isSet,
			Source:              r.sources[definition.Key],
			Required:            definition.Required,
			Editable:            definition.Editable && r.stored != nil,
			Notes:               []string{},
		}
		if definition.Kind != msg.ServiceSettingKindSecret {
			setting.Value = value
			setting.DefaultValue = definition.Default
		}
		if r.environmentAlsoSet[definition.Key] && setting.Source == msg.ServiceSettingSourceStored {
			setting.Notes = append(setting.Notes, fmt.Sprintf("%s is also set where the process is started. It seeded this record once and is ignored now; the stored value wins.", definition.EnvironmentVariable))
		}
		settings = append(settings, setting)
	}
	return msg.ServiceSettings{
		Service:  r.service,
		Settings: settings,
		Kinds:    msg.ServiceSettingKinds(),
		Sources:  msg.ServiceSettingSources(),
	}
}

// SecretEnvironmentVariables lists the variable of every declared secret: what
// a service that spawns child processes must keep out of their environment.
func (r *Registry) SecretEnvironmentVariables() []string {
	var names []string
	for _, definition := range r.definitions {
		if definition.Kind == msg.ServiceSettingKindSecret {
			names = append(names, definition.EnvironmentVariable)
		}
	}
	return names
}

func (r *Registry) mustDefinition(key string) Definition {
	definition, declared := r.byKey[key]
	if !declared {
		panic(fmt.Sprintf("servicesettings: %s reads %q, which it never declared", r.service, key))
	}
	return definition
}

func (r *Registry) text(key string, valueType msg.ServiceSettingValueType) string {
	definition := r.mustDefinition(key)
	if definition.ValueType != valueType {
		panic(fmt.Sprintf("servicesettings: %s is declared %s and read as %s", key, definition.ValueType, valueType))
	}
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	return r.values[key]
}

// String returns the value in force, empty when none is. Reading a key that
// was never declared, or as the wrong type, is a programming fault and panics.
func (r *Registry) String(key string) string {
	return r.text(key, msg.ServiceSettingValueTypeString)
}

// Integer returns the value in force, zero when none is.
func (r *Registry) Integer(key string) int {
	text := r.text(key, msg.ServiceSettingValueTypeInteger)
	if text == "" {
		return 0
	}
	number, _ := strconv.Atoi(text) // parsed when it was accepted
	return number
}

// Decimal returns the value in force, zero when none is.
func (r *Registry) Decimal(key string) float64 {
	text := r.text(key, msg.ServiceSettingValueTypeDecimal)
	if text == "" {
		return 0
	}
	number, _ := strconv.ParseFloat(text, 64) // parsed when it was accepted
	return number
}

// Boolean returns the value in force, false when none is.
func (r *Registry) Boolean(key string) bool {
	flag, _ := strconv.ParseBool(r.text(key, msg.ServiceSettingValueTypeBoolean))
	return flag
}

// Duration returns the value in force, zero when none is.
func (r *Registry) Duration(key string) time.Duration {
	text := r.text(key, msg.ServiceSettingValueTypeDuration)
	if text == "" {
		return 0
	}
	duration, _ := time.ParseDuration(text)
	return duration
}

// StringList returns the items in force, trimmed, with empty ones dropped.
func (r *Registry) StringList(key string) []string {
	return splitList(r.text(key, msg.ServiceSettingValueTypeStringList))
}

// StringMap returns the key:value pairs in force.
func (r *Registry) StringMap(key string) map[string]string {
	pairs, _ := splitMap(r.text(key, msg.ServiceSettingValueTypeStringMap))
	return pairs
}

func splitList(text string) []string {
	var items []string
	for _, item := range strings.Split(text, ",") {
		if item = strings.TrimSpace(item); item != "" {
			items = append(items, item)
		}
	}
	return items
}

func splitMap(text string) (map[string]string, error) {
	pairs := make(map[string]string)
	for _, pair := range splitList(text) {
		key, value, found := strings.Cut(pair, ":")
		key, value = strings.TrimSpace(key), strings.TrimSpace(value)
		if !found || key == "" || value == "" {
			return nil, fmt.Errorf("%q is not a key:value pair", pair)
		}
		pairs[key] = value
	}
	return pairs, nil
}

// checkValue reports whether text is written as valueType says.
func checkValue(valueType msg.ServiceSettingValueType, text string) error {
	switch valueType {
	case msg.ServiceSettingValueTypeString, msg.ServiceSettingValueTypeStringList:
		return nil
	case msg.ServiceSettingValueTypeInteger:
		if _, err := strconv.Atoi(text); err != nil {
			return fmt.Errorf("%q is not a whole number", text)
		}
	case msg.ServiceSettingValueTypeDecimal:
		// ParseFloat takes "NaN" and "Inf", which no setting means.
		if number, err := strconv.ParseFloat(text, 64); err != nil || math.IsNaN(number) || math.IsInf(number, 0) {
			return fmt.Errorf("%q is not a decimal number such as 25 or 12.50", text)
		}
	case msg.ServiceSettingValueTypeBoolean:
		if _, err := strconv.ParseBool(text); err != nil {
			return fmt.Errorf("%q is not true or false", text)
		}
	case msg.ServiceSettingValueTypeDuration:
		if _, err := time.ParseDuration(text); err != nil {
			return fmt.Errorf("%q is not a duration such as 15m or 20s", text)
		}
	case msg.ServiceSettingValueTypeStringMap:
		if _, err := splitMap(text); err != nil {
			return err
		}
	default:
		return fmt.Errorf("value type %q is not one this package knows", valueType)
	}
	return nil
}
