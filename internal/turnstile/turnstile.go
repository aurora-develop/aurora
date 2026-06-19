// Package turnstile 实现 chatgpt turnstile 挑战求解。
// 基于真实 sdk.js 的 35-opcode VM 反编译实现 (不需要外部 captcha 服务 / 浏览器)。
package turnstile

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	turnstileQueueReg    = 9
	turnstileWindowReg   = 10
	turnstileKeyReg      = 16
	turnstileSuccessReg  = 3
	turnstileErrorReg    = 4
	turnstileCallbackReg = 30
	orderedKeysMeta      = "__ordered_keys__"
	prototypeMeta        = "__prototype__"
)

type vmFunc func(args ...any) (any, error)

type regMapRef struct {
	solver *turnstileSolver
}

type turnstileSolver struct {
	profile   turnstileRequirementsProfile
	regs      map[string]any
	window    map[string]any
	done      bool
	resolved  string
	rejected  string
	stepCount int
	maxSteps  int
}

type turnstileRequirementsProfile struct {
	ScreenSum           int
	UserAgent           string
	Language            string
	LanguagesJoin       string
	NavigatorProbe      string
	DocumentProbe       string
	WindowProbe         string
	PerformanceNow      float64
	SessionID           string
	HardwareConcurrency int
	TimeOrigin          float64
}

type browserHints struct {
	sendClientHints bool
	versionMajor    string
	versionFull     string
	platformName    string
	platformVersion string
	architecture    string
	vendor          string
	webGLVendor     string
	webGLRenderer   string
}

// SolveDX 给定 requirementsToken (gAAAAAC<base64>~S) 和 dx 挑战,返回 turnstile response token。
func SolveDX(requirementsToken, dx string) (string, error) {
	profile, err := parseTurnstileRequirementsProfile(requirementsToken)
	if err != nil {
		return "", err
	}
	solver := &turnstileSolver{
		profile:  profile,
		maxSteps: 50000,
	}
	return solver.solve(requirementsToken, dx)
}

var authWindowKeyOrder = []string{
	"0", "window", "self", "document", "name", "location", "customElements", "history", "navigation", "locationbar",
	"menubar", "personalbar", "scrollbars", "statusbar", "toolbar", "status", "closed", "frames", "length", "top",
	"opener", "parent", "frameElement", "navigator", "origin", "external", "screen", "innerWidth", "innerHeight",
	"scrollX", "pageXOffset", "scrollY", "pageYOffset", "visualViewport", "screenX", "screenY", "outerWidth",
	"outerHeight", "devicePixelRatio", "event", "clientInformation", "screenLeft", "screenTop", "styleMedia",
	"onsearch", "trustedTypes", "performance", "onappinstalled", "onbeforeinstallprompt", "crypto", "indexedDB",
	"sessionStorage", "localStorage", "onbeforexrselect", "onabort", "onbeforeinput", "onbeforematch",
	"onbeforetoggle", "onblur", "oncancel", "oncanplay", "oncanplaythrough", "onchange", "onclick", "onclose",
	"oncommand", "oncontentvisibilityautostatechange", "oncontextlost", "oncontextmenu", "oncontextrestored",
	"oncuechange", "ondblclick", "ondrag", "ondragend", "ondragenter", "ondragleave", "ondragover", "ondragstart",
	"ondrop", "ondurationchange", "onemptied", "onended", "onerror", "onfocus", "onformdata", "oninput", "oninvalid",
	"onkeydown", "onkeypress", "onkeyup", "onload", "onloadeddata", "onloadedmetadata", "onloadstart", "onmousedown",
	"onmouseenter", "onmouseleave", "onmousemove", "onmouseout", "onmouseover", "onmouseup", "onmousewheel",
	"onpause", "onplay", "onplaying", "onprogress", "onratechange", "onreset", "onresize", "onscroll", "onscrollend",
	"onsecuritypolicyviolation", "onseeked", "onseeking", "onselect", "onslotchange", "onstalled", "onsubmit",
	"onsuspend", "ontimeupdate", "ontoggle", "onvolumechange", "onwaiting", "onwebkitanimationend",
	"onwebkitanimationiteration", "onwebkitanimationstart", "onwebkittransitionend", "onwheel", "onauxclick",
	"ongotpointercapture", "onlostpointercapture", "onpointerdown", "onpointermove", "onpointerup", "onpointercancel",
	"onpointerover", "onpointerout", "onpointerenter", "onpointerleave", "onselectstart", "onselectionchange",
	"onanimationend", "onanimationiteration", "onanimationstart", "ontransitionrun", "ontransitionstart",
	"ontransitionend", "ontransitioncancel", "onafterprint", "onbeforeprint", "onbeforeunload", "onhashchange",
	"onlanguagechange", "onmessage", "onmessageerror", "onoffline", "ononline", "onpagehide", "onpageshow",
	"onpopstate", "onrejectionhandled", "onstorage", "onunhandledrejection", "onunload", "isSecureContext",
	"crossOriginIsolated", "scheduler", "alert", "atob", "blur", "btoa", "cancelAnimationFrame", "cancelIdleCallback",
	"captureEvents", "clearInterval", "clearTimeout", "close", "confirm", "createImageBitmap", "fetch", "find",
	"focus", "getComputedStyle", "getSelection", "matchMedia", "moveBy", "moveTo", "open", "postMessage", "print",
	"prompt", "queueMicrotask", "releaseEvents", "reportError", "requestAnimationFrame", "requestIdleCallback",
	"resizeBy", "resizeTo", "scroll", "scrollBy", "scrollTo", "setInterval", "setTimeout", "stop", "structuredClone",
	"webkitCancelAnimationFrame", "webkitRequestAnimationFrame", "chrome", "caches", "cookieStore", "ondevicemotion",
	"ondeviceorientation", "ondeviceorientationabsolute", "onpointerrawupdate", "documentPictureInPicture",
	"sharedStorage", "fetchLater", "getScreenDetails", "queryLocalFonts", "showDirectoryPicker", "showOpenFilePicker",
	"showSaveFilePicker", "originAgentCluster", "viewport", "onpageswap", "onpagereveal", "credentialless", "fence",
	"launchQueue", "speechSynthesis", "onscrollsnapchange", "onscrollsnapchanging", "webkitRequestFileSystem",
	"webkitResolveLocalFileSystemURL", "__reactRouterContext", "$RB", "$RV", "$RC", "$RT", "__reactRouterManifest",
	"__STATSIG__", "__reactRouterVersion", "__REACT_INTL_CONTEXT__", "DD_RUM", "__SEGMENT_INSPECTOR__",
	"__reactRouterRouteModules", "__reactRouterDataRouter", "__sentinel_token_pending", "__sentinel_init_pending",
	"SentinelSDK", "rwha4gh7no",
}

