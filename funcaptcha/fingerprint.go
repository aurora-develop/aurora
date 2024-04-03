package funcaptcha

import (
	"encoding/base64"
	"fmt"
	"math"
	"math/rand"
	"reflect"
	"strconv"
	"strings"
)

var baseFingerprint = map[string]interface{}{
	"DNT":  "unknown",
	"L":    "en-US",
	"D":    24,
	"PR":   1,
	"S":    []int{1920, 1200},
	"AS":   []int{1920, 1200},
	"TO":   9999,
	"SS":   true,
	"LS":   true,
	"IDB":  true,
	"B":    false,
	"ODB":  true,
	"CPUC": "unknown",
	"PK":   "Win32",
	"CFP":  fmt.Sprintf("canvas winding:yes~canvas fp:data:image/png;base64,%s", base64.StdEncoding.EncodeToString([]byte(strconv.FormatFloat(rand.Float64(), 'f', -1, 64)))),
	"FR":   false,
	"FOS":  false,
	"FB":   false,
	"JSF":  "",
	"P": []string{
		"Chrome PDF Plugin::Portable Document Format::application/x-google-chrome-pdf~pdf",
		"Chrome PDF Viewer::::application/pdf~pdf",
		"Native Client::::application/x-nacl~,application/x-pnacl~",
	},
	"T":   []interface{}{0, false, false},
	"H":   24,
	"SWF": false, // Flash support
}

var languages = []string{
	"af",
	"af-ZA",
	"ar",
	"ar-AE",
	"ar-BH",
	"ar-DZ",
	"ar-EG",
	"ar-IQ",
	"ar-JO",
	"ar-KW",
	"ar-LB",
	"ar-LY",
	"ar-MA",
	"ar-OM",
	"ar-QA",
	"ar-SA",
	"ar-SY",
	"ar-TN",
	"ar-YE",
	"az",
	"az-AZ",
	"az-AZ",
	"be",
	"be-BY",
	"bg",
	"bg-BG",
	"bs-BA",
	"ca",
	"ca-ES",
	"cs",
	"cs-CZ",
	"cy",
	"cy-GB",
	"da",
	"da-DK",
	"de",
	"de-AT",
	"de-CH",
	"de-DE",
	"de-LI",
	"de-LU",
	"dv",
	"dv-MV",
	"el",
	"el-GR",
	"en",
	"en-AU",
	"en-BZ",
	"en-CA",
	"en-CB",
	"en-GB",
	"en-IE",
	"en-JM",
	"en-NZ",
	"en-PH",
	"en-TT",
	"en-US",
	"en-ZA",
	"en-ZW",
	"eo",
	"es",
	"es-AR",
	"es-BO",
	"es-CL",
	"es-CO",
	"es-CR",
	"es-DO",
	"es-EC",
	"es-ES",
	"es-ES",
	"es-GT",
	"es-HN",
	"es-MX",
	"es-NI",
	"es-PA",
	"es-PE",
	"es-PR",
	"es-PY",
	"es-SV",
	"es-UY",
	"es-VE",
	"et",
	"et-EE",
	"eu",
	"eu-ES",
	"fa",
	"fa-IR",
	"fi",
	"fi-FI",
	"fo",
	"fo-FO",
	"fr",
	"fr-BE",
	"fr-CA",
	"fr-CH",
	"fr-FR",
	"fr-LU",
	"fr-MC",
	"gl",
	"gl-ES",
	"gu",
	"gu-IN",
	"he",
	"he-IL",
	"hi",
	"hi-IN",
	"hr",
	"hr-BA",
	"hr-HR",
	"hu",
	"hu-HU",
	"hy",
	"hy-AM",
	"id",
	"id-ID",
	"is",
	"is-IS",
	"it",
	"it-CH",
	"it-IT",
	"ja",
	"ja-JP",
	"ka",
	"ka-GE",
	"kk",
	"kk-KZ",
	"kn",
	"kn-IN",
	"ko",
	"ko-KR",
	"kok",
	"kok-IN",
	"ky",
	"ky-KG",
	"lt",
	"lt-LT",
	"lv",
	"lv-LV",
	"mi",
	"mi-NZ",
	"mk",
	"mk-MK",
	"mn",
	"mn-MN",
	"mr",
	"mr-IN",
	"ms",
	"ms-BN",
	"ms-MY",
	"mt",
	"mt-MT",
	"nb",
	"nb-NO",
	"nl",
	"nl-BE",
	"nl-NL",
	"nn-NO",
	"ns",
	"ns-ZA",
	"pa",
	"pa-IN",
	"pl",
	"pl-PL",
	"ps",
	"ps-AR",
	"pt",
	"pt-BR",
	"pt-PT",
	"qu",
	"qu-BO",
	"qu-EC",
	"qu-PE",
	"ro",
	"ro-RO",
	"ru",
	"ru-RU",
	"sa",
	"sa-IN",
	"se",
	"se-FI",
	"se-FI",
	"se-FI",
	"se-NO",
	"se-NO",
	"se-NO",
	"se-SE",
	"se-SE",
	"se-SE",
	"sk",
	"sk-SK",
	"sl",
	"sl-SI",
	"sq",
	"sq-AL",
	"sr-BA",
	"sr-BA",
	"sr-SP",
	"sr-SP",
	"sv",
	"sv-FI",
	"sv-SE",
	"sw",
	"sw-KE",
	"syr",
	"syr-SY",
	"ta",
	"ta-IN",
	"te",
	"te-IN",
	"th",
	"th-TH",
	"tl",
	"tl-PH",
	"tn",
	"tn-ZA",
	"tr",
	"tr-TR",
	"tt",
	"tt-RU",
	"ts",
	"uk",
	"uk-UA",
	"ur",
	"ur-PK",
	"uz",
	"uz-UZ",
	"uz-UZ",
	"vi",
	"vi-VN",
	"xh",
	"xh-ZA",
	"zh",
	"zh-CN",
	"zh-HK",
	"zh-MO",
	"zh-SG",
	"zh-TW",
	"zu",
	"zu-ZA",
}

