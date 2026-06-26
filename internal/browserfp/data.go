package browserfp

// ─── WebGL GPU 指纹池 ──────────────────────────────────────────────────
// 来源: _third_party/gpu_profiles/*.json (102 条真实 GPU 配置)
// 涵盖 NVIDIA RTX 20~50 / AMD RX 6000~7000 / Apple M1~M4 / Intel Arc & UHD / llvmpipe

var webglUnmaskedVendors = []string{
	"Google Inc. (AMD)",
	"Google Inc. (Apple)",
	"Google Inc. (Intel)",
	"Google Inc. (Google)",
	"Google Inc. (NVIDIA)",
}

// webglUnmaskedRenderersMap 每个 vendor 对应的 renderer 列表，下标与 webglUnmaskedVendors 一致。
var webglUnmaskedRenderersMap = [][]string{
	{
		"ANGLE (AMD, AMD Radeon RX 6700 XT (radeonsi, navi22, LLVM 16.0.0, DRM 3.49, 6.5.0-15-generic), OpenGL 4.6)",
		"ANGLE (AMD, AMD Radeon RX 7900 XTX (radeonsi, navi31, LLVM 17.0.6, DRM 3.54, 6.6.0-12-generic), OpenGL 4.6)",
		"ANGLE (AMD, AMD Radeon(TM) Graphics (0x0000164E) Direct3D11 vs_5_0 ps_5_0, D3D11)",
		"ANGLE (AMD, AMD Radeon 780M Graphics (0x000015BF) Direct3D11 vs_5_0 ps_5_0, D3D11)",
		"ANGLE (AMD, ANGLE Metal Renderer: AMD Radeon Pro 5500M, Unspecified Version)",
		"ANGLE (AMD, ANGLE Metal Renderer: AMD Radeon Pro 555X, Unspecified Version)",
		"ANGLE (AMD, ANGLE Metal Renderer: AMD Radeon Pro Vega 56, Unspecified Version)",
		"ANGLE (AMD, AMD Radeon RX 6500 XT (0x0000743F) Direct3D11 vs_5_0 ps_5_0, D3D11)",
		"ANGLE (AMD, AMD Radeon RX 6600 (0x000073FF) Direct3D11 vs_5_0 ps_5_0, D3D11)",
		"ANGLE (AMD, AMD Radeon RX 6600 XT (0x000073FF) Direct3D11 vs_5_0 ps_5_0, D3D11)",
		"ANGLE (AMD, AMD Radeon RX 6650 XT (0x000073EF) Direct3D11 vs_5_0 ps_5_0, D3D11)",
		"ANGLE (AMD, AMD Radeon RX 6700 XT (0x000073DF) Direct3D11 vs_5_0 ps_5_0, D3D11)",
		"ANGLE (AMD, AMD Radeon RX 6750 XT (0x000073DF) Direct3D11 vs_5_0 ps_5_0, D3D11)",
		"ANGLE (AMD, AMD Radeon RX 6800 (0x000073BF) Direct3D11 vs_5_0 ps_5_0, D3D11)",
		"ANGLE (AMD, AMD Radeon RX 6800 XT (0x000073BF) Direct3D11 vs_5_0 ps_5_0, D3D11)",
		"ANGLE (AMD, AMD Radeon RX 6900 XT (0x00007448) Direct3D11 vs_5_0 ps_5_0, D3D11)",
		"ANGLE (AMD, AMD Radeon RX 6950 XT (0x000073A5) Direct3D11 vs_5_0 ps_5_0, D3D11)",
		"ANGLE (AMD, AMD Radeon RX 7600 (0x00007480) Direct3D11 vs_5_0 ps_5_0, D3D11)",
		"ANGLE (AMD, AMD Radeon RX 7700 XT (0x0000747E) Direct3D11 vs_5_0 ps_5_0, D3D11)",
		"ANGLE (AMD, AMD Radeon RX 7800 XT (0x0000747E) Direct3D11 vs_5_0 ps_5_0, D3D11)",
		"ANGLE (AMD, AMD Radeon RX 7900 GRE (0x0000747E) Direct3D11 vs_5_0 ps_5_0, D3D11)",
		"ANGLE (AMD, AMD Radeon RX 7900 XT (0x0000744C) Direct3D11 vs_5_0 ps_5_0, D3D11)",
		"ANGLE (AMD, AMD Radeon RX 7900 XTX (0x0000744C) Direct3D11 vs_5_0 ps_5_0, D3D11)",
	},
	{
		"ANGLE (Apple, ANGLE Metal Renderer: Apple M1, Unspecified Version)",
		"ANGLE (Apple, ANGLE Metal Renderer: Apple M1 Max, Unspecified Version)",
		"ANGLE (Apple, ANGLE Metal Renderer: Apple M1 Pro, Unspecified Version)",
		"ANGLE (Apple, ANGLE Metal Renderer: Apple M1 Ultra, Unspecified Version)",
		"ANGLE (Apple, ANGLE Metal Renderer: Apple M2, Unspecified Version)",
		"ANGLE (Apple, ANGLE Metal Renderer: Apple M2 Max, Unspecified Version)",
		"ANGLE (Apple, ANGLE Metal Renderer: Apple M2 Pro, Unspecified Version)",
		"ANGLE (Apple, ANGLE Metal Renderer: Apple M2 Ultra, Unspecified Version)",
		"ANGLE (Apple, ANGLE Metal Renderer: Apple M3, Unspecified Version)",
		"ANGLE (Apple, ANGLE Metal Renderer: Apple M3 Max, Unspecified Version)",
		"ANGLE (Apple, ANGLE Metal Renderer: Apple M3 Pro, Unspecified Version)",
		"ANGLE (Apple, ANGLE Metal Renderer: Apple M4, Unspecified Version)",
		"ANGLE (Apple, ANGLE Metal Renderer: Apple M4 Max, Unspecified Version)",
		"ANGLE (Apple, ANGLE Metal Renderer: Apple M4 Pro, Unspecified Version)",
	},
	{
		"ANGLE (Intel, Intel(R) Arc(TM) A380 Graphics (0x000056A5) Direct3D11 vs_5_0 ps_5_0, D3D11)",
		"ANGLE (Intel, Intel(R) Arc(TM) A750 Graphics (0x000056A1) Direct3D11 vs_5_0 ps_5_0, D3D11)",
		"ANGLE (Intel, Intel(R) Arc(TM) A770 Graphics (0x000056A0) Direct3D11 vs_5_0 ps_5_0, D3D11)",
		"ANGLE (Intel, Intel(R) Arc(TM) B580 Graphics (0x0000E20B) Direct3D11 vs_5_0 ps_5_0, D3D11)",
		"ANGLE (Intel, Intel(R) HD Graphics 520 (0x00001916) Direct3D11 vs_5_0 ps_5_0, D3D11)",
		"ANGLE (Intel, Intel(R) HD Graphics 5500 (0x00001616) Direct3D11 vs_5_0 ps_5_0, D3D11)",
		"ANGLE (Intel, ANGLE Metal Renderer: Intel(R) Iris(TM) Plus Graphics 645, Unspecified Version)",
		"ANGLE (Intel, ANGLE Metal Renderer: Intel(R) Iris(TM) Plus Graphics, Unspecified Version)",
		"ANGLE (Intel, ANGLE Metal Renderer: Intel(R) Iris(TM) Pro Graphics 5200, Unspecified Version)",
		"ANGLE (Intel, Intel(R) Iris(R) Xe Graphics (0x000046A8) Direct3D11 vs_5_0 ps_5_0, D3D11)",
		"ANGLE (Intel, Intel(R) Iris(R) Xe Graphics (0x00009A49) Direct3D11 vs_5_0 ps_5_0, D3D11)",
		"ANGLE (Intel, Intel(R) Iris(R) Xe Graphics (0x0000A7A1) Direct3D11 vs_5_0 ps_5_0, D3D11)",
		"ANGLE (Intel, Intel(R) Iris(R) Xe Graphics (0x0000A7A1) Direct3D11 vs_5_0 ps_5_0, D3D11)",
		"ANGLE (Intel, Mesa Intel(R) HD Graphics 520 (SKL GT2), OpenGL 4.6)",
		"ANGLE (Intel, Mesa Intel(R) HD Graphics 630 (KBL GT2), OpenGL 4.6)",
		"ANGLE (Intel, Mesa Intel(R) UHD Graphics 770 (ADL-S GT1), OpenGL 4.6)",
		"ANGLE (Intel, Mesa Intel(R) UHD Graphics (CML GT2), OpenGL 4.6)",
		"ANGLE (Intel, Mesa Intel(R) Xe Graphics (TGL GT2), OpenGL 4.6)",
		"ANGLE (Intel, Intel(R) UHD Graphics 620 (0x00005917) Direct3D11 vs_5_0 ps_5_0, D3D11)",
		"ANGLE (Intel, Intel(R) UHD Graphics 630 (0x00003E9B) Direct3D11 vs_5_0 ps_5_0, D3D11)",
		"ANGLE (Intel, Intel(R) UHD Graphics 730 (0x00004C8B) Direct3D11 vs_5_0 ps_5_0, D3D11)",
		"ANGLE (Intel, Intel(R) UHD Graphics 770 (0x00004680) Direct3D11 vs_5_0 ps_5_0, D3D11)",
	},
	{
		"ANGLE (Google, Vulkan 1.3.0 (SwiftShader Device (Subzero) (0x0000C0DE)), SwiftShader driver)",
	},
	{
		"ANGLE (NVIDIA, NVIDIA GeForce GTX 1650 (0x00001F82) Direct3D11 vs_5_0 ps_5_0, D3D11)",
		"ANGLE (NVIDIA, NVIDIA GeForce GTX 1660 SUPER (0x000021C4) Direct3D11 vs_5_0 ps_5_0, D3D11)",
		"ANGLE (NVIDIA, NVIDIA GeForce RTX 2060 SUPER (0x00001F06) Direct3D11 vs_5_0 ps_5_0, D3D11)",
		"ANGLE (NVIDIA, NVIDIA GeForce RTX 2060 (0x00001E89) Direct3D11 vs_5_0 ps_5_0, D3D11)",
		"ANGLE (NVIDIA, NVIDIA GeForce RTX 2070 with Max-Q Design (0x00001F10) Direct3D11 vs_5_0 ps_5_0, D3D11)",
		"ANGLE (NVIDIA, NVIDIA GeForce RTX 2070 SUPER (0x00001E84) Direct3D11 vs_5_0 ps_5_0, D3D11)",
		"ANGLE (NVIDIA, NVIDIA GeForce RTX 2070 (0x00001F02) Direct3D11 vs_5_0 ps_5_0, D3D11)",
		"ANGLE (NVIDIA, NVIDIA GeForce RTX 2080 SUPER (0x00001E81) Direct3D11 vs_5_0 ps_5_0, D3D11)",
		"ANGLE (NVIDIA, NVIDIA GeForce RTX 2080 Ti (0x00001E04) Direct3D11 vs_5_0 ps_5_0, D3D11)",
		"ANGLE (NVIDIA, NVIDIA GeForce RTX 2080 (0x00001E87) Direct3D11 vs_5_0 ps_5_0, D3D11)",
		"ANGLE (NVIDIA, NVIDIA GeForce RTX 3050 Laptop GPU (0x000025A2) Direct3D11 vs_5_0 ps_5_0, D3D11)",
		"ANGLE (NVIDIA, NVIDIA GeForce RTX 3050 Ti Laptop GPU (0x000025A0) Direct3D11 vs_5_0 ps_5_0, D3D11)",
		"ANGLE (NVIDIA, NVIDIA GeForce RTX 3050 (0x00002507) Direct3D11 vs_5_0 ps_5_0, D3D11)",
		"ANGLE (NVIDIA Corporation, NVIDIA GeForce RTX 3060/PCIe/SSE2, OpenGL 4.6.0 NVIDIA 535.146.02)",
		"ANGLE (NVIDIA, NVIDIA GeForce RTX 3060 Ti (0x00002489) Direct3D11 vs_5_0 ps_5_0, D3D11)",
		"ANGLE (NVIDIA, NVIDIA GeForce RTX 3060 (0x00002504) Direct3D11 vs_5_0 ps_5_0, D3D11)",
		"ANGLE (NVIDIA, NVIDIA GeForce RTX 3070 (0x00002488) Direct3D11 vs_5_0 ps_5_0, D3D11)",
		"ANGLE (NVIDIA, NVIDIA GeForce RTX 3080 Laptop GPU (0x000024DC) Direct3D11 vs_5_0 ps_5_0, D3D11)",
		"ANGLE (NVIDIA, NVIDIA GeForce RTX 3080 Laptop GPU (0x0000249C) Direct3D11 vs_5_0 ps_5_0, D3D11)",
		"ANGLE (NVIDIA, NVIDIA GeForce RTX 3080 Ti Laptop GPU (0x00002420) Direct3D11 vs_5_0 ps_5_0, D3D11)",
		"ANGLE (NVIDIA, NVIDIA GeForce RTX 3080 Ti (0x00002208) Direct3D11 vs_5_0 ps_5_0, D3D11)",
		"ANGLE (NVIDIA, NVIDIA GeForce RTX 3080 (0x00002206) Direct3D11 vs_5_0 ps_5_0, D3D11)",
		"ANGLE (NVIDIA, NVIDIA GeForce RTX 3090 Ti (0x00002203) Direct3D11 vs_5_0 ps_5_0, D3D11)",
		"ANGLE (NVIDIA, NVIDIA GeForce RTX 3090 (0x00002204) Direct3D11 vs_5_0 ps_5_0, D3D11)",
		"ANGLE (NVIDIA, NVIDIA GeForce RTX 4050 Laptop GPU (0x000028A1) Direct3D11 vs_5_0 ps_5_0, D3D11)",
		"ANGLE (NVIDIA, NVIDIA GeForce RTX 4060 Laptop GPU (0x000028E0) Direct3D11 vs_5_0 ps_5_0, D3D11)",
		"ANGLE (NVIDIA, NVIDIA GeForce RTX 4060 Ti (0x00002803) Direct3D11 vs_5_0 ps_5_0, D3D11)",
		"ANGLE (NVIDIA, NVIDIA GeForce RTX 4060 (0x00002882) Direct3D11 vs_5_0 ps_5_0, D3D11)",
		"ANGLE (NVIDIA Corporation, NVIDIA GeForce RTX 4070/PCIe/SSE2, OpenGL 4.6.0 NVIDIA 550.54.14)",
		"ANGLE (NVIDIA, NVIDIA GeForce RTX 4070 SUPER (0x00002783) Direct3D11 vs_5_0 ps_5_0, D3D11)",
		"ANGLE (NVIDIA, NVIDIA GeForce RTX 4070 Ti SUPER (0x00002705) Direct3D11 vs_5_0 ps_5_0, D3D11)",
		"ANGLE (NVIDIA, NVIDIA GeForce RTX 4070 Ti (0x00002782) Direct3D11 vs_5_0 ps_5_0, D3D11)",
		"ANGLE (NVIDIA, NVIDIA GeForce RTX 4070 (0x00002786) Direct3D11 vs_5_0 ps_5_0, D3D11)",
		"ANGLE (NVIDIA, NVIDIA GeForce RTX 4080 SUPER (0x00002702) Direct3D11 vs_5_0 ps_5_0, D3D11)",
		"ANGLE (NVIDIA, NVIDIA GeForce RTX 4080 (0x00002704) Direct3D11 vs_5_0 ps_5_0, D3D11)",
		"ANGLE (NVIDIA, NVIDIA GeForce RTX 4090 (0x00002684) Direct3D11 vs_5_0 ps_5_0, D3D11)",
		"ANGLE (NVIDIA, NVIDIA RTX 5000 Ada Generation (0x000026B2) Direct3D11 vs_5_0 ps_5_0, D3D11)",
		"ANGLE (NVIDIA, NVIDIA GeForce RTX 5060 (0x00002D05) Direct3D11 vs_5_0 ps_5_0, D3D11)",
		"ANGLE (NVIDIA, NVIDIA GeForce RTX 5070 Ti (0x00002C02) Direct3D11 vs_5_0 ps_5_0, D3D11)",
		"ANGLE (NVIDIA, NVIDIA GeForce RTX 5070 (0x00002C05) Direct3D11 vs_5_0 ps_5_0, D3D11)",
		"ANGLE (NVIDIA, NVIDIA GeForce RTX 5080 (0x00002C04) Direct3D11 vs_5_0 ps_5_0, D3D11)",
		"ANGLE (NVIDIA, NVIDIA GeForce RTX 5090 (0x00002B85) Direct3D11 vs_5_0 ps_5_0, D3D11)",
	},
}