var authNavigatorPrototypeKeys = []string{
	"vendorSub", "productSub", "vendor", "maxTouchPoints", "scheduling", "userActivation", "geolocation", "doNotTrack",
	"connection", "plugins", "mimeTypes", "pdfViewerEnabled", "webkitTemporaryStorage", "webkitPersistentStorage",
	"hardwareConcurrency", "cookieEnabled", "appCodeName", "appName", "appVersion", "platform", "product", "userAgent",
	"language", "languages", "onLine", "webdriver", "getGamepads", "javaEnabled", "sendBeacon", "vibrate",
	"windowControlsOverlay", "deprecatedRunAdAuctionEnforcesKAnonymity", "protectedAudience", "bluetooth",
	"storageBuckets", "clipboard", "credentials", "keyboard", "managed", "mediaDevices", "storage", "serviceWorker",
	"virtualKeyboard", "wakeLock", "deviceMemory", "userAgentData", "login", "ink", "mediaCapabilities",
	"devicePosture", "hid", "locks", "gpu", "mediaSession", "permissions", "presentation", "serial", "usb", "xr",
	"adAuctionComponents", "runAdAuction", "canLoadAdAuctionFencedFrame", "canShare", "share", "clearAppBadge",
	"getBattery", "getUserMedia", "requestMIDIAccess", "requestMediaKeySystemAccess", "setAppBadge", "webkitGetUserMedia",
	"clearOriginJoinedAdInterestGroups", "createAuctionNonce", "joinAdInterestGroup", "leaveAdInterestGroup",
	"updateAdInterestGroups", "deprecatedReplaceInURN", "deprecatedURNToURL", "getInstalledRelatedApps",
	"getInterestGroupAdAuctionData", "registerProtocolHandler", "unregisterProtocolHandler",
}

func parseTurnstileRequirementsProfile(requirementsToken string) (turnstileRequirementsProfile, error) {
	var profile turnstileRequirementsProfile
	token := strings.TrimSpace(requirementsToken)
	token = strings.TrimPrefix(token, "gAAAAAC")
	token = strings.TrimSuffix(token, "~S")
	if token == "" {
		return profile, fmt.Errorf("empty requirements token")
	}
	body, err := base64.StdEncoding.DecodeString(token)
	if err != nil {
		return profile, err
	}
	var fields []any
	if err := json.Unmarshal(body, &fields); err != nil {
		return profile, err
	}
	if len(fields) < 18 {
		return profile, fmt.Errorf("invalid requirements field count: %d", len(fields))
	}
	profile.ScreenSum = int(jsonFloat(fields[0]))
	profile.UserAgent = jsonString(fields[4])
	profile.Language = jsonString(fields[7])
	profile.LanguagesJoin = jsonString(fields[8])
	profile.NavigatorProbe = jsonString(fields[10])
	profile.DocumentProbe = jsonString(fields[11])
	profile.WindowProbe = jsonString(fields[12])
	profile.PerformanceNow = jsonFloat(fields[13])
	profile.SessionID = jsonString(fields[14])
	profile.HardwareConcurrency = int(jsonFloat(fields[16]))
	profile.TimeOrigin = jsonFloat(fields[17])
	return profile, nil
}

func (s *turnstileSolver) solve(requirementsToken, dx string) (string, error) {
	s.regs = map[string]any{}
	s.window = s.buildWindow()
	s.done = false
	s.resolved = ""
	s.rejected = ""
	s.stepCount = 0
	s.initRuntime()

	s.setReg(turnstileSuccessReg, vmFunc(func(args ...any) (any, error) {
		if !s.done {
			s.done = true
			var value any
			if len(args) > 0 {
				value = args[0]
			}
			s.resolved = latin1Base64Encode(s.jsToString(value))
		}
		return nil, nil
	}))
	s.setReg(turnstileErrorReg, vmFunc(func(args ...any) (any, error) {
		if !s.done {
			s.done = true
			var value any
			if len(args) > 0 {
				value = args[0]
			}
			s.rejected = latin1Base64Encode(s.jsToString(value))
		}
		return nil, nil
	}))
	s.setReg(turnstileCallbackReg, vmFunc(func(args ...any) (any, error) {
		if len(args) < 3 {
			return nil, nil
		}
		targetReg := args[0]
		returnReg := args[1]
		argRegs, _ := args[2].([]any)
		innerQueue := argRegs
		mappedArgRegs := []any{}
		if len(args) >= 4 {
			if mapped, ok := args[2].([]any); ok {
				mappedArgRegs = mapped
			}
			if queueValue, ok := args[3].([]any); ok {
				innerQueue = queueValue
			}
		}
		s.setReg(targetReg, vmFunc(func(callArgs ...any) (any, error) {
			if s.done {
				return nil, nil
			}
			previousQueue := s.copyQueue()
			for i, regID := range mappedArgRegs {
				if i < len(callArgs) {
					s.setReg(regID, callArgs[i])
				} else {
					s.setReg(regID, nil)
				}
			}
			s.setReg(turnstileQueueReg, copyAnySlice(innerQueue))
			err := s.runQueue()
			s.setReg(turnstileQueueReg, previousQueue)
			if err != nil {
				return err.Error(), nil
			}
			return s.getReg(returnReg), nil
		}))
		return nil, nil
	}))
	s.setReg(turnstileKeyReg, requirementsToken)

	decoded, err := latin1Base64Decode(dx)
	if err != nil {
		return "", err
	}
	plain := xorString(decoded, requirementsToken)
	var queue []any
	if err := json.Unmarshal([]byte(plain), &queue); err != nil {
		return "", err
	}
	s.setReg(turnstileQueueReg, queue)
	if err := s.runQueue(); err != nil && !s.done {
		if success, ok := s.getReg(turnstileSuccessReg).(vmFunc); ok {
			_, _ = success(fmt.Sprintf("%d: %v", s.stepCount, err))
		}
	}
	if s.rejected != "" {
		return "", errors.New(s.rejected)
	}
	if s.resolved == "" {
		return "", fmt.Errorf("turnstile vm unresolved after %d steps", s.stepCount)
	}
	return s.resolved, nil
}

