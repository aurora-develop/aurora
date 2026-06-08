package chatgpt

import (
	"encoding/json"
	"path"
	"regexp"
	"strconv"
	"strings"
)

const (
	ArtifactDeliveryURL           = "url"
	ArtifactDeliveryBase64        = "base64"
	ArtifactDeliveryBase64Chunked = "base64_chunked"

	StreamEventArtifactPending    = "artifact_pending"
	StreamEventArtifact           = "artifact"
	StreamEventArtifactChunk      = "artifact_chunk"
	StreamEventArtifactDone       = "artifact_done"
	StreamEventArtifactSuperseded = "artifact_superseded"
	StreamEventArtifactSlotFinal  = "artifact_slot_final"
)

type ArtifactSignalType string

const (
	SignalImageGenTaskID   ArtifactSignalType = "image_gen_task_id"
	SignalGhostrider       ArtifactSignalType = "ghostrider"
	SignalDalleTool        ArtifactSignalType = "dalle_tool"
	SignalImageAsset       ArtifactSignalType = "image_asset_pointer"
	SignalPythonTool       ArtifactSignalType = "python_tool"
	SignalCodeInterpreter  ArtifactSignalType = "code_interpreter_recipient"
	SignalExecutionOutput  ArtifactSignalType = "execution_output"
	SignalSandboxPath      ArtifactSignalType = "sandbox_path"
	SignalContentReference ArtifactSignalType = "content_reference"
	SignalToolInvokedMeta  ArtifactSignalType = "tool_invoked_metadata"
	SignalTurnUseCase      ArtifactSignalType = "turn_use_case"
	SignalFileSearch       ArtifactSignalType = "file_search"
)

type ArtifactSignal struct {
	Type   ArtifactSignalType `json:"type"`
	Value  string             `json:"value,omitempty"`
	Source string             `json:"source,omitempty"`
}

type SandboxArtifact struct {
	MessageID   string `json:"message_id"`
	SandboxPath string `json:"sandbox_path"`
	FileName    string `json:"file_name"`
}

type PDFArtifact = SandboxArtifact

type StreamEvent struct {
	Event string `json:"event"`
	Kind  string `json:"kind,omitempty"`
	Title string `json:"title,omitempty"`

	Index int `json:"index,omitempty"`
	Total int `json:"total,omitempty"`

	SlotIndex        int    `json:"slot_index,omitempty"`
	Revision         int    `json:"revision,omitempty"`
	GenID            string `json:"gen_id,omitempty"`
	ParentGenID      string `json:"parent_gen_id,omitempty"`
	UpdateType       string `json:"update_type,omitempty"`
	IsFinal          bool   `json:"is_final,omitempty"`
	SupersedesFileID string `json:"supersedes_file_id,omitempty"`

	Name        string `json:"name,omitempty"`
	MimeType    string `json:"mime_type,omitempty"`
	SizeBytes   int    `json:"size_bytes,omitempty"`
	URL         string `json:"url,omitempty"`
	FileID      string `json:"file_id,omitempty"`
	MessageID   string `json:"message_id,omitempty"`
	SandboxPath string `json:"sandbox_path,omitempty"`

	Data       string `json:"data,omitempty"`
	ChunkIndex int    `json:"chunk_index,omitempty"`
	ChunkTotal int    `json:"chunk_total,omitempty"`

	Error string `json:"error,omitempty"`
}

type ArtifactStreamConfig struct {
	Delivery  string
	ChunkSize int
}

func (cfg ArtifactStreamConfig) normalized() ArtifactStreamConfig {
	if cfg.Delivery == "" {
		cfg.Delivery = ArtifactDeliveryURL
	}
	if cfg.ChunkSize <= 0 {
		cfg.ChunkSize = 384 * 1024
	}
	return cfg
}

type generatedImageSlot struct {
	SlotIndex   int
	Revision    int
	GenID       string
	ParentGenID string
	MessageID   string
	FileID      string
	FileHistory []string
	Final       bool
}

type parsedGeneratedImage struct {
	FileID      string
	Asset       string
	MessageID   string
	GenID       string
	ParentGenID string
	SlotIndex   int
}