// ─── 网络 & 屏幕 & DPI ────────────────────────────────────────────────

var networkDownlinks = []float64{
	0.7, 1.75, 2.3, 3.25, 1.3, 5.95, 6.75, 5.2, 8.45, 9.75, 10, 8.8, 5.6, 8.9, 9.6, 9,
	7.8, 9.35, 9.3, 2.75, 3.75, 4, 4.25, 4.75, 4.5, 3.5, 5, 5.25, 5.5, 5.75, 1.5, 1.25,
	6, 1, 0.25, 7.75, 7, 0.5, 6.5, 6.25, 8.5, 8.75, 8, 7.5, 8.25, 9.25, 9.5, 8.05,
	2.5, 2, 2.25, 3, 1.4, 1.15, 1.9, 1.65, 2.65, 2.15, 2.9, 2.4, 3.65, 3.9, 3.15, 3.4,
	3.55, 0.45, 4.3, 4.65, 4.9, 4.15, 4.4, 5.4, 5.15, 5.65, 0.75, 5.9, 6.15, 6.65, 6.9,
	7.9, 7.4, 7.65, 7.15, 8.15, 8.65, 9.15, 9.4, 9.9, 9.65, 0.15, 2.8, 2.05, 2.55, 3.05,
	0.8, 3.8, 3.3, 0.55, 4.8, 1.55, 4.55, 1.8, 1.05, 5.55, 5.3, 5.05, 5.8, 4.05, 6.3,
	6.55, 6.05, 6.8, 7.3, 7.05, 7.55, 0.6, 0.4, 0.85, 0.65, 0.9, 7.1, 0.35, 7.35, 1.85,
	8.85, 8.2, 8.7, 8.55, 8.3, 8.95, 9.7, 9.2, 9.8, 0.95, 9.95, 9.55, 9.05, 1.45, 1.2,
	1.7, 1.95, 2.7, 2.95, 2.45, 2.2, 3.95, 3.7, 3.45, 3.2, 4.2, 4.7, 4.45, 4.95, 5.45,
	5.7, 6.95, 6.7, 6.45, 6.2, 7.7, 7.45, 7.2, 7.95, 0.3, 4.85, 4.1, 4.6, 4.35, 5.1,
	5.35, 5.85, 6.6, 6.1, 6.35, 6.85, 7.6, 7.85, 1.35, 1.6, 1.1, 8.6, 8.35, 2.85, 2.1,
	2.35, 9.85, 9.1, 3.85, 3.6, 2.6, 3.35, 3.1,
}