func (s *turnstileSolver) initRuntime() {
	s.setReg(0, vmFunc(func(args ...any) (any, error) {
		if len(args) == 0 {
			return nil, nil
		}
		value, err := SolveDX(s.jsToString(s.getReg(turnstileKeyReg)), s.jsToString(args[0]))
		if err != nil {
			return nil, err
		}
		return value, nil
	}))
	s.setReg(1, vmFunc(func(args ...any) (any, error) {
		if len(args) < 2 {
			return nil, nil
		}
		target, keyReg := args[0], args[1]
		s.setReg(target, xorString(s.jsToString(s.getReg(target)), s.jsToString(s.getReg(keyReg))))
		return nil, nil
	}))
	s.setReg(2, vmFunc(func(args ...any) (any, error) {
		if len(args) < 2 {
			return nil, nil
		}
		s.setReg(args[0], args[1])
		return nil, nil
	}))
	s.setReg(5, vmFunc(func(args ...any) (any, error) {
		if len(args) < 2 {
			return nil, nil
		}
		left := s.getReg(args[0])
		right := s.getReg(args[1])
		if arr, ok := left.([]any); ok {
			s.setReg(args[0], append(arr, right))
			return nil, nil
		}
		if lNum, ok := s.asNumber(left); ok {
			if rNum, ok := s.asNumber(right); ok {
				s.setReg(args[0], lNum+rNum)
				return nil, nil
			}
		}
		s.setReg(args[0], s.jsToString(left)+s.jsToString(right))
		return nil, nil
	}))
	s.setReg(6, vmFunc(func(args ...any) (any, error) {
		if len(args) < 3 {
			return nil, nil
		}
		s.setReg(args[0], s.jsGetProp(s.getReg(args[1]), s.getReg(args[2])))
		return nil, nil
	}))
	s.setReg(7, vmFunc(func(args ...any) (any, error) {
		if len(args) == 0 {
			return nil, nil
		}
		_, err := s.callFn(s.getReg(args[0]), s.derefArgs(args[1:])...)
		return nil, err
	}))
	s.setReg(8, vmFunc(func(args ...any) (any, error) {
		if len(args) < 2 {
			return nil, nil
		}
		s.setReg(args[0], s.getReg(args[1]))
		return nil, nil
	}))
	s.setReg(11, vmFunc(func(args ...any) (any, error) {
		if len(args) < 2 {
			return nil, nil
		}
		pattern := s.jsToString(s.getReg(args[1]))
		rx, err := regexp.Compile(pattern)
		if err != nil {
			s.setReg(args[0], nil)
			return nil, nil
		}
		scripts, _ := s.jsGetProp(s.jsGetProp(s.window, "document"), "scripts").([]any)
		for _, item := range scripts {
			src := s.jsToString(s.jsGetProp(item, "src"))
			if src == "" {
				continue
			}
			if hit := rx.FindString(src); hit != "" {
				s.setReg(args[0], hit)
				return nil, nil
			}
		}
		s.setReg(args[0], nil)
		return nil, nil
	}))
	s.setReg(12, vmFunc(func(args ...any) (any, error) {
		if len(args) == 0 {
			return nil, nil
		}
		s.setReg(args[0], regMapRef{solver: s})
		return nil, nil
	}))
	s.setReg(13, vmFunc(func(args ...any) (any, error) {
		if len(args) < 2 {
			return nil, nil
		}
		_, err := s.callFn(s.getReg(args[1]), args[2:]...)
		if err != nil {
			s.setReg(args[0], err.Error())
		}
		return nil, nil
	}))
	s.setReg(14, vmFunc(func(args ...any) (any, error) {
		if len(args) < 2 {
			return nil, nil
		}
		var out any
		if err := json.Unmarshal([]byte(s.jsToString(s.getReg(args[1]))), &out); err != nil {
			return nil, err
		}
		s.setReg(args[0], out)
		return nil, nil
	}))
	s.setReg(15, vmFunc(func(args ...any) (any, error) {
		if len(args) < 2 {
			return nil, nil
		}
		body, err := jsJSONStringify(s.getReg(args[1]))
		if err != nil {
			return nil, err
		}
		s.setReg(args[0], body)
		return nil, nil
	}))
	s.setReg(17, vmFunc(func(args ...any) (any, error) {
		if len(args) < 2 {
			return nil, nil
		}
		value, err := s.callFn(s.getReg(args[1]), s.derefArgs(args[2:])...)
		if err != nil {
			s.setReg(args[0], err.Error())
			return nil, nil
		}
		s.setReg(args[0], value)
		return nil, nil
	}))
	s.setReg(18, vmFunc(func(args ...any) (any, error) {
		if len(args) == 0 {
			return nil, nil
		}
		decoded, err := latin1Base64Decode(s.jsToString(s.getReg(args[0])))
		if err != nil {
			return nil, err
		}
		s.setReg(args[0], decoded)
		return nil, nil
	}))
	s.setReg(19, vmFunc(func(args ...any) (any, error) {
		if len(args) == 0 {
			return nil, nil
		}
		s.setReg(args[0], latin1Base64Encode(s.jsToString(s.getReg(args[0]))))
		return nil, nil
	}))
	s.setReg(20, vmFunc(func(args ...any) (any, error) {
		if len(args) < 3 {
			return nil, nil
		}
		if s.valuesEqual(s.getReg(args[0]), s.getReg(args[1])) {
			_, err := s.callFn(s.getReg(args[2]), args[3:]...)
			return nil, err
		}
		return nil, nil
	}))
	s.setReg(21, vmFunc(func(args ...any) (any, error) {
		if len(args) < 4 {
			return nil, nil
		}
		left, _ := s.asNumber(s.getReg(args[0]))
		right, _ := s.asNumber(s.getReg(args[1]))
		threshold, _ := s.asNumber(s.getReg(args[2]))
		if math.Abs(left-right) > threshold {
			_, err := s.callFn(s.getReg(args[3]), args[4:]...)
			return nil, err
		}
		return nil, nil
	}))
	s.setReg(22, vmFunc(func(args ...any) (any, error) {
		if len(args) < 2 {
			return nil, nil
		}
		previousQueue := s.copyQueue()
		if nextQueue, ok := args[1].([]any); ok {
			s.setReg(turnstileQueueReg, copyAnySlice(nextQueue))
		} else {
			s.setReg(turnstileQueueReg, []any{})
		}
		err := s.runQueue()
		s.setReg(turnstileQueueReg, previousQueue)
		if err != nil {
			s.setReg(args[0], err.Error())
		}
		return nil, nil
	}))
	s.setReg(23, vmFunc(func(args ...any) (any, error) {
		if len(args) < 2 {
			return nil, nil
		}
		if s.getReg(args[0]) == nil {
			return nil, nil
		}
		_, err := s.callFn(s.getReg(args[1]), args[2:]...)
		return nil, err
	}))
	s.setReg(24, vmFunc(func(args ...any) (any, error) {
		if len(args) < 3 {
			return nil, nil
		}
		method := s.jsGetProp(s.getReg(args[1]), s.getReg(args[2]))
		if _, ok := method.(vmFunc); ok {
			s.setReg(args[0], method)
		} else {
			s.setReg(args[0], nil)
		}
		return nil, nil
	}))
	s.setReg(25, vmFunc(func(args ...any) (any, error) { return nil, nil }))
	s.setReg(26, vmFunc(func(args ...any) (any, error) { return nil, nil }))
	s.setReg(27, vmFunc(func(args ...any) (any, error) {
		if len(args) < 2 {
			return nil, nil
		}
		left := s.getReg(args[0])
		right := s.getReg(args[1])
		if arr, ok := left.([]any); ok {
			filtered := arr[:0]
			for _, item := range arr {
				if !s.valuesEqual(item, right) {
					filtered = append(filtered, item)
				}
			}
			s.setReg(args[0], append([]any{}, filtered...))
			return nil, nil
		}
		lNum, lok := s.asNumber(left)
		rNum, rok := s.asNumber(right)
		if lok && rok {
			s.setReg(args[0], lNum-rNum)
		}
		return nil, nil
	}))
	s.setReg(28, vmFunc(func(args ...any) (any, error) { return nil, nil }))
	s.setReg(29, vmFunc(func(args ...any) (any, error) {
		if len(args) < 3 {
			return nil, nil
		}
		left, _ := s.asNumber(s.getReg(args[1]))
		right, _ := s.asNumber(s.getReg(args[2]))
		s.setReg(args[0], left < right)
		return nil, nil
	}))
	s.setReg(33, vmFunc(func(args ...any) (any, error) {
		if len(args) < 3 {
			return nil, nil
		}
		left, _ := s.asNumber(s.getReg(args[1]))
		right, _ := s.asNumber(s.getReg(args[2]))
		s.setReg(args[0], left*right)
		return nil, nil
	}))
	s.setReg(34, vmFunc(func(args ...any) (any, error) {
		if len(args) < 2 {
			return nil, nil
		}
		s.setReg(args[0], s.getReg(args[1]))
		return nil, nil
	}))
	s.setReg(35, vmFunc(func(args ...any) (any, error) {
		if len(args) < 3 {
			return nil, nil
		}
		left, _ := s.asNumber(s.getReg(args[1]))
		right, _ := s.asNumber(s.getReg(args[2]))
		if right == 0 {
			s.setReg(args[0], float64(0))
		} else {
			s.setReg(args[0], left/right)
		}
		return nil, nil
	}))
	s.setReg(turnstileWindowReg, s.window)
}