type artifactAccumulator struct {
	Signals               []ArtifactSignal
	SandboxArtifacts      []SandboxArtifact
	PDFArtifacts          []PDFArtifact
	ImageFileIDs          []string
	ExpectGeneratedImages bool
	LastAssistantMsgID    string
	ConversationID        string

	seenSignals          map[string]bool
	emittedArtifacts     map[string]bool
	imageSlots           map[string]*generatedImageSlot
	nextImageSlotIndex   int
	imagePendingEmitted  bool
	sandboxPendingEmited bool
}

var sandboxFileRe = regexp.MustCompile(`/mnt/data/[^\s"'<>),;]+`)
var fileIDRe = regexp.MustCompile(`file[-_][A-Za-z0-9]+`)

func newArtifactAccumulator() *artifactAccumulator {
	return &artifactAccumulator{
		seenSignals:        map[string]bool{},
		emittedArtifacts:   map[string]bool{},
		imageSlots:         map[string]*generatedImageSlot{},
		nextImageSlotIndex: 1,
	}
}

func (a *artifactAccumulator) ObserveRaw(raw map[string]interface{}, conversationID string) []StreamEvent {
	if a == nil || raw == nil {
		return nil
	}
	if conversationID != "" {
		a.ConversationID = conversationID
	}
	if cid := firstConversationID(raw); cid != "" {
		a.ConversationID = cid
	}
	if msgID := lastAssistantMessageID(raw); msgID != "" {
		a.LastAssistantMsgID = msgID
	}

	var events []StreamEvent
	for _, signal := range ExtractSignalsFromJSON(raw) {
		if signal.Source == "" {
			signal.Source, _ = raw["type"].(string)
			if signal.Source == "" {
				signal.Source = "sse"
			}
		}
		key := string(signal.Type) + "\x00" + signal.Value
		if a.seenSignals[key] {
			continue
		}
		a.seenSignals[key] = true
		a.Signals = append(a.Signals, signal)

		switch signal.Type {
		case SignalImageGenTaskID, SignalGhostrider, SignalDalleTool:
			a.ExpectGeneratedImages = true
			events = append(events, a.generatedImagePendingEvent()...)
		case SignalSandboxPath:
			events = append(events, a.observeSandboxPath(signal.Value)...)
		}
	}

	updateType, _ := raw["type"].(string)
	for _, parsed := range parseGeneratedImagesFromValue(raw) {
		if parsed.MessageID == "" {
			parsed.MessageID = a.LastAssistantMsgID
		}
		if parsed.GenID != "" {
			a.ExpectGeneratedImages = true
			events = append(events, a.generatedImagePendingEvent()...)
		}
		events = append(events, a.noteGeneratedImageRevision(parsed, updateType)...)
	}
	return events
}

func (a *artifactAccumulator) Finalize() []StreamEvent {
	if a == nil || len(a.imageSlots) == 0 {
		return nil
	}
	var events []StreamEvent
	total := len(a.imageSlots)
	for _, slot := range a.imageSlots {
		if slot == nil || slot.FileID == "" || slot.Final {
			continue
		}
		slot.Final = true
		events = append(events, StreamEvent{
			Event:     StreamEventArtifactSlotFinal,
			Kind:      "generated_image",
			SlotIndex: slot.SlotIndex,
			Revision:  slot.Revision,
			GenID:     slot.GenID,
			MessageID: slot.MessageID,
			FileID:    slot.FileID,
			IsFinal:   true,
			Total:     total,
		})
	}
	return events
}

func (a *artifactAccumulator) generatedImagePendingEvent() []StreamEvent {
	if a.imagePendingEmitted {
		return nil
	}
	a.imagePendingEmitted = true
	return []StreamEvent{{
		Event: StreamEventArtifactPending,
		Kind:  "generated_image",
		Title: "Generating image",
	}}
}

