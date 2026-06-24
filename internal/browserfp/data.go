package browserfp

var webglUnmaskedRenderers = []string{
	"ANGLE (NVIDIA, NVIDIA GeForce GT 720 Direct3D11 vs_5_0 ps_5_0, D3D11)",
	"ANGLE (NVIDIA, NVIDIA GeForce RTX 3090 Ti (0x00002203) Direct3D11 vs_5_0 ps_5_0, D3D11)",
	"ANGLE (NVIDIA, NVIDIA GeForce GTX 1650 (0x00001F0A) Direct3D11 vs_5_0 ps_5_0, D3D11)",
	"ANGLE (Intel, Intel(R) UHD Graphics 750 Direct3D11 vs_5_0 ps_5_0, D3D11)",
	"ANGLE (NVIDIA, NVIDIA GeForce RTX 3050 Laptop GPU (0x000025A2) Direct3D11 vs_5_0 ps_5_0, D3D11)",
	"ANGLE (Intel, Intel(R) UHD Graphics 620 (0x00005917) Direct3D11 vs_5_0 ps_5_0, D3D11)",
	"ANGLE (NVIDIA, NVIDIA GeForce RTX 3060 Ti Direct3D11 vs_5_0 ps_5_0, D3D11-31.0.15.3742)",
	"ANGLE (AMD, AMD Radeon 780M Graphics (0x000015BF) Direct3D11 vs_5_0 ps_5_0, D3D11)",
	"ANGLE (NVIDIA, NVIDIA GeForce RTX 3060 (0x00002503) Direct3D11 vs_5_0 ps_5_0, D3D11)",
	"ANGLE (AMD, AMD Radeon(TM) Vega 8 Graphics Direct3D11 vs_5_0 ps_5_0, D3D11)",
	"ANGLE (NVIDIA Corporation, NVIDIA GeForce RTX 3060/PCIe/SSE2, OpenGL 4.5.0)",
	"ANGLE (NVIDIA, NVIDIA Quadro P5000 (0x00001BB0) Direct3D11 vs_5_0 ps_5_0, D3D11)",
	"ANGLE (NVIDIA, NVIDIA GeForce RTX 3060 Laptop GPU Direct3D11 vs_5_0 ps_5_0, D3D11)",
	"ANGLE (NVIDIA, NVIDIA GeForce RTX 2080 (0x00001E87) Direct3D11 vs_5_0 ps_5_0, D3D11)",
	"ANGLE (NVIDIA, NVIDIA GeForce GTX 950 Direct3D11 vs_5_0 ps_5_0, D3D11)",
	"ANGLE (Intel, Intel(R) UHD Graphics 730 (0x00004C8B) Direct3D11 vs_5_0 ps_5_0, D3D11)",
	"ANGLE (VMware, VMware SVGA 3D (0x00000405) Direct3D11 vs_5_0 ps_5_0, D3D11)",
	"ANGLE (Intel, Intel(R) UHD Graphics 630 Direct3D11 vs_5_0 ps_5_0, D3D11-30.0.101.1340)",
	"ANGLE (AMD, AMD Radeon(TM) Graphics (0x00001638) Direct3D11 vs_5_0 ps_5_0, D3D11)",
	"Google SwiftShader",
	"ANGLE (NVIDIA, NVIDIA GeForce RTX 3070 Laptop GPU (0x000024DD) Direct3D11 vs_5_0 ps_5_0, D3D11)",
	"Mali-G78",
	"Adreno (TM) 640",
	"ANGLE (NVIDIA, NVIDIA Quadro P400 Direct3D11 vs_5_0 ps_5_0, D3D11)",
	"ANGLE (Intel(R) HD Graphics 400 Direct3D11 vs_5_0 ps_5_0)",
	"ANGLE (AMD, AMD Radeon RX 640 (0x00006987) Direct3D11 vs_5_0 ps_5_0, D3D11)",
	"ANGLE (NVIDIA, NVIDIA GeForce RTX 2060 (0x00001F15) Direct3D11 vs_5_0 ps_5_0, D3D11)",
	"ANGLE (NVIDIA, NVIDIA GeForce RTX 3060 Laptop GPU (0x00002560) Direct3D11 vs_5_0 ps_5_0, D3D11)",
	"ANGLE (NVIDIA, NVIDIA GeForce RTX 3050 Laptop GPU (0x000025E2) Direct3D11 vs_5_0 ps_5_0, D3D11)",
	"Intel Iris OpenGL Engine",
	"ANGLE (NVIDIA, NVIDIA GeForce RTX 3060 Laptop GPU (0x00002520) Direct3D11 vs_5_0 ps_5_0, D3D11)",
	"Apple GPU",
	"ANGLE (Intel Inc., Intel(R) UHD Graphics 617, OpenGL 4.1)",
	"ANGLE (NVIDIA GeForce GTX 1060 Direct3D11 vs_5_0 ps_5_0)",
	"ANGLE (NVIDIA GeForce RTX 2060 Direct3D11 vs_5_0 ps_5_0)",
	"ANGLE (Intel, Intel(R) HD Graphics 4000 (0x00000166) Direct3D11 vs_5_0 ps_5_0, D3D11)",
	"ANGLE (Intel, Intel(R) Iris(R) Xe Graphics (0x00009A49) Direct3D11 vs_5_0 ps_5_0, D3D11)",
	"ANGLE (NVIDIA, NVIDIA GeForce RTX 2060 SUPER (0x00001F06) Direct3D11 vs_5_0 ps_5_0, D3D11)",
	"ANGLE (NVIDIA, NVIDIA GeForce RTX 4060 Laptop GPU (0x000028E0) Direct3D11 vs_5_0 ps_5_0, D3D11)",
	"ANGLE (NVIDIA, NVIDIA GeForce MX550 Direct3D11 vs_5_0 ps_5_0, D3D11)",
	"ANGLE (Intel, Intel(R) UHD Graphics (0x00009BC4) Direct3D11 vs_5_0 ps_5_0, D3D11)",
	"ANGLE (AMD, Radeon RX550/550 Series Direct3D11 vs_5_0 ps_5_0, D3D11)",
	"ANGLE (NVIDIA, NVIDIA Quadro P400 (0x00001CB3) Direct3D11 vs_5_0 ps_5_0, D3D11)",
	"ANGLE (NVIDIA, NVIDIA GeForce GTX 750 Ti (0x00001380) Direct3D11 vs_5_0 ps_5_0, D3D11)",
	"ANGLE (Intel, Intel(R) UHD Graphics 630 (0x00003E98) Direct3D11 vs_5_0 ps_5_0, D3D11)",
	"ANGLE (NVIDIA, NVIDIA GeForce RTX 3070 (0x00002488) Direct3D11 vs_5_0 ps_5_0, D3D11)",
	"ANGLE (AMD, AMD Radeon 780M Graphics Direct3D11 vs_5_0 ps_5_0, D3D11)",
	"ANGLE (Intel, Mesa Intel(R) Graphics (ADL GT2), OpenGL 4.6)",
	"ANGLE (AMD, AMD Radeon(TM) RX Vega 10 Graphics (0x000015D8) Direct3D11 vs_5_0 ps_5_0, D3D11)",
	"ANGLE (AMD, Radeon RX 570 Series Direct3D11 vs_5_0 ps_5_0, D3D11-30.0.15002.1004)",
	"ANGLE (NVIDIA, NVIDIA GeForce GTX 1080 Direct3D11 vs_5_0 ps_5_0, D3D11)",
	"ANGLE (NVIDIA, NVIDIA GeForce RTX 4060 Direct3D11 vs_5_0 ps_5_0, D3D11)",
	"ANGLE (NVIDIA, NVIDIA GeForce RTX 3050 Ti Laptop GPU Direct3D11 vs_5_0 ps_5_0, D3D11)",
	"ANGLE (Intel, Intel(R) UHD Graphics (0x0000A720) Direct3D11 vs_5_0 ps_5_0, D3D11)",
	"ANGLE (NVIDIA GeForce GTX 1050 Direct3D11 vs_5_0 ps_5_0)",
	"ANGLE (Intel, Intel(R) UHD Graphics 770 (0x00004680) Direct3D11 vs_5_0 ps_5_0, D3D11)",
	"llvmpipe",
	"ANGLE (Intel, Intel(R) UHD Graphics 730 (0x00004682) Direct3D11 vs_5_0 ps_5_0, D3D11)",
	"Intel HD Graphics 5000 OpenGL Engine",
	"ANGLE (Qualcomm, Adreno (TM) 640, OpenGL ES 3.2)",
	"ANGLE (NVIDIA, NVIDIA GeForce GTX 960 (0x00001401) Direct3D11 vs_5_0 ps_5_0, D3D11)",
	"ANGLE (NVIDIA, NVIDIA GeForce GTX 1050 with Max-Q Design Direct3D11 vs_5_0 ps_5_0, D3D11-31.0.15.4633)",
	"ANGLE (Intel, Intel(R) UHD Graphics 630 Direct3D11 vs_5_0 ps_5_0, D3D11)",
	"ANGLE (NVIDIA, NVIDIA GeForce GTX 1070 (0x00001B81) Direct3D11 vs_5_0 ps_5_0, D3D11)",
	"ANGLE (NVIDIA, NVIDIA GeForce RTX 4060 Ti (0x00002805) Direct3D11 vs_5_0 ps_5_0, D3D11)",
	"ANGLE (NVIDIA, NVIDIA GeForce GTX 1660 Ti (0x00002182) Direct3D11 vs_5_0 ps_5_0, D3D11)",
	"ANGLE (Intel, Intel(R) HD Graphics 630 (0x00005912) Direct3D11 vs_5_0 ps_5_0, D3D11)",
	"ANGLE (NVIDIA, NVIDIA Quadro M5000 Direct3D11 vs_5_0 ps_5_0, D3D11-21.21.13.7651)",
	"ANGLE (Intel, Intel(R) UHD Graphics 630 (0x00003E91) Direct3D11 vs_5_0 ps_5_0, D3D11)",
	"ANGLE (NVIDIA, NVIDIA GeForce GTX 1060 Direct3D11 vs_5_0 ps_5_0, D3D11-27.21.14.5148)",
	"ANGLE (NVIDIA, NVIDIA GeForce RTX 3050 (0x00002582) Direct3D11 vs_5_0 ps_5_0, D3D11)",
	"ANGLE (AMD, AMD Radeon(TM) R7 Graphics Direct3D11 vs_5_0 ps_5_0, D3D11)",
	"ANGLE (Intel, Intel(R) UHD Graphics 730 Direct3D11 vs_5_0 ps_5_0, D3D11)",
	"ANGLE (AMD, AMD Radeon RX 580 2048SP (0x00006FDF) Direct3D11 vs_5_0 ps_5_0, D3D11)",
	"ANGLE (AMD, AMD Radeon RX 6600 Direct3D11 vs_5_0 ps_5_0, D3D11-31.0.22023.1014)",
	"ANGLE (NVIDIA, NVIDIA GeForce RTX 3060 Ti (0x00002489) Direct3D11 vs_5_0 ps_5_0, D3D11)",
	"ANGLE (NVIDIA, NVIDIA GeForce GTX 1650 (0x00001F91) Direct3D11 vs_5_0 ps_5_0, D3D11)",
	"ANGLE (Intel, Intel(R) UHD Graphics 770 Direct3D11 vs_5_0 ps_5_0, D3D11)",
	"ANGLE (NVIDIA, NVIDIA GeForce GTX 1050 Direct3D11 vs_5_0 ps_5_0, D3D11)",
	"ANGLE (NVIDIA, NVIDIA Quadro RTX 4000 (0x00001EB1) Direct3D11 vs_5_0 ps_5_0, D3D11)",
	"ANGLE (Intel, Mesa Intel(R) UHD Graphics 630 (CFL GT2), OpenGL 4.6)",
	"ANGLE (Intel, Mesa Intel(R) Graphics (RPL-S), OpenGL 4.6)",
	"ANGLE (NVIDIA, NVIDIA GeForce RTX 3050 Laptop GPU Direct3D11 vs_5_0 ps_5_0, D3D11)",
	"ANGLE (NVIDIA GeForce RTX 4070 Laptop GPU Direct3D11 vs_5_0 ps_5_0)",
	"ANGLE (AMD, AMD Radeon (TM) Graphics Direct3D11 vs_5_0 ps_5_0, D3D11)",
	"ANGLE (NVIDIA, NVIDIA GeForce RTX 3050 Ti Laptop GPU (0x000025A0) Direct3D11 vs_5_0 ps_5_0, D3D11)",
	"ANGLE (Intel, Intel(R) UHD Graphics 750 (0x00004C8A) Direct3D11 vs_5_0 ps_5_0, D3D11)",
	"ANGLE (NVIDIA, NVIDIA GeForce GTX 1660 SUPER Direct3D11 vs_5_0 ps_5_0, D3D11)",
}

var webglUnmaskedVendors = []string{
	"Google Inc. (NVIDIA)", "ARM", "Google Inc. (VMware)", "Google Inc. (0x00001D17)",
	"Google Inc. (AMD)", "Google Inc. (Glenfly Tech Co. Ltd)",
	"Google Inc. (AMD) #km6q0E5WHM", "Google Inc. (0x344C5250)", "Apple Inc.",
	"Google Inc. (AMD) #6cI1zDD8Cn", "Google Inc. (Intel)", "Google Inc. (ARM)",
	"Google Inc. (Apple)", "Google Inc. (ATI Technologies Inc.)", "Imagination Technologies",
	"Intel", "ATI Technologies Inc.", "Google Inc. (Unknown)", "Google Inc. (Intel Inc.)",
	"Google Inc.", "Mesa/X.org", "Mesa",
	"Google Inc. (Imagination Technologies)", "Qualcomm", "NVIDIA Corporation", "Intel Inc.",
	"Google Inc. (NVIDIA Corporation)", "Google Inc. (Google)", "Apple",
	"Intel Open Source Technology Center", "Google Inc. (Microsoft)",
	"Google Inc. (Qualcomm)",
}

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