func (s *turnstileSolver) buildWindow() map[string]any {
	ua := "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/135.0.0.0 Safari/537.36"
	lang := "en-US"
	languagesJoin := "en-US,en"
	width := 1512
	height := 982
	innerWidth := width
	innerHeight := height - 86
	outerWidth := 160
	outerHeight := 28
	screenX := -25600
	screenY := -25600
	hardwareConcurrency := 8
	heapLimit := int64(4294705152)
	deviceID := "9e5f94bc-e8a4-4e73-b8be-63364c29d753"
	timeOrigin := float64(time.Now().Add(-10 * time.Second).UnixMilli())
	performanceNow := 9270.4
	vendor := "Google Inc."
	platform := "MacIntel"
	documentProbe := "__reactContainer$b63yiita51i"
	windowProbe := "__oai_cached_session"
	if s.profile.ScreenSum > 0 {
		width, height = splitScreenSum(s.profile.ScreenSum)
		innerWidth = width
		innerHeight = maxInt(height-86, 640)
	}
	if strings.TrimSpace(s.profile.UserAgent) != "" {
		ua = s.profile.UserAgent
	}
	if strings.TrimSpace(s.profile.Language) != "" {
		lang = s.profile.Language
	}
	if strings.TrimSpace(s.profile.LanguagesJoin) != "" {
		languagesJoin = s.profile.LanguagesJoin
	}
	if s.profile.HardwareConcurrency > 0 {
		hardwareConcurrency = s.profile.HardwareConcurrency
	}
	if s.profile.TimeOrigin > 0 {
		timeOrigin = s.profile.TimeOrigin
	}
	if s.profile.PerformanceNow > 0 {
		performanceNow = s.profile.PerformanceNow
	}
	if strings.TrimSpace(s.profile.DocumentProbe) != "" {
		documentProbe = strings.TrimSpace(s.profile.DocumentProbe)
	}
	if strings.TrimSpace(s.profile.WindowProbe) != "" {
		windowProbe = strings.TrimSpace(s.profile.WindowProbe)
	}
	browserHints := deriveBrowserHints(ua, platform, vendor)
	statsigSessionID := strings.TrimSpace(s.profile.SessionID)
	if statsigSessionID == "" {
		statsigSessionID = "session-" + strings.ReplaceAll(deviceID, "-", "")
	}
	location := withOrderedKeys(map[string]any{
		"href":   "https://auth.openai.com/create-account/password",
		"search": "",
	}, []string{})
	scripts := []any{
		withOrderedKeys(map[string]any{"src": "https://sentinel.openai.com/backend-api/sentinel/sdk.js"}, []string{}),
		withOrderedKeys(map[string]any{"src": "https://sentinel.openai.com/sentinel/20260219f9f6/sdk.js"}, []string{}),
	}
	localStorageData := withOrderedKeys(map[string]any{
		"statsig.stable_id.444584300":         `"` + deviceID + `"`,
		"statsig.session_id.444584300":        fmt.Sprintf(`{"sessionID":%q,"startTime":%.0f,"lastUpdate":%.0f}`, statsigSessionID, timeOrigin, timeOrigin+performanceNow),
		"statsig.network_fallback.2742193661": `{"initialize":{"urlConfigChecksum":"3392903","url":"https://assetsconfigcdn.org/v1/initialize","expiryTime":1775799927542,"previous":[]}}`,
	}, []string{
		"statsig.stable_id.444584300",
		"statsig.session_id.444584300",
		"statsig.network_fallback.2742193661",
	})
	storageKeys := []string{
		"statsig.stable_id.444584300",
		"statsig.session_id.444584300",
		"statsig.network_fallback.2742193661",
	}
	localStorage := withOrderedKeys(map[string]any{
		"__storage_data__": localStorageData,
		"__storage_keys__": append([]string{}, storageKeys...),
		"length":           float64(len(storageKeys)),
	}, []string{})
	refreshLocalStorageMeta := func() {
		storageKeys = keysOfMap(localStorageData)
		localStorage["__storage_keys__"] = append([]string{}, storageKeys...)
		localStorage["length"] = float64(len(storageKeys))
	}
	localStorage["key"] = vmFunc(func(args ...any) (any, error) {
		if len(args) == 0 {
			return nil, nil
		}
		idx := toIntIndex(args[0])
		if idx < 0 || idx >= len(storageKeys) {
			return nil, nil
		}
		return storageKeys[idx], nil
	})
	localStorage["getItem"] = vmFunc(func(args ...any) (any, error) {
		if len(args) == 0 {
			return nil, nil
		}
		return localStorageData[s.jsToString(args[0])], nil
	})
	localStorage["setItem"] = vmFunc(func(args ...any) (any, error) {
		if len(args) < 2 {
			return nil, nil
		}
		localStorageData[s.jsToString(args[0])] = s.jsToString(args[1])
		refreshLocalStorageMeta()
		return nil, nil
	})
	localStorage["removeItem"] = vmFunc(func(args ...any) (any, error) {
		if len(args) == 0 {
			return nil, nil
		}
		delete(localStorageData, s.jsToString(args[0]))
		refreshLocalStorageMeta()
		return nil, nil
	})
	localStorage["clear"] = vmFunc(func(args ...any) (any, error) {
		for key := range localStorageData {
			delete(localStorageData, key)
		}
		refreshLocalStorageMeta()
		return nil, nil
	})
	screen := withOrderedKeys(map[string]any{
		"availWidth":  float64(width),
		"availHeight": float64(height),
		"availLeft":   float64(0),
		"availTop":    float64(0),
		"colorDepth":  float64(24),
		"pixelDepth":  float64(24),
		"width":       float64(width),
		"height":      float64(height),
	}, []string{})
	document := withOrderedKeys(map[string]any{
		"scripts":  scripts,
		"location": location,
		"documentElement": withOrderedKeys(map[string]any{
			"getAttribute": vmFunc(func(args ...any) (any, error) {
				return nil, nil
			}),
		}, []string{}),
	}, []string{
		"location",
		documentProbe,
		"_reactListeningj3rmi50kcy",
		"closure_lm_184788",
	})
	document["body"] = withOrderedKeys(map[string]any{
		"getBoundingClientRect": vmFunc(func(args ...any) (any, error) {
			return withOrderedKeys(map[string]any{
				"x":      float64(0),
				"y":      float64(0),
				"width":  float64(800),
				"height": float64(346),
				"top":    float64(0),
				"left":   float64(0),
				"right":  float64(800),
				"bottom": float64(346),
			}, []string{}), nil
		}),
	}, []string{})
	document["getElementById"] = vmFunc(func(args ...any) (any, error) {
		return document["body"], nil
	})
	document["querySelector"] = vmFunc(func(args ...any) (any, error) {
		return document["body"], nil
	})
	document["createElement"] = vmFunc(func(args ...any) (any, error) {
		tag := ""
		if len(args) > 0 {
			tag = strings.ToLower(s.jsToString(args[0]))
		}
		element := withOrderedKeys(map[string]any{
			"tagName": strings.ToUpper(tag),
			"style":   withOrderedKeys(map[string]any{}, []string{}),
			"appendChild": vmFunc(func(args ...any) (any, error) {
				if len(args) > 0 {
					return args[0], nil
				}
				return nil, nil
			}),
			"removeChild": vmFunc(func(args ...any) (any, error) {
				if len(args) > 0 {
					return args[0], nil
				}
				return nil, nil
			}),
			"remove": vmFunc(func(args ...any) (any, error) { return nil, nil }),
		}, []string{})
		if tag == "canvas" {
			element["getContext"] = vmFunc(func(args ...any) (any, error) {
				return withOrderedKeys(map[string]any{
					"getExtension": vmFunc(func(args ...any) (any, error) {
						if len(args) > 0 && s.jsToString(args[0]) == "WEBGL_debug_renderer_info" {
							return withOrderedKeys(map[string]any{
								"UNMASKED_VENDOR_WEBGL":   float64(37445),
								"UNMASKED_RENDERER_WEBGL": float64(37446),
							}, []string{}), nil
						}
						return nil, nil
					}),
					"getParameter": vmFunc(func(args ...any) (any, error) {
						if len(args) == 0 {
							return nil, nil
						}
						param := toIntIndex(args[0])
						switch param {
						case 37445, 7936:
							return browserHints.webGLVendor, nil
						case 37446, 7937:
							return browserHints.webGLRenderer, nil
						default:
							return nil, nil
						}
					}),
				}, []string{}), nil
			})
		}
		return element, nil
	})
	navigator := withOrderedKeys(map[string]any{
		"userAgent":           ua,
		"vendor":              browserHints.vendor,
		"platform":            platform,
		"hardwareConcurrency": float64(hardwareConcurrency),
		"deviceMemory":        float64(8),
		"maxTouchPoints":      float64(0),
		"language":            lang,
		"languages":           stringSliceToAny(strings.Split(strings.ReplaceAll(languagesJoin, ";q=0.9", ""), ",")),
		"webdriver":           false,
	}, []string{})
	navigator["clipboard"] = withOrderedKeys(map[string]any{}, []string{})
	navigator["xr"] = withOrderedKeys(map[string]any{}, []string{})
	navigator["storage"] = withOrderedKeys(map[string]any{
		"estimate": vmFunc(func(args ...any) (any, error) {
			return withOrderedKeys(map[string]any{
				"quota":        float64(306461727129),
				"usage":        float64(0),
				"usageDetails": withOrderedKeys(map[string]any{}, []string{}),
			}, []string{}), nil
		}),
	}, []string{})
	if browserHints.sendClientHints {
		navigator["userAgentData"] = withOrderedKeys(map[string]any{
			"brands": []any{
				withOrderedKeys(map[string]any{"brand": "Chromium", "version": browserHints.versionMajor}, []string{}),
				withOrderedKeys(map[string]any{"brand": "Google Chrome", "version": browserHints.versionMajor}, []string{}),
				withOrderedKeys(map[string]any{"brand": "Not_A Brand", "version": "24"}, []string{}),
			},
			"mobile":   false,
			"platform": browserHints.platformName,
			"getHighEntropyValues": vmFunc(func(args ...any) (any, error) {
				return withOrderedKeys(map[string]any{
					"platform":        browserHints.platformName,
					"platformVersion": browserHints.platformVersion,
					"architecture":    browserHints.architecture,
					"model":           "",
					"uaFullVersion":   browserHints.versionFull,
				}, []string{}), nil
			}),
		}, []string{})
	} else {
		navigator["userAgentData"] = nil
	}
	start := time.Now()
	navigator[prototypeMeta] = withOrderedKeys(map[string]any{}, authNavigatorPrototypeKeys)
	for _, key := range authNavigatorPrototypeKeys {
		if _, exists := navigator[prototypeMeta].(map[string]any)[key]; !exists {
			navigator[prototypeMeta].(map[string]any)[key] = nil
		}
	}
	window := withOrderedKeys(map[string]any{}, authWindowKeyOrder)
	window["Reflect"] = withOrderedKeys(map[string]any{
		"set": vmFunc(func(args ...any) (any, error) {
			if len(args) < 3 {
				return true, nil
			}
			return s.jsSetProp(args[0], args[1], args[2]), nil
		}),
	}, []string{})
	window["Object"] = withOrderedKeys(map[string]any{
		"keys": vmFunc(func(args ...any) (any, error) {
			if len(args) == 0 {
				return []any{}, nil
			}
			return objectKeys(args[0]), nil
		}),
		"getPrototypeOf": vmFunc(func(args ...any) (any, error) {
			if len(args) == 0 {
				return nil, nil
			}
			if target, ok := args[0].(map[string]any); ok {
				return target[prototypeMeta], nil
			}
			return nil, nil
		}),
		"create": vmFunc(func(args ...any) (any, error) {
			return withOrderedKeys(map[string]any{}, []string{}), nil
		}),
	}, []string{})
	window["Math"] = withOrderedKeys(map[string]any{
		"random": vmFunc(func(args ...any) (any, error) { return rand.Float64(), nil }),
		"abs": vmFunc(func(args ...any) (any, error) {
			if len(args) == 0 {
				return float64(0), nil
			}
			n, _ := s.asNumber(args[0])
			return math.Abs(n), nil
		}),
	}, []string{})
	window["JSON"] = withOrderedKeys(map[string]any{
		"parse": vmFunc(func(args ...any) (any, error) {
			if len(args) == 0 {
				return nil, nil
			}
			var out any
			if err := json.Unmarshal([]byte(s.jsToString(args[0])), &out); err != nil {
				return nil, err
			}
			return out, nil
		}),
		"stringify": vmFunc(func(args ...any) (any, error) {
			if len(args) == 0 {
				return "null", nil
			}
			body, err := jsJSONStringify(args[0])
			if err != nil {
				return nil, err
			}
			return body, nil
		}),
	}, []string{})
	window["atob"] = vmFunc(func(args ...any) (any, error) {
		if len(args) == 0 {
			return "", nil
		}
		return latin1Base64Decode(s.jsToString(args[0]))
	})
	window["btoa"] = vmFunc(func(args ...any) (any, error) {
		if len(args) == 0 {
			return "", nil
		}
		return latin1Base64Encode(s.jsToString(args[0])), nil
	})
	window["localStorage"] = localStorage
	window["document"] = document
	window["navigator"] = navigator
	window["screen"] = screen
	window["location"] = location
	window["history"] = withOrderedKeys(map[string]any{"length": float64(3)}, []string{})
	window["performance"] = withOrderedKeys(map[string]any{
		"now": vmFunc(func(args ...any) (any, error) {
			return performanceNow + float64(time.Since(start).Nanoseconds())/1e6, nil
		}),
		"timeOrigin": timeOrigin,
		"memory": withOrderedKeys(map[string]any{
			"jsHeapSizeLimit": float64(heapLimit),
		}, []string{}),
	}, []string{})
	window["Array"] = withOrderedKeys(map[string]any{
		"from": vmFunc(func(args ...any) (any, error) {
			if len(args) == 0 {
				return []any{}, nil
			}
			switch value := args[0].(type) {
			case []any:
				return append([]any{}, value...), nil
			case []string:
				out := make([]any, 0, len(value))
				for _, item := range value {
					out = append(out, item)
				}
				return out, nil
			default:
				return []any{}, nil
			}
		}),
	}, []string{})
	window["0"] = window
	window["innerWidth"] = float64(innerWidth)
	window["innerHeight"] = float64(innerHeight)
	window["outerWidth"] = float64(outerWidth)
	window["outerHeight"] = float64(outerHeight)
	window["screenX"] = float64(screenX)
	window["screenY"] = float64(screenY)
	window["scrollX"] = float64(0)
	window["pageXOffset"] = float64(0)
	window["scrollY"] = float64(0)
	window["pageYOffset"] = float64(0)
	window["devicePixelRatio"] = 1.0000000149011612
	window["hardwareConcurrency"] = float64(hardwareConcurrency)
	window["isSecureContext"] = true
	window["crossOriginIsolated"] = false
	window["event"] = nil
	window["clientInformation"] = navigator
	window["screenLeft"] = float64(screenX)
	window["screenTop"] = float64(screenY)
	if browserHints.sendClientHints {
		window["chrome"] = withOrderedKeys(map[string]any{"runtime": withOrderedKeys(map[string]any{}, []string{})}, []string{})
	} else {
		window["chrome"] = nil
	}
	window["__reactRouterContext"] = withOrderedKeys(map[string]any{
		"future":         withOrderedKeys(map[string]any{}, []string{}),
		"routeDiscovery": withOrderedKeys(map[string]any{}, []string{}),
		"ssr":            true,
		"isSpaMode":      true,
		"loaderData": withOrderedKeys(map[string]any{
			"routes/layouts/client-auth-session-layout/layout": withOrderedKeys(map[string]any{
				"session": withOrderedKeys(map[string]any{
					"session_id":              statsigSessionID,
					"auth_session_logging_id": "3691f3d7-3e89-440c-99c1-0788585b7688",
				}, []string{}),
			}, []string{}),
		}, []string{}),
	}, []string{})
	window["$RB"] = []any{}
	window["$RV"] = vmFunc(func(args ...any) (any, error) { return nil, nil })
	window["$RC"] = vmFunc(func(args ...any) (any, error) { return nil, nil })
	window["$RT"] = performanceNow
	window["__reactRouterManifest"] = withOrderedKeys(map[string]any{}, []string{})
	window["__STATSIG__"] = withOrderedKeys(map[string]any{}, []string{})
	window["__reactRouterVersion"] = "7.9.3"
	window["__REACT_INTL_CONTEXT__"] = withOrderedKeys(map[string]any{}, []string{})
	window["DD_RUM"] = withOrderedKeys(map[string]any{}, []string{})
	window["__SEGMENT_INSPECTOR__"] = withOrderedKeys(map[string]any{}, []string{})
	window["__reactRouterRouteModules"] = withOrderedKeys(map[string]any{}, []string{})
	window["__reactRouterDataRouter"] = withOrderedKeys(map[string]any{}, []string{})
	window["__sentinel_token_pending"] = withOrderedKeys(map[string]any{}, []string{})
	window["__sentinel_init_pending"] = withOrderedKeys(map[string]any{}, []string{})
	window["SentinelSDK"] = withOrderedKeys(map[string]any{}, []string{})
	window["rwha4gh7no"] = vmFunc(func(args ...any) (any, error) { return nil, nil })
	window[windowProbe] = nil
	appendOrderedKey(window, windowProbe)
	document[documentProbe] = nil
	document["_reactListeningj3rmi50kcy"] = true
	document["closure_lm_184788"] = nil
	window["window"] = window
	window["self"] = window
	window["globalThis"] = window
	for _, key := range authWindowKeyOrder {
		if _, exists := window[key]; !exists {
			window[key] = nil
		}
	}
	return window
}