func (a *artifactAccumulator) observeSandboxPath(sandboxPath string) []StreamEvent {
	if sandboxPath == "" || !isValidSandboxPath(sandboxPath) {
		return nil
	}
	key := "sandbox:" + sandboxPath
	if a.emittedArtifacts[key] {
		return nil
	}
	a.emittedArtifacts[key] = true
	art := SandboxArtifact{
		MessageID:   a.LastAssistantMsgID,
		SandboxPath: sandboxPath,
		FileName:    path.Base(sandboxPath),
	}
	a.SandboxArtifacts = append(a.SandboxArtifacts, art)
	a.PDFArtifacts = filterPDFArtifacts(a.SandboxArtifacts)

	var events []StreamEvent
	if !a.sandboxPendingEmited {
		a.sandboxPendingEmited = true
		events = append(events, StreamEvent{
			Event: StreamEventArtifactPending,
			Kind:  "sandbox_file",
			Title: "Sandbox artifact",
		})
	}
	events = append(events, StreamEvent{
		Event:       StreamEventArtifact,
		Kind:        sandboxArtifactKind(art),
		Index:       len(a.SandboxArtifacts),
		Total:       len(a.SandboxArtifacts),
		Name:        art.FileName,
		MessageID:   art.MessageID,
		SandboxPath: art.SandboxPath,
	})
	return events
}

func (a *artifactAccumulator) noteGeneratedImageRevision(p parsedGeneratedImage, updateType string) []StreamEvent {
	if p.FileID == "" || a == nil {
		return nil
	}
	if p.GenID == "" && !a.ExpectGeneratedImages {
		return nil
	}
	if !stringSeen(a.ImageFileIDs, p.FileID) {
		a.ImageFileIDs = append(a.ImageFileIDs, p.FileID)
	}

	slotKey := generatedImageSlotKey(p)
	slot := a.imageSlots[slotKey]
	if slot == nil {
		index := p.SlotIndex
		if index <= 0 {
			index = a.nextImageSlotIndex
			a.nextImageSlotIndex++
		} else if index >= a.nextImageSlotIndex {
			a.nextImageSlotIndex = index + 1
		}
		slot = &generatedImageSlot{SlotIndex: index}
		a.imageSlots[slotKey] = slot
	}
	if slot.FileID == p.FileID {
		return nil
	}

	var events []StreamEvent
	previousFileID := slot.FileID
	if previousFileID != "" {
		events = append(events, StreamEvent{
			Event:      StreamEventArtifactSuperseded,
			Kind:       "generated_image",
			SlotIndex:  slot.SlotIndex,
			Revision:   slot.Revision,
			GenID:      slot.GenID,
			MessageID:  slot.MessageID,
			FileID:     previousFileID,
			UpdateType: updateType,
			IsFinal:    false,
		})
	}

	slot.Revision++
	slot.GenID = firstNonEmpty(p.GenID, slot.GenID)
	slot.ParentGenID = firstNonEmpty(p.ParentGenID, slot.ParentGenID)
	slot.MessageID = firstNonEmpty(p.MessageID, slot.MessageID)
	slot.FileID = p.FileID
	slot.FileHistory = append(slot.FileHistory, p.FileID)
	slot.Final = false

	events = append(events, StreamEvent{
		Event:            StreamEventArtifact,
		Kind:             "generated_image",
		Index:            slot.SlotIndex,
		SlotIndex:        slot.SlotIndex,
		Revision:         slot.Revision,
		GenID:            slot.GenID,
		ParentGenID:      slot.ParentGenID,
		MessageID:        slot.MessageID,
		FileID:           slot.FileID,
		UpdateType:       updateType,
		IsFinal:          false,
		SupersedesFileID: previousFileID,
		MimeType:         "image/png",
		Name:             "generated_slot" + strconv.Itoa(slot.SlotIndex) + "_rev" + strconv.Itoa(slot.Revision) + ".png",
	})
	return events
}

func generatedImageSlotKey(p parsedGeneratedImage) string {
	if p.SlotIndex > 0 {
		return "slot:" + strconv.Itoa(p.SlotIndex)
	}
	if p.GenID != "" {
		return "gen:" + p.GenID
	}
	return "file:" + p.FileID
}

func ExtractSignalsFromJSON(v interface{}) []ArtifactSignal {
	var out []ArtifactSignal
	walkSignals(v, "", &out)
	return dedupeSignals(out)
}

