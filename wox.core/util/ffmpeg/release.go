package ffmpeg

// Version pins the upstream release and its executable digests independently of PATH tools.
const Version = "b6.1.1"

const releaseURL = "https://github.com/eugeneware/ffmpeg-static/releases/download/" + Version

type releaseAsset struct {
	name          string
	size          int64
	sha256        string
	licenseSHA256 string
	readmeSHA256  string
}

// releaseAssets records publisher SHA-256 digests from the fixed GitHub release.
var releaseAssets = map[string]releaseAsset{
	"darwin-amd64":  {name: "darwin-x64", size: 25296431, sha256: "929b375c1182d956c51f7ac25e0b2b0411fb01f6f407aa15c9758efeb4242106", licenseSHA256: "2e1d16c72fd74e12063776371da757322f8b77589386532f4fd8634bde7de1af", readmeSHA256: "e88a0325f8e5b75210355e37341824f074d3cd82def2125be54c914b62848a36"},
	"darwin-arm64":  {name: "darwin-arm64", size: 19246198, sha256: "8923876afa8db5585022d7860ec7e589af192f441c56793971276d450ed3bbfa", licenseSHA256: "cb48bf09a11f5fb576cddb0431c8f5ed0a60157a9ec942adffc13907cbe083f2", readmeSHA256: "05ba4b92c96605434b1aaae3eedf5a2c280c9607bf78ffca9a5b536d9af2dc6a"},
	"windows-amd64": {name: "win32-x64", size: 29581307, sha256: "8883a3dffbd0a16cf4ef95206ea05283f78908dbfb118f73c83f4951dcc06d77", licenseSHA256: "8ceb4b9ee5adedde47b31e975c1d90c73ad27b6b165a1dcd80c7c545eb65b903", readmeSHA256: "a636a7183c58006351acbaf35303c0ed85c6e1320fd4e80de453ba6157de6311"},
	"linux-amd64":   {name: "linux-x64", size: 29354986, sha256: "bfe8a8fc511530457b528c48d77b5737527b504a3797a9bc4866aeca69c2dffa", licenseSHA256: "8ceb4b9ee5adedde47b31e975c1d90c73ad27b6b165a1dcd80c7c545eb65b903", readmeSHA256: "72f4b1b06d419d22ace6e7cc75f06826f90737345aa0b1736158929f4aacc537"},
	"linux-arm64":   {name: "linux-arm64", size: 25568691, sha256: "754a678672298bc68156adff58aa7385a592c2b30b1d0ae8750c45c915c4bac0", licenseSHA256: "8ceb4b9ee5adedde47b31e975c1d90c73ad27b6b165a1dcd80c7c545eb65b903", readmeSHA256: "d6777d2fd276b23f0ac6666fa619e88ffe4826521881c7ff83836e30cb4acec2"},
}