func deriveBrowserHints(ua, platform, vendor string) browserHints {
	hints := browserHints{
		sendClientHints: true,
		versionMajor:    "135",
		versionFull:     "135.0.0.0",
		platformName:    "macOS",
		platformVersion: "15.3.1",
		architecture:    "x86",
		vendor:          vendor,
		webGLVendor:     "Google Inc. (Apple)",
		webGLRenderer:   "ANGLE (Apple, ANGLE Metal Renderer: Apple M4 Pro, Unspecified Version)",
	}
	if strings.TrimSpace(hints.vendor) == "" {
		hints.vendor = "Google Inc."
	}
	if strings.EqualFold(strings.TrimSpace(platform), "Win32") {
		hints.platformName = "Windows"
		hints.platformVersion = "10.0.0"
		hints.webGLVendor = "Google Inc. (NVIDIA)"
		hints.webGLRenderer = "ANGLE (NVIDIA, NVIDIA GeForce RTX 4060 Direct3D11 vs_5_0 ps_5_0, D3D11)"
	}
	if version := findUserAgentVersion(ua, `Chrome/([0-9]+(?:\.[0-9]+)*)`); version != "" {
		hints.versionFull = version
		hints.versionMajor = splitVersionMajor(version)
	}
	if strings.Contains(ua, "Version/") && strings.Contains(ua, "Safari/") && !strings.Contains(ua, "Chrome/") {
		hints.sendClientHints = false
		hints.vendor = "Apple Computer, Inc."
		if version := findUserAgentVersion(ua, `Version/([0-9]+(?:\.[0-9]+)*)`); version != "" {
			hints.versionFull = version
			hints.versionMajor = splitVersionMajor(version)
		}
		hints.webGLVendor = "Apple Inc."
		hints.webGLRenderer = "Apple GPU"
	}
	return hints
}