func walkSignals(v interface{}, ctx string, out *[]ArtifactSignal) {
	switch x := v.(type) {
	case map[string]interface{}:
		inspectMessageMap(x, out)
		for k, val := range x {
			walkSignals(val, k, out)
		}
	case []interface{}:
		for _, item := range x {
			walkSignals(item, ctx, out)
		}
	case string:
		for _, m := range sandboxFileRe.FindAllString(x, -1) {
			if isValidSandboxPath(m) {
				*out = append(*out, ArtifactSignal{Type: SignalSandboxPath, Value: m})
			}
		}
		if strings.HasPrefix(x, "sediment://") || strings.Contains(ctx, "asset_pointer") {
			if fid := extractFileID(x); fid != "" {
				*out = append(*out, ArtifactSignal{Type: SignalImageAsset, Value: fid})
			}
		}
	}
}

func inspectMessageMap(m map[string]interface{}, out *[]ArtifactSignal) {
	if t, ok := m["type"].(string); ok && t == "server_ste_metadata" {
		if md, ok := m["metadata"].(map[string]interface{}); ok {
			if inv, ok := md["tool_invoked"].(bool); ok && inv {
				toolName, _ := md["tool_name"].(string)
				*out = append(*out, ArtifactSignal{Type: SignalToolInvokedMeta, Value: toolName})
			}
			if uc, ok := md["turn_use_case"].(string); ok && uc != "" {
				*out = append(*out, ArtifactSignal{Type: SignalTurnUseCase, Value: uc})
			}
		}
	}
	if meta, ok := m["metadata"].(map[string]interface{}); ok {
		if tid, ok := meta["image_gen_task_id"].(string); ok && tid != "" {
			*out = append(*out, ArtifactSignal{Type: SignalImageGenTaskID, Value: tid})
		}
		if _, ok := meta["ghostrider"]; ok {
			*out = append(*out, ArtifactSignal{Type: SignalGhostrider, Value: "1"})
		}
		if refs, ok := meta["content_references"].([]interface{}); ok && len(refs) > 0 {
			*out = append(*out, ArtifactSignal{Type: SignalContentReference, Value: "present"})
		}
		if agg, ok := meta["aggregate_result"].(map[string]interface{}); ok {
			if code, ok := agg["code"].(string); ok {
				for _, p := range sandboxFileRe.FindAllString(code, -1) {
					if isValidSandboxPath(p) {
						*out = append(*out, ArtifactSignal{Type: SignalSandboxPath, Value: p})
					}
				}
			}
		}
	}
	if author, ok := m["author"].(map[string]interface{}); ok {
		role, _ := author["role"].(string)
		name, _ := author["name"].(string)
		if role == "tool" && name != "" {
			lower := strings.ToLower(name)
			if strings.Contains(lower, "dalle") || strings.Contains(lower, "image_gen") {
				*out = append(*out, ArtifactSignal{Type: SignalDalleTool, Value: name})
			}
			if name == "file_search" {
				*out = append(*out, ArtifactSignal{Type: SignalFileSearch, Value: name})
			}
			if name == "python" || strings.Contains(lower, "canmore") {
				*out = append(*out, ArtifactSignal{Type: SignalPythonTool, Value: name})
			}
		}
	}
	if recipient, ok := m["recipient"].(string); ok && recipient == "code_interpreter" {
		*out = append(*out, ArtifactSignal{Type: SignalCodeInterpreter, Value: recipient})
	}
	if content, ok := m["content"].(map[string]interface{}); ok {
		ct, _ := content["content_type"].(string)
		if ct == "execution_output" || ct == "code" {
			*out = append(*out, ArtifactSignal{Type: SignalExecutionOutput, Value: ct})
		}
		if ct == "image_asset_pointer" {
			if parts, ok := content["parts"].([]interface{}); ok {
				for _, p := range parts {
					if pm, ok := p.(map[string]interface{}); ok {
						if ap, ok := pm["asset_pointer"].(string); ok && ap != "" {
							*out = append(*out, ArtifactSignal{Type: SignalImageAsset, Value: ap})
						}
					}
				}
			}
		}
	}
	if ap, ok := m["asset_pointer"].(string); ok && ap != "" {
		*out = append(*out, ArtifactSignal{Type: SignalImageAsset, Value: ap})
	}
}