var screenRes = [][]int{
	{1920, 1080},
	{1920, 1200},
	{2048, 1080},
	{2560, 1440},
	{1366, 768},
	{1440, 900},
	{1536, 864},
	{1680, 1050},
	{1280, 1024},
	{1280, 800},
	{1280, 720},
	{1600, 1200},
	{1600, 900},
}

type Fingerprint map[string]interface{}

func randomScreenRes() []int {
	return screenRes[rand.Intn(len(screenRes))]
}

func getFingerprint() Fingerprint {
	f := make(Fingerprint)
	for k, v := range baseFingerprint {
		f[k] = v
	}
	f["DNT"] = "unknown"
	f["L"] = languages[rand.Intn(len(languages))]
	f["D"] = []int{1, 4, 8, 15, 16, 24, 32, 48}[rand.Intn(8)]
	f["PR"] = float64(rand.Intn(100))/100.0*2 + 0.5
	screenRes := randomScreenRes()
	f["S"] = screenRes
	f["AS"] = []int{screenRes[0], screenRes[1] - 40}
	f["TO"] = (rand.Intn(24) - 12) * 60
	f["SS"] = rand.Float64() > 0.5
	f["LS"] = rand.Float64() > 0.5
	f["IDB"] = rand.Float64() > 0.5
	f["B"] = rand.Float64() > 0.5
	f["ODB"] = rand.Float64() > 0.5
	f["CPUC"] = "unknown"
	f["PK"] = []string{"HP-UX", "Mac68K", "MacPPC", "SunOS", "Win16", "Win32", "WinCE"}[rand.Intn(7)]
	f["CFP"] = ""
	f["FR"] = false
	f["FOS"] = false
	f["FB"] = false
	f["JSF"] = ""
	if val, ok := f["P"].([]string); ok {
		newVal := make([]string, 0, len(val))
		for _, v := range val {
			if rand.Float64() > 0.5 {
				newVal = append(newVal, v)
			}
		}
		f["P"] = newVal
	}
	f["T"] = []interface{}{rand.Intn(8), rand.Float64() > 0.5, rand.Float64() > 0.5}
	f["H"] = math.Pow(2, float64(rand.Intn(6)))

	return f
}

func prepareF(f Fingerprint) string {
	var res []string
	for key := range f {
		val := f[key]
		switch reflect.TypeOf(val).Kind() {
		case reflect.Slice:
			s := reflect.ValueOf(val)
			sliceOfString := make([]string, s.Len())
			for i := 0; i < s.Len(); i++ {
				sliceOfString[i] = fmt.Sprintf("%v", s.Index(i).Interface())
			}
			res = append(res, strings.Join(sliceOfString, ";"))
		default:
			res = append(res, fmt.Sprintf("%v", val))
		}
	}
	return strings.Join(res, "~~~")
}

func prepareFe(fingerprint Fingerprint) []string {
	fe := make([]string, 0)
	for k, v := range fingerprint {
		switch k {
		case "P":
			value, ok := v.([]string)
			if ok {
				mapped := MapSlice(value, func(s string) string {
					split := strings.Split(s, "::")
					return split[0]
				})
				fe = append(fe, fmt.Sprintf("%s:%s", k, strings.Join(mapped, ",")))
			}
		default:
			fe = append(fe, fmt.Sprintf("%s:%v", k, v))
		}
	}
	return fe
}

func MapSlice(a []string, f func(string) string) []string {
	n := make([]string, len(a))
	for i, e := range a {
		n[i] = f(e)
	}
	return n
}