func findUserAgentVersion(ua, pattern string) string {
	matches := regexp.MustCompile(pattern).FindStringSubmatch(ua)
	if len(matches) < 2 {
		return ""
	}
	return strings.TrimSpace(matches[1])
}

func splitVersionMajor(version string) string {
	version = strings.TrimSpace(version)
	if version == "" {
		return ""
	}
	if index := strings.IndexByte(version, '.'); index >= 0 {
		return version[:index]
	}
	return version
}

func (s *turnstileSolver) runQueue() error {
	for !s.done {
		queue, ok := s.getReg(turnstileQueueReg).([]any)
		if !ok || len(queue) == 0 {
			return nil
		}
		ins, ok := queue[0].([]any)
		s.setReg(turnstileQueueReg, queue[1:])
		if !ok || len(ins) == 0 {
			continue
		}
		fn, ok := s.getReg(ins[0]).(vmFunc)
		if !ok {
			return fmt.Errorf("vm opcode not callable: %v", ins[0])
		}
		if _, err := fn(ins[1:]...); err != nil {
			return err
		}
		s.stepCount++
		if s.stepCount > s.maxSteps {
			return fmt.Errorf("turnstile vm step overflow")
		}
	}
	return nil
}

func (s *turnstileSolver) callFn(value any, args ...any) (any, error) {
	fn, ok := value.(vmFunc)
	if !ok {
		return nil, nil
	}
	return fn(args...)
}