func dedupeSignals(in []ArtifactSignal) []ArtifactSignal {
	seen := make(map[string]bool, len(in))
	out := make([]ArtifactSignal, 0, len(in))
	for _, s := range in {
		key := string(s.Type) + "\x00" + s.Value
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, s)
	}
	return out
}

func MergeSignals(a, b []ArtifactSignal) []ArtifactSignal {
	return dedupeSignals(append(append([]ArtifactSignal{}, a...), b...))
}

func SandboxArtifactsFromSignals(signals []ArtifactSignal, messageID string) []SandboxArtifact {
	seen := make(map[string]bool)
	var arts []SandboxArtifact
	for _, s := range signals {
		if s.Type != SignalSandboxPath || s.Value == "" || !isValidSandboxPath(s.Value) || seen[s.Value] {
			continue
		}
		seen[s.Value] = true
		arts = append(arts, SandboxArtifact{
			MessageID:   messageID,
			SandboxPath: s.Value,
			FileName:    path.Base(s.Value),
		})
	}
	return arts
}

func ImageFileIDsFromSignals(signals []ArtifactSignal) []string {
	seen := make(map[string]bool)
	var ids []string
	for _, s := range signals {
		if s.Type != SignalImageAsset || s.Value == "" {
			continue
		}
		fid := extractFileID(s.Value)
		if fid == "" || seen[fid] {
			continue
		}
		seen[fid] = true
		ids = append(ids, fid)
	}
	return ids
}

func filterPDFArtifacts(arts []SandboxArtifact) []PDFArtifact {
	var out []PDFArtifact
	for _, art := range arts {
		if strings.HasSuffix(strings.ToLower(art.FileName), ".pdf") {
			out = append(out, PDFArtifact(art))
		}
	}
	return out
}

func sandboxArtifactKind(art SandboxArtifact) string {
	if strings.HasSuffix(strings.ToLower(art.FileName), ".pdf") {
		return "pdf"
	}
	return "sandbox_file"
}

func isValidSandboxPath(value string) bool {
	return strings.HasPrefix(value, "/mnt/data/") && path.Base(value) != "."
}

func extractFileID(value string) string {
	if value == "" {
		return ""
	}
	if match := fileIDRe.FindString(value); match != "" {
		return match
	}
	if strings.Contains(value, "//") {
		value = strings.SplitN(value, "//", 2)[1]
	}
	value = strings.Trim(value, "/")
	value = strings.Split(value, "?")[0]
	value = strings.TrimSuffix(value, "/download")
	if strings.Contains(value, "/") {
		parts := strings.Split(value, "/")
		value = parts[len(parts)-1]
	}
	if value == "" {
		return ""
	}
	return value
}

func parseGeneratedImagesFromValue(value interface{}) []parsedGeneratedImage {
	var out []parsedGeneratedImage
	walkGeneratedImages(value, "", "", "", 0, &out)
	return dedupeParsedGeneratedImages(out)
}