var networkRTTs = []int{
	900, 650, 400, 1300, 150, 800, 550, 300, 50, 3000,
	700, 450, 200, 850, 600, 350, 100, 1000, 750, 500, 250,
}

var screenResolutions = [][2]int{
	{1920, 1080}, {1920, 1200}, {2048, 1080}, {2560, 1440}, {1366, 768},
	{1440, 900}, {1536, 864}, {1680, 1050}, {1280, 1024}, {1280, 800},
	{1280, 720}, {1600, 1200}, {1600, 900},
	// 以下为真实设备常见分辨率补充
	{2560, 1600}, // MacBook Pro 16" / 30" monitor
	{3840, 2160}, // 4K UHD
	{3440, 1440}, // Ultrawide 21:9
	{2736, 1824}, // Surface Pro
	{2880, 1800}, // MacBook Pro 15" Retina
	{2256, 1504}, // Surface Laptop
	{3072, 1920}, // MacBook Pro 16" scaled
	{1024, 768},  // old laptop / iPad
	{1360, 768},  // common budget laptop
	{3200, 2000}, // Lenovo / Huawei 3:2
	{3456, 2234}, // MacBook Pro 16" M3
	{2560, 1080}, // Ultrawide 21:9 (smaller)
}

// Device pixel ratios — 对齐真实浏览器 navigator.devicePixelRatio 分布。
// 1.0 最常见（Windows 桌面/普通笔记本），2.0 次常见（Retina Mac / 4K 高DPI）。
var devicePixelRatios = []float64{
	1.0, 1.0, 1.0, 1.0,
	1.25, 1.25,
	1.5, 1.5, 1.5,
	2.0, 2.0, 2.0, 2.0,
	2.5,
	3.0,
}