func (s *turnstileSolver) derefArgs(args []any) []any {
	out := make([]any, 0, len(args))
	for _, arg := range args {
		out = append(out, s.getReg(arg))
	}
	return out
}

func (s *turnstileSolver) getReg(key any) any {
	return s.regs[regKey(key)]
}

func (s *turnstileSolver) setReg(key any, value any) {
	s.regs[regKey(key)] = value
}

func (s *turnstileSolver) copyQueue() []any {
	queue, _ := s.getReg(turnstileQueueReg).([]any)
	return copyAnySlice(queue)
}

func (s *turnstileSolver) asNumber(value any) (float64, bool) {
	switch v := value.(type) {
	case nil:
		return math.NaN(), false
	case bool:
		if v {
			return 1, true
		}
		return 0, true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case float64:
		return v, true
	case string:
		if strings.TrimSpace(v) == "" {
			return 0, true
		}
		parsed, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
		if err != nil {
			return math.NaN(), false
		}
		return parsed, true
	default:
		return math.NaN(), false
	}
}

func (s *turnstileSolver) valuesEqual(left, right any) bool {
	switch l := left.(type) {
	case float64:
		r, ok := right.(float64)
		return ok && l == r
	case string:
		r, ok := right.(string)
		return ok && l == r
	case bool:
		r, ok := right.(bool)
		return ok && l == r
	case nil:
		return right == nil
	default:
		return fmt.Sprintf("%v", left) == fmt.Sprintf("%v", right)
	}
}

func (s *turnstileSolver) jsToString(value any) string {
	switch v := value.(type) {
	case nil:
		return "undefined"
	case bool:
		if v {
			return "true"
		}
		return "false"
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case float64:
		if math.IsNaN(v) {
			return "NaN"
		}
		if math.IsInf(v, 1) {
			return "Infinity"
		}
		if math.IsInf(v, -1) {
			return "-Infinity"
		}
		if math.Trunc(v) == v {
			return strconv.FormatInt(int64(v), 10)
		}
		return strconv.FormatFloat(v, 'f', -1, 64)
	case string:
		return v
	case []any:
		parts := make([]string, 0, len(v))
		for _, item := range v {
			parts = append(parts, s.jsToStringArrayItem(item))
		}
		return strings.Join(parts, ",")
	case map[string]any:
		if href, ok := v["href"].(string); ok && v["search"] != nil {
			return href
		}
		return "[object Object]"
	default:
		return fmt.Sprintf("%v", v)
	}
}

func (s *turnstileSolver) jsToStringArrayItem(value any) string {
	if value == nil {
		return ""
	}
	return s.jsToString(value)
}

func (s *turnstileSolver) jsGetProp(obj any, prop any) any {
	switch value := obj.(type) {
	case nil:
		return nil
	case regMapRef:
		return value.solver.getReg(prop)
	case map[string]any:
		propKey := s.jsToString(prop)
		if storage, ok := value["__storage_data__"].(map[string]any); ok {
			switch propKey {
			case "__storage_data__", "__storage_keys__", "length", "key", "getItem", "setItem", "removeItem", "clear":
				return value[propKey]
			default:
				if item, ok := value[propKey]; ok {
					return item
				}
				return storage[propKey]
			}
		}
		return value[propKey]
	case []any:
		if s.jsToString(prop) == "length" {
			return float64(len(value))
		}
		index := toIntIndex(prop)
		if index < 0 || index >= len(value) {
			return nil
		}
		return value[index]
	case []string:
		if s.jsToString(prop) == "length" {
			return float64(len(value))
		}
		index := toIntIndex(prop)
		if index < 0 || index >= len(value) {
			return nil
		}
		return value[index]
	case string:
		if s.jsToString(prop) == "length" {
			return float64(len(value))
		}
		index := toIntIndex(prop)
		if index < 0 || index >= len(value) {
			return nil
		}
		return string(value[index])
	default:
		return nil
	}
}

func (s *turnstileSolver) jsSetProp(obj any, prop any, value any) bool {
	switch target := obj.(type) {
	case regMapRef:
		target.solver.setReg(prop, value)
		return true
	case map[string]any:
		propKey := s.jsToString(prop)
		if storage, ok := target["__storage_data__"].(map[string]any); ok {
			switch propKey {
			case "__storage_data__", "__storage_keys__", "length", "key", "getItem", "setItem", "removeItem", "clear":
				target[propKey] = value
			default:
				storage[propKey] = value
				target[propKey] = value
				target["__storage_keys__"] = keysOfMap(storage)
				target["length"] = float64(len(keysOfMap(storage)))
			}
			return true
		}
		target[propKey] = value
		appendOrderedKey(target, propKey)
		return true
	default:
		return false
	}
}

