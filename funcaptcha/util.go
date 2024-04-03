package funcaptcha

import (
	"encoding/base64"
	"encoding/json"
	"math"
	mathRand "math/rand"
	"strconv"
	"strings"
	"time"
)

const chars = "0123456789abcdef"
const DEFAULT_USER_AGENT = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/105.0.0.0 Safari/537.36"

type Bda struct {
	Key   string      `json:"key"`
	Value interface{} `json:"value"`
}

func Random() string {
	result := make([]byte, 32)
	for i := range result {
		result[i] = chars[mathRand.Intn(len(chars))]
	}
	return string(result)
}

func GetBda(userAgent string, referer string, location string) string {
	fp := getFingerprint()
	fe := prepareFe(fp)
	bda := []Bda{
		{Key: "api_type", Value: "js"},
		{Key: "p", Value: 1},
		{Key: "f", Value: GetMurmur128String(prepareF(fp), 31)},
		{Key: "n", Value: base64.StdEncoding.EncodeToString([]byte(strconv.Itoa(int(math.Round(float64(time.Now().Unix()))))))},
		{Key: "wh", Value: Random() + "|" + Random()},
		{Key: "enhanced_fp", Value: []Bda{
			{
				Key:   "webgl_extensions",
				Value: "ANGLE_instanced_arrays;EXT_blend_minmax;EXT_color_buffer_half_float;EXT_disjoint_timer_query;EXT_float_blend;EXT_frag_depth;EXT_shader_texture_lod;EXT_texture_compression_bptc;EXT_texture_compression_rgtc;EXT_texture_filter_anisotropic;EXT_sRGB;KHR_parallel_shader_compile;OES_element_index_uint;OES_fbo_render_mipmap;OES_standard_derivatives;OES_texture_float;OES_texture_float_linear;OES_texture_half_float;OES_texture_half_float_linear;OES_vertex_array_object;WEBGL_color_buffer_float;WEBGL_compressed_texture_s3tc;WEBGL_compressed_texture_s3tc_srgb;WEBGL_debug_renderer_info;WEBGL_debug_shaders;WEBGL_depth_texture;WEBGL_draw_buffers;WEBGL_lose_context;WEBGL_multi_draw",
			},
			{
				Key:   "webgl_extensions_hash",
				Value: Random(),
			},
			{
				Key:   "webgl_renderer",
				Value: "WebKit WebGL",
			},
			{
				Key:   "webgl_vendor",
				Value: "WebKit",
			},
			{
				Key:   "webgl_version",
				Value: "WebGL 1.0 (OpenGL ES 2.0 Chromium)",
			},
			{
				Key:   "webgl_shading_language_version",
				Value: "WebGL GLSL ES 1.0 (OpenGL ES GLSL ES 1.0 Chromium)",
			},
			{
				Key:   "webgl_aliased_line_width_range",
				Value: "[1, 1]",
			},
			{
				Key:   "webgl_aliased_point_size_range",
				Value: "[1, 1024]",
			},
			{
				Key:   "webgl_antialiasing",
				Value: "yes",
			},
			{
				Key:   "webgl_bits",
				Value: "8,8,24,8,8,0",
			},
			{
				Key:   "webgl_max_params",
				Value: "16,32,16384,1024,16384,16,16384,30,16,16,4095",
			},
			{
				Key:   "webgl_max_viewport_dims",
				Value: "[32767, 32767]",
			},
			{
				Key:   "webgl_unmasked_vendor",
				Value: "Google Inc. (NVIDIA)",
			},
			{
				Key:   "webgl_unmasked_renderer",
				Value: "ANGLE (NVIDIA, NVIDIA GeForce RTX 3060 Ti Direct3D11 vs_5_0 ps_5_0, D3D11)",
			},
			{
				Key:   "webgl_vsf_params",
				Value: "23,127,127,23,127,127,23,127,127",
			},
			{
				Key:   "webgl_vsi_params",
				Value: "0,31,30,0,31,30,0,31,30",
			},
			{
				Key:   "webgl_fsf_params",
				Value: "23,127,127,23,127,127,23,127,127",
			},
			{
				Key:   "webgl_fsi_params",
				Value: "0,31,30,0,31,30,0,31,30",
			},
			{
				Key:   "webgl_hash_webgl",
				Value: Random(),
			},
			{
				Key:   "user_agent_data_brands",
				Value: "Chromium,Google Chrome,Not:A-Brand",
			},
			{
				Key:   "user_agent_data_mobile",
				Value: false,
			},
			{
				Key:   "navigator_connection_downlink",
				Value: 10,
			},
			{
				Key:   "navigator_connection_downlink_max",
				Value: nil,
			},
			{
				Key:   "network_info_rtt",
				Value: 50,
			},
			{
				Key:   "network_info_save_data",
				Value: false,
			},
			{
				Key:   "network_info_rtt_type",
				Value: nil,
			},
			{
				Key:   "screen_pixel_depth",
				Value: 24,
			},
			{
				Key:   "navigator_device_memory",
				Value: 8,
			},
			{
				Key:   "navigator_languages",
				Value: "en-US,fr,fr-FR,en,nl",
			},
			{
				Key:   "window_inner_width",
				Value: 0,
			},
			{
				Key:   "window_inner_height",
				Value: 0,
			},
			{
				Key:   "window_outer_width",
				Value: 1920,
			},
			{
				Key:   "window_outer_height",
				Value: 1080,
			},
			{
				Key:   "browser_detection_firefox",
				Value: false,
			},
			{
				Key:   "browser_detection_brave",
				Value: false,
			},
			{
				Key:   "audio_codecs",
				Value: "{\"ogg\":\"probably\",\"mp3\":\"probably\",\"wav\":\"probably\",\"m4a\":\"maybe\",\"aac\":\"probably\"}",
			},
			{
				Key:   "video_codecs",
				Value: "{\"ogg\":\"probably\",\"h264\":\"probably\",\"webm\":\"probably\",\"mpeg4v\":\"\",\"mpeg4a\":\"\",\"theora\":\"\"}",
			},
			{
				Key:   "media_query_dark_mode",
				Value: true,
			},
			{
				Key:   "headless_browser_phantom",
				Value: false,
			},
			{
				Key:   "headless_browser_selenium",
				Value: false,
			},
			{
				Key:   "headless_browser_nightmare_js",
				Value: false,
			},
			{
				Key:   "window__ancestor_origins",
				Value: []string{},
			},
			{
				Key:   "window__tree_index",
				Value: []string{},
			},
			{
				Key:   "window__tree_structure",
				Value: "[[],[[]]]",
			},
			{
				Key:   "client_config__surl",
				Value: nil,
			},
			{
				Key:   "client_config__language",
				Value: nil,
			},
			{
				Key:   "navigator_battery_charging",
				Value: true,
			},
			{
				Key:   "audio_fingerprint",
				Value: "124.04347527516074",
			},
			{
				Key:   "mobile_sdk__is_sdk",
				Value: "__no_value_place_holder__",
			},
		}},
		{Key: "fe", Value: fe},
		{Key: "ife_hash", Value: GetMurmur128String(strings.Join(fe, ", "), 38)},
		{Key: "cs", Value: 1},
		{Key: "jsbd", Value: "{\"HL\":4,\"DT\":\"\",\"NWD\":\"false\",\"DOTO\":1,\"DMTO\":1}"},
	}

	var enhancedFp []Bda
	for _, val := range bda {
		if val.Key == "enhanced_fp" {
			enhancedFp, _ = val.Value.([]Bda)
			break
		}
	}

	if len(referer) > 0 {
		enhancedFp = append(enhancedFp, Bda{
			Key:   "document__referrer",
			Value: referer,
		})
	}

	if len(location) > 0 {
		enhancedFp = append(enhancedFp, Bda{
			Key:   "window__location_href",
			Value: location,
		}, Bda{
			Key:   "client_config__sitedata_location_href",
			Value: location,
		})
	}

	for i := range bda {
		if bda[i].Key == "enhanced_fp" {
			bda[i].Value = enhancedFp
			break
		}
	}

	b, _ := json.Marshal(bda)
	strB := string(b)
	strB = strings.ReplaceAll(strB, `,"value":"__no_value_place_holder__"`, "")
	currentTime := time.Now().Unix()
	key := userAgent + strconv.FormatInt(currentTime-(currentTime%21600), 10)
	ciphertext := Encrypt(strB, key)
	return base64.StdEncoding.EncodeToString([]byte(ciphertext))
}