func walkGeneratedImages(value interface{}, messageID, genID, parentGenID string, slotIndex int, out *[]parsedGeneratedImage) {
	switch item := value.(type) {
	case map[string]interface{}:
		nextMessageID := messageID
		if id, _ := item["id"].(string); id != "" {
			if author, ok := item["author"].(map[string]interface{}); ok {
				if role, _ := author["role"].(string); role == "assistant" || role == "tool" {
					nextMessageID = id
				}
			}
		}
		nextGenID := firstNonEmpty(stringValue(item["gen_id"]), getNestedString(item, "metadata", "dalle", "gen_id"), getNestedString(item, "dalle", "gen_id"), genID)
		nextParentGenID := firstNonEmpty(stringValue(item["parent_gen_id"]), getNestedString(item, "metadata", "dalle", "parent_gen_id"), getNestedString(item, "dalle", "parent_gen_id"), parentGenID)
		nextSlotIndex := firstNonZero(intValue(item["slot_index"]), intValue(item["slot"]), intValue(getNestedValue(item, "metadata", "dalle", "slot_index")), slotIndex)

		if ap, _ := item["asset_pointer"].(string); ap != "" {
			if fid := extractFileID(ap); fid != "" {
				*out = append(*out, parsedGeneratedImage{
					FileID:      fid,
					Asset:       ap,
					MessageID:   nextMessageID,
					GenID:       nextGenID,
					ParentGenID: nextParentGenID,
					SlotIndex:   nextSlotIndex,
				})
			}
		}
		if content, ok := item["content"].(map[string]interface{}); ok {
			if ct, _ := content["content_type"].(string); ct == "image_asset_pointer" {
				if parts, ok := content["parts"].([]interface{}); ok {
					for _, p := range parts {
						if part, ok := p.(map[string]interface{}); ok {
							ap, _ := part["asset_pointer"].(string)
							if fid := extractFileID(ap); fid != "" {
								partGenID := firstNonEmpty(getNestedString(part, "metadata", "dalle", "gen_id"), nextGenID)
								partParentGenID := firstNonEmpty(getNestedString(part, "metadata", "dalle", "parent_gen_id"), nextParentGenID)
								partSlotIndex := firstNonZero(intValue(part["slot_index"]), intValue(getNestedValue(part, "metadata", "dalle", "slot_index")), nextSlotIndex)
								*out = append(*out, parsedGeneratedImage{
									FileID:      fid,
									Asset:       ap,
									MessageID:   nextMessageID,
									GenID:       partGenID,
									ParentGenID: partParentGenID,
									SlotIndex:   partSlotIndex,
								})
							}
						}
					}
				}
			}
		}
		for _, nested := range item {
			walkGeneratedImages(nested, nextMessageID, nextGenID, nextParentGenID, nextSlotIndex, out)
		}
	case []interface{}:
		for _, nested := range item {
			walkGeneratedImages(nested, messageID, genID, parentGenID, slotIndex, out)
		}
	}
}

func dedupeParsedGeneratedImages(in []parsedGeneratedImage) []parsedGeneratedImage {
	seen := map[string]bool{}
	var out []parsedGeneratedImage
	for _, p := range in {
		key := p.FileID + "\x00" + p.GenID + "\x00" + strconv.Itoa(p.SlotIndex)
		if p.FileID == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, p)
	}
	return out
}

func lastAssistantMessageID(value interface{}) string {
	var last string
	var walk func(interface{})
	walk = func(v interface{}) {
		switch item := v.(type) {
		case map[string]interface{}:
			if id, _ := item["id"].(string); id != "" {
				if author, ok := item["author"].(map[string]interface{}); ok {
					if role, _ := author["role"].(string); role == "assistant" || role == "tool" {
						last = id
					}
				}
			}
			for _, nested := range item {
				walk(nested)
			}
		case []interface{}:
			for _, nested := range item {
				walk(nested)
			}
		}
	}
	walk(value)
	return last
}

func firstConversationID(value interface{}) string {
	switch item := value.(type) {
	case map[string]interface{}:
		if cid, _ := item["conversation_id"].(string); cid != "" {
			return cid
		}
		for _, nested := range item {
			if cid := firstConversationID(nested); cid != "" {
				return cid
			}
		}
	case []interface{}:
		for _, nested := range item {
			if cid := firstConversationID(nested); cid != "" {
				return cid
			}
		}
	}
	return ""
}

func streamEventToMap(ev StreamEvent) map[string]interface{} {
	data, err := json.Marshal(ev)
	if err != nil {
		return map[string]interface{}{"event": ev.Event}
	}
	var out map[string]interface{}
	if err := json.Unmarshal(data, &out); err != nil {
		return map[string]interface{}{"event": ev.Event}
	}
	return out
}

func stringValue(value interface{}) string {
	text, _ := value.(string)
	return text
}

func intValue(value interface{}) int {
	switch item := value.(type) {
	case int:
		return item
	case int64:
		return int(item)
	case float64:
		return int(item)
	default:
		return 0
	}
}

func getNestedValue(value map[string]interface{}, keys ...string) interface{} {
	var current interface{} = value
	for _, key := range keys {
		asMap, ok := current.(map[string]interface{})
		if !ok {
			return nil
		}
		current = asMap[key]
	}
	return current
}

func getNestedString(value map[string]interface{}, keys ...string) string {
	return stringValue(getNestedValue(value, keys...))
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func firstNonZero(values ...int) int {
	for _, value := range values {
		if value != 0 {
			return value
		}
	}
	return 0
}

func stringSeen(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}