func objectKeys(value any) []any {
	switch obj := value.(type) {
	case map[string]any:
		if storage, ok := obj["__storage_data__"].(map[string]any); ok {
			out := make([]any, 0, len(storage))
			for _, key := range keysOfMap(storage) {
				out = append(out, key)
			}
			return out
		}
		if keys, ok := obj[orderedKeysMeta].([]string); ok {
			out := make([]any, 0, len(keys))
			for _, key := range keys {
				if isInternalMetaKey(key) {
					continue
				}
				out = append(out, key)
			}
			return out
		}
		keys := keysOfMap(obj)
		out := make([]any, 0, len(keys))
		for _, key := range keys {
			if isInternalMetaKey(key) {
				continue
			}
			out = append(out, key)
		}
		return out
	case []any:
		out := make([]any, 0, len(obj))
		for idx := range obj {
			out = append(out, float64(idx))
		}
		return out
	default:
		return []any{}
	}
}

func keysOfMap(value map[string]any) []string {
	if keys, ok := value[orderedKeysMeta].([]string); ok {
		return append([]string{}, keys...)
	}
	out := make([]string, 0, len(value))
	for key := range value {
		if isInternalMetaKey(key) {
			continue
		}
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}

func withOrderedKeys(value map[string]any, keys []string) map[string]any {
	if value == nil {
		value = map[string]any{}
	}
	ordered := make([]string, 0, len(keys)+len(value))
	seen := map[string]struct{}{}
	for _, key := range keys {
		if isInternalMetaKey(key) {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		ordered = append(ordered, key)
		seen[key] = struct{}{}
	}
	existing := make([]string, 0, len(value))
	for key := range value {
		if isInternalMetaKey(key) {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		existing = append(existing, key)
	}
	sort.Strings(existing)
	ordered = append(ordered, existing...)
	value[orderedKeysMeta] = ordered
	return value
}

func appendOrderedKey(value map[string]any, key string) {
	if isInternalMetaKey(key) {
		return
	}
	keys, ok := value[orderedKeysMeta].([]string)
	if !ok {
		value[orderedKeysMeta] = []string{key}
		return
	}
	for _, existing := range keys {
		if existing == key {
			return
		}
	}
	value[orderedKeysMeta] = append(keys, key)
}

func isInternalMetaKey(key string) bool {
	switch key {
	case orderedKeysMeta, prototypeMeta, "__storage_data__", "__storage_keys__":
		return true
	default:
		return false
	}
}

func jsJSONStringify(value any) (string, error) {
	switch v := value.(type) {
	case nil:
		return "null", nil
	case bool:
		if v {
			return "true", nil
		}
		return "false", nil
	case string:
		body, err := json.Marshal(v)
		return string(body), err
	case int:
		return strconv.Itoa(v), nil
	case int64:
		return strconv.FormatInt(v, 10), nil
	case float64:
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return "null", nil
		}
		body, err := json.Marshal(v)
		return string(body), err
	case []any:
		parts := make([]string, 0, len(v))
		for _, item := range v {
			body, err := jsJSONStringify(item)
			if err != nil {
				return "", err
			}
			parts = append(parts, body)
		}
		return "[" + strings.Join(parts, ",") + "]", nil
	case []string:
		parts := make([]string, 0, len(v))
		for _, item := range v {
			body, err := jsJSONStringify(item)
			if err != nil {
				return "", err
			}
			parts = append(parts, body)
		}
		return "[" + strings.Join(parts, ",") + "]", nil
	case map[string]any:
		keys := keysOfMap(v)
		parts := make([]string, 0, len(keys))
		for _, key := range keys {
			if isInternalMetaKey(key) {
				continue
			}
			body, err := jsJSONStringify(v[key])
			if err != nil {
				return "", err
			}
			name, err := json.Marshal(key)
			if err != nil {
				return "", err
			}
			parts = append(parts, string(name)+":"+body)
		}
		return "{" + strings.Join(parts, ",") + "}", nil
	default:
		body, err := json.Marshal(v)
		return string(body), err
	}
}

func stringSliceToAny(values []string) []any {
	out := make([]any, 0, len(values))
	for _, item := range values {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		out = append(out, item)
	}
	return out
}

func splitScreenSum(sum int) (int, int) {
	common := [][2]int{
		{2048, 1152},
		{1920, 1080},
		{1536, 864},
		{1440, 900},
		{1600, 900},
		{1366, 768},
	}
	for _, item := range common {
		if item[0]+item[1] == sum {
			return item[0], item[1]
		}
	}
	if sum > 2000 {
		width := int(math.Round(float64(sum) * 0.64))
		height := sum - width
		return width, height
	}
	return sum, 0
}

func jsonString(value any) string {
	if value == nil {
		return ""
	}
	if text, ok := value.(string); ok {
		return text
	}
	return fmt.Sprintf("%v", value)
}

func jsonFloat(value any) float64 {
	switch v := value.(type) {
	case float64:
		return v
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case string:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
		if err == nil {
			return parsed
		}
	}
	return 0
}

func regKey(value any) string {
	switch v := value.(type) {
	case nil:
		return "nil"
	case string:
		return "s:" + v
	case int:
		return "n:" + strconv.Itoa(v)
	case int64:
		return "n:" + strconv.FormatInt(v, 10)
	case float64:
		if math.Trunc(v) == v {
			return "n:" + strconv.FormatInt(int64(v), 10)
		}
		return "n:" + strconv.FormatFloat(v, 'g', -1, 64)
	default:
		return "x:" + fmt.Sprintf("%v", value)
	}
}

func toIntIndex(value any) int {
	switch v := value.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		if math.Trunc(v) == v {
			return int(v)
		}
	case string:
		if parsed, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			return parsed
		}
	}
	return -1
}

func copyAnySlice(value []any) []any {
	if len(value) == 0 {
		return []any{}
	}
	out := make([]any, len(value))
	copy(out, value)
	return out
}

func latin1Base64Decode(value string) (string, error) {
	body, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return "", err
	}
	return string(bytesToLatin1Runes(body)), nil
}

func latin1Base64Encode(value string) string {
	return base64.StdEncoding.EncodeToString(latin1StringToBytes(value))
}

func xorString(data, key string) string {
	if key == "" {
		return data
	}
	dataBytes := latin1StringToBytes(data)
	keyBytes := latin1StringToBytes(key)
	out := make([]byte, len(dataBytes))
	for idx := range dataBytes {
		out[idx] = dataBytes[idx] ^ keyBytes[idx%len(keyBytes)]
	}
	return string(bytesToLatin1Runes(out))
}

func latin1StringToBytes(value string) []byte {
	bytes := make([]byte, 0, len(value))
	for _, r := range value {
		bytes = append(bytes, byte(r))
	}
	return bytes
}

func bytesToLatin1Runes(value []byte) []rune {
	out := make([]rune, 0, len(value))
	for _, b := range value {
		out = append(out, rune(b))
	}
	return out
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
