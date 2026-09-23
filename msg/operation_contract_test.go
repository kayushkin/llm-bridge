package msg

import (
	"bytes"
	"encoding/json"
	"os"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// The receipt's wire shape is pinned by a fixture that sets every field. A
// renamed or dropped json tag fails here before any caller sees it.
func TestOperationReceiptWireShapeMatchesTheFixture(t *testing.T) {
	fixture, err := os.ReadFile("testdata/operation_receipt.json")
	if err != nil {
		t.Fatal(err)
	}
	decoder := json.NewDecoder(bytes.NewReader(fixture))
	decoder.DisallowUnknownFields()
	var receipt OperationReceipt
	if err := decoder.Decode(&receipt); err != nil {
		t.Fatalf("fixture does not decode into OperationReceipt: %v", err)
	}
	encoded, err := json.Marshal(receipt)
	if err != nil {
		t.Fatal(err)
	}
	var want, got any
	if err := json.Unmarshal(fixture, &want); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(encoded, &got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(want, got) {
		t.Fatalf("receipt does not round-trip the fixture\nwant %s\n got %s", fixture, encoded)
	}
}

// operationContractTypes are the types ts/msg.ts must carry field for field.
var operationContractTypes = []any{
	OperationReference{},
	OperationIntent{},
	OperationProgress{},
	OperationChildrenSummary{},
	OperationEvidence{},
	OperationEffect{},
	OperationUsage{},
	OperationError{},
	OperationReceipt{},
	OperationEvent{},
	OperationTypeDescription{},
	ClassificationRunInput{},
	ClassificationLabel{},
	ClassificationItem{},
	ClassificationRunResult{},
	ClassificationItemResult{},
}

var typeScriptFieldPattern = regexp.MustCompile(`(?m)^  ([a-z_]+)\??:`)

// Go is the source of truth and ts/msg.ts is generated from it; this catches
// a Go change committed without running generate-ts.sh.
func TestTypeScriptOperationTypesMatchGo(t *testing.T) {
	generated, err := os.ReadFile("../ts/msg.ts")
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range operationContractTypes {
		goType := reflect.TypeOf(value)
		t.Run(goType.Name(), func(t *testing.T) {
			block := typeScriptInterfaceBody(t, string(generated), goType.Name())
			var typeScriptFields []string
			for _, match := range typeScriptFieldPattern.FindAllStringSubmatch(block, -1) {
				typeScriptFields = append(typeScriptFields, match[1])
			}
			goFields := jsonFieldNames(goType)
			sort.Strings(typeScriptFields)
			sort.Strings(goFields)
			if !reflect.DeepEqual(goFields, typeScriptFields) {
				t.Fatalf("ts/msg.ts is stale: run ./generate-ts.sh\nGo:         %v\nTypeScript: %v", goFields, typeScriptFields)
			}
		})
	}
	for _, state := range AllOperationStates {
		if !strings.Contains(string(generated), `"`+string(state)+`"`) {
			t.Errorf("ts/msg.ts has no constant for operation state %q: run ./generate-ts.sh", state)
		}
	}
}

func typeScriptInterfaceBody(t *testing.T, generated, name string) string {
	t.Helper()
	start := strings.Index(generated, "export interface "+name+" {\n")
	if start < 0 {
		t.Fatalf("ts/msg.ts has no interface %s: run ./generate-ts.sh", name)
	}
	body := generated[start:]
	end := strings.Index(body, "\n}\n")
	if end < 0 {
		t.Fatalf("interface %s in ts/msg.ts has no closing brace", name)
	}
	return body[:end]
}

func jsonFieldNames(goType reflect.Type) []string {
	var names []string
	for index := 0; index < goType.NumField(); index++ {
		tag := goType.Field(index).Tag.Get("json")
		name := strings.Split(tag, ",")[0]
		if name != "" && name != "-" {
			names = append(names, name)
		}
	}
	return names
}

func TestEveryTerminalOperationStateHasAnEvent(t *testing.T) {
	for _, state := range AllOperationStates {
		_, hasEvent := OperationEventKindForTerminalState[state]
		if state.IsTerminal() != hasEvent {
			t.Errorf("state %q: terminal=%v but has terminal event=%v", state, state.IsTerminal(), hasEvent)
		}
		if !state.IsKnown() {
			t.Errorf("state %q is listed but not known", state)
		}
	}
	for _, state := range []OperationState{OperationStateQueued, OperationStateRunning} {
		if state.IsTerminal() {
			t.Errorf("%q must not be terminal", state)
		}
	}
	if OperationState("partial").IsKnown() {
		t.Error("partial is shown by a surface from child counts; it is not a state")
	}
}
