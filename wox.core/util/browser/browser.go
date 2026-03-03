package browser

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"wox/common"
	"wox/util"
	"wox/util/shell"
)

const (
	BrowserIDChrome   = "chrome"
	BrowserIDEdge     = "edge"
	BrowserIDFirefox  = "firefox"
	BrowserIDBrave    = "brave"
	BrowserIDOpera    = "opera"
	BrowserIDChromium = "chromium"
	BrowserIDSafari   = "safari"
)

type BrowserOption struct {
	ID    string
	Label string
	Icon  common.WoxImage
}

var SupportedBrowsers = []BrowserOption{
	{ID: BrowserIDChrome, Label: "i18n:plugin_websearch_browser_google_chrome", Icon: common.NewWoxImageSvg(`<svg xmlns="http://www.w3.org/2000/svg" width="256" height="256" viewBox="0 0 256 256"><path fill="#fff" d="M128.003 199.216c39.335 0 71.221-31.888 71.221-71.223S167.338 56.77 128.003 56.77S56.78 88.658 56.78 127.993s31.887 71.223 71.222 71.223"/><path fill="#229342" d="M35.89 92.997Q27.92 79.192 17.154 64.02a127.98 127.98 0 0 0 110.857 191.981q17.671-24.785 23.996-35.74q12.148-21.042 31.423-60.251v-.015a63.993 63.993 0 0 1-110.857.017Q46.395 111.19 35.89 92.998"/><path fill="#fbc116" d="M128.008 255.996A127.97 127.97 0 0 0 256 127.997A128 128 0 0 0 238.837 64q-36.372-3.585-53.686-3.585q-19.632 0-57.152 3.585l-.014.01a63.99 63.99 0 0 1 55.444 31.987a63.99 63.99 0 0 1-.001 64.01z"/><path fill="#1a73e8" d="M128.003 178.677c27.984 0 50.669-22.685 50.669-50.67s-22.685-50.67-50.67-50.67c-27.983 0-50.669 22.686-50.669 50.67s22.686 50.67 50.67 50.67"/><path fill="#e33b2e" d="M128.003 64.004H238.84a127.973 127.973 0 0 0-221.685.015l55.419 95.99l.015.008a63.993 63.993 0 0 1 55.415-96.014z"/></svg>`)},
	{ID: BrowserIDEdge, Label: "i18n:plugin_websearch_browser_microsoft_edge", Icon: common.NewWoxImageSvg(`<svg xmlns="http://www.w3.org/2000/svg" width="256" height="256" viewBox="0 0 256 256"><defs><radialGradient id="SVGbwvgocxp" cx="161.83" cy="788.401" r="95.38" gradientTransform="matrix(.9999 0 0 .9498 -4.622 -570.387)" gradientUnits="userSpaceOnUse"><stop offset=".72" stop-opacity="0"/><stop offset=".95" stop-opacity="0.53"/><stop offset="1"/></radialGradient><radialGradient id="SVGLz3cAcmq" cx="-773.636" cy="746.715" r="143.24" gradientTransform="matrix(.15 -.9898 .8 .12 -410.718 -656.341)" gradientUnits="userSpaceOnUse"><stop offset=".76" stop-opacity="0"/><stop offset=".95" stop-opacity="0.5"/><stop offset="1"/></radialGradient><radialGradient id="SVGrO7nVtsm" cx="230.593" cy="-106.038" r="202.43" gradientTransform="matrix(-.04 .9998 -2.1299 -.07998 -190.775 -191.635)" gradientUnits="userSpaceOnUse"><stop offset="0" stop-color="#35c1f1"/><stop offset=".11" stop-color="#34c1ed"/><stop offset=".23" stop-color="#2fc2df"/><stop offset=".31" stop-color="#2bc3d2"/><stop offset=".67" stop-color="#36c752"/></radialGradient><radialGradient id="SVGGx8U7cIp" cx="536.357" cy="-117.703" r="97.34" gradientTransform="matrix(.28 .9598 -.78 .23 -1.928 -410.318)" gradientUnits="userSpaceOnUse"><stop offset="0" stop-color="#66eb6e"/><stop offset="1" stop-color="#66eb6e" stop-opacity="0"/></radialGradient><linearGradient id="SVGaEjy6oMd" x1="63.334" x2="241.617" y1="757.83" y2="757.83" gradientTransform="translate(-4.63 -580.81)" gradientUnits="userSpaceOnUse"><stop offset="0" stop-color="#0c59a4"/><stop offset="1" stop-color="#114a8b"/></linearGradient><linearGradient id="SVGwaOKscCn" x1="157.401" x2="46.028" y1="680.556" y2="801.868" gradientTransform="translate(-4.63 -580.81)" gradientUnits="userSpaceOnUse"><stop offset="0" stop-color="#1b9de2"/><stop offset=".16" stop-color="#1595df"/><stop offset=".67" stop-color="#0680d7"/><stop offset="1" stop-color="#0078d4"/></linearGradient></defs><path fill="url(#SVGaEjy6oMd)" d="M231 190.5c-3.4 1.8-6.9 3.4-10.5 4.7c-11.5 4.3-23.6 6.5-35.9 6.5c-47.3 0-88.5-32.5-88.5-74.3c.1-11.4 6.4-21.9 16.4-27.3c-42.8 1.8-53.8 46.4-53.8 72.5c0 73.9 68.1 81.4 82.8 81.4c7.9 0 19.8-2.3 27-4.6l1.3-.4c27.6-9.5 51-28.1 66.6-52.8c1.2-1.9.6-4.3-1.2-5.5c-1.3-.8-2.9-.9-4.2-.2"/><path fill="url(#SVGbwvgocxp)" d="M231 190.5c-3.4 1.8-6.9 3.4-10.5 4.7c-11.5 4.3-23.6 6.5-35.9 6.5c-47.3 0-88.5-32.5-88.5-74.3c.1-11.4 6.4-21.9 16.4-27.3c-42.8 1.8-53.8 46.4-53.8 72.5c0 73.9 68.1 81.4 82.8 81.4c7.9 0 19.8-2.3 27-4.6l1.3-.4c27.6-9.5 51-28.1 66.6-52.8c1.2-1.9.6-4.3-1.2-5.5c-1.3-.8-2.9-.9-4.2-.2" opacity="0.35"/><path fill="url(#SVGwaOKscCn)" d="M105.7 241.4c-8.9-5.5-16.6-12.8-22.7-21.3c-26.3-36-18.4-86.5 17.6-112.8c3.8-2.7 7.7-5.2 11.9-7.2c3.1-1.5 8.4-4.1 15.5-4c10.1.1 19.6 4.9 25.7 13c4 5.4 6.3 11.9 6.4 18.7c0-.2 24.5-79.6-80-79.6c-43.9 0-80 41.7-80 78.2c-.2 19.3 4 38.5 12.1 56c27.6 58.8 94.8 87.6 156.4 67.1c-21.1 6.6-44.1 3.7-62.9-8.1"/><path fill="url(#SVGLz3cAcmq)" d="M105.7 241.4c-8.9-5.5-16.6-12.8-22.7-21.3c-26.3-36-18.4-86.5 17.6-112.8c3.8-2.7 7.7-5.2 11.9-7.2c3.1-1.5 8.4-4.1 15.5-4c10.1.1 19.6 4.9 25.7 13c4 5.4 6.3 11.9 6.4 18.7c0-.2 24.5-79.6-80-79.6c-43.9 0-80 41.7-80 78.2c-.2 19.3 4 38.5 12.1 56c27.6 58.8 94.8 87.6 156.4 67.1c-21.1 6.6-44.1 3.7-62.9-8.1" opacity="0.41"/><path fill="url(#SVGrO7nVtsm)" d="M152.3 148.9c-.8 1-3.3 2.5-3.3 5.7c0 2.6 1.7 5.1 4.7 7.2c14.4 10 41.5 8.7 41.6 8.7c10.7 0 21.1-2.9 30.3-8.3c18.8-11 30.4-31.1 30.4-52.9c.3-22.4-8-37.3-11.3-43.9C223.5 23.9 177.7 0 128 0C58 0 1 56.2 0 126.2c.5-36.5 36.8-66 80-66c3.5 0 23.5.3 42 10.1c16.3 8.6 24.9 18.9 30.8 29.2c6.2 10.7 7.3 24.1 7.3 29.5c0 5.3-2.7 13.3-7.8 19.9"/><path fill="url(#SVGGx8U7cIp)" d="M152.3 148.9c-.8 1-3.3 2.5-3.3 5.7c0 2.6 1.7 5.1 4.7 7.2c14.4 10 41.5 8.7 41.6 8.7c10.7 0 21.1-2.9 30.3-8.3c18.8-11 30.4-31.1 30.4-52.9c.3-22.4-8-37.3-11.3-43.9C223.5 23.9 177.7 0 128 0C58 0 1 56.2 0 126.2c.5-36.5 36.8-66 80-66c3.5 0 23.5.3 42 10.1c16.3 8.6 24.9 18.9 30.8 29.2c6.2 10.7 7.3 24.1 7.3 29.5c0 5.3-2.7 13.3-7.8 19.9"/></svg>`)},
	{ID: BrowserIDFirefox, Label: "i18n:plugin_websearch_browser_mozilla_firefox", Icon: common.NewWoxImageSvg(`<svg xmlns="http://www.w3.org/2000/svg" width="256" height="265" viewBox="0 0 256 265"><defs><radialGradient id="SVGAPIKG7dM" cx="-7907.187" cy="-8515.121" r="80.797" gradientTransform="translate(26367.938 28186.305)scale(3.3067)" gradientUnits="userSpaceOnUse"><stop offset=".129" stop-color="#ffbd4f"/><stop offset=".186" stop-color="#ffac31"/><stop offset=".247" stop-color="#ff9d17"/><stop offset=".283" stop-color="#ff980e"/><stop offset=".403" stop-color="#ff563b"/><stop offset=".467" stop-color="#ff3750"/><stop offset=".71" stop-color="#f5156c"/><stop offset=".782" stop-color="#eb0878"/><stop offset=".86" stop-color="#e50080"/></radialGradient><radialGradient id="SVG2UUAoeGa" cx="-7936.711" cy="-8482.089" r="80.797" gradientTransform="translate(26367.938 28186.305)scale(3.3067)" gradientUnits="userSpaceOnUse"><stop offset=".3" stop-color="#960e18"/><stop offset=".351" stop-color="#b11927" stop-opacity="0.74"/><stop offset=".435" stop-color="#db293d" stop-opacity="0.343"/><stop offset=".497" stop-color="#f5334b" stop-opacity="0.094"/><stop offset=".53" stop-color="#ff3750" stop-opacity="0"/></radialGradient><radialGradient id="SVGe2bLKcug" cx="-7926.97" cy="-8533.457" r="58.534" gradientTransform="translate(26367.938 28186.305)scale(3.3067)" gradientUnits="userSpaceOnUse"><stop offset=".132" stop-color="#fff44f"/><stop offset=".252" stop-color="#ffdc3e"/><stop offset=".506" stop-color="#ff9d12"/><stop offset=".526" stop-color="#ff980e"/></radialGradient><radialGradient id="SVGGwwdhbnj" cx="-7945.648" cy="-8460.984" r="38.471" gradientTransform="translate(26367.938 28186.305)scale(3.3067)" gradientUnits="userSpaceOnUse"><stop offset=".353" stop-color="#3a8ee6"/><stop offset=".472" stop-color="#5c79f0"/><stop offset=".669" stop-color="#9059ff"/><stop offset="1" stop-color="#c139e6"/></radialGradient><radialGradient id="SVGnDihHchs" cx="-7935.62" cy="-8491.546" r="20.397" gradientTransform="matrix(3.21411 -.77707 .90934 3.76302 33365.914 25904.014)" gradientUnits="userSpaceOnUse"><stop offset=".206" stop-color="#9059ff" stop-opacity="0"/><stop offset=".278" stop-color="#8c4ff3" stop-opacity="0.064"/><stop offset=".747" stop-color="#7716a8" stop-opacity="0.45"/><stop offset=".975" stop-color="#6e008b" stop-opacity="0.6"/></radialGradient><radialGradient id="SVGYeK40d5n" cx="-7937.731" cy="-8518.427" r="27.676" gradientTransform="translate(26367.938 28186.305)scale(3.3067)" gradientUnits="userSpaceOnUse"><stop offset="0" stop-color="#ffe226"/><stop offset=".121" stop-color="#ffdb27"/><stop offset=".295" stop-color="#ffc82a"/><stop offset=".502" stop-color="#ffa930"/><stop offset=".732" stop-color="#ff7e37"/><stop offset=".792" stop-color="#ff7139"/></radialGradient><radialGradient id="SVGts592dPQ" cx="-7915.977" cy="-8535.981" r="118.081" gradientTransform="translate(26367.938 28186.305)scale(3.3067)" gradientUnits="userSpaceOnUse"><stop offset=".113" stop-color="#fff44f"/><stop offset=".456" stop-color="#ff980e"/><stop offset=".622" stop-color="#ff5634"/><stop offset=".716" stop-color="#ff3647"/><stop offset=".904" stop-color="#e31587"/></radialGradient><radialGradient id="SVG0FLrfcRz" cx="-7927.165" cy="-8522.859" r="86.499" gradientTransform="matrix(.3472 3.29017 -2.15928 .22816 -15491.597 28008.376)" gradientUnits="userSpaceOnUse"><stop offset="0" stop-color="#fff44f"/><stop offset=".06" stop-color="#ffe847"/><stop offset=".168" stop-color="#ffc830"/><stop offset=".304" stop-color="#ff980e"/><stop offset=".356" stop-color="#ff8b16"/><stop offset=".455" stop-color="#ff672a"/><stop offset=".57" stop-color="#ff3647"/><stop offset=".737" stop-color="#e31587"/></radialGradient><radialGradient id="SVGtj6L8cbn" cx="-7938.383" cy="-8508.176" r="73.72" gradientTransform="translate(26367.938 28186.305)scale(3.3067)" gradientUnits="userSpaceOnUse"><stop offset=".137" stop-color="#fff44f"/><stop offset=".48" stop-color="#ff980e"/><stop offset=".592" stop-color="#ff5634"/><stop offset=".655" stop-color="#ff3647"/><stop offset=".904" stop-color="#e31587"/></radialGradient><radialGradient id="SVGV9Ps4bXg" cx="-7918.923" cy="-8503.861" r="80.686" gradientTransform="translate(26367.938 28186.305)scale(3.3067)" gradientUnits="userSpaceOnUse"><stop offset=".094" stop-color="#fff44f"/><stop offset=".231" stop-color="#ffe141"/><stop offset=".509" stop-color="#ffaf1e"/><stop offset=".626" stop-color="#ff980e"/></radialGradient><linearGradient id="SVGlkpZIeqR" x1="70.786" x2="6.447" y1="12.393" y2="74.468" gradientTransform="translate(-2.999 -.01)scale(3.3067)" gradientUnits="userSpaceOnUse"><stop offset=".048" stop-color="#fff44f"/><stop offset=".111" stop-color="#ffe847"/><stop offset=".225" stop-color="#ffc830"/><stop offset=".368" stop-color="#ff980e"/><stop offset=".401" stop-color="#ff8b16"/><stop offset=".462" stop-color="#ff672a"/><stop offset=".534" stop-color="#ff3647"/><stop offset=".705" stop-color="#e31587"/></linearGradient><linearGradient id="SVGl1fRmclZ" x1="70.013" x2="15.267" y1="12.061" y2="66.806" gradientTransform="translate(-2.999 -.01)scale(3.3067)" gradientUnits="userSpaceOnUse"><stop offset=".167" stop-color="#fff44f" stop-opacity="0.8"/><stop offset=".266" stop-color="#fff44f" stop-opacity="0.634"/><stop offset=".489" stop-color="#fff44f" stop-opacity="0.217"/><stop offset=".6" stop-color="#fff44f" stop-opacity="0"/></linearGradient></defs><path fill="url(#SVGlkpZIeqR)" d="M248.033 88.713c-5.569-13.399-16.864-27.866-25.71-32.439a133.2 133.2 0 0 1 12.979 38.9l.023.215c-14.49-36.126-39.062-50.692-59.13-82.41a155 155 0 0 1-3.019-4.907a41 41 0 0 1-1.412-2.645a23.3 23.3 0 0 1-1.912-5.076a.33.33 0 0 0-.291-.331a.5.5 0 0 0-.241 0c-.016 0-.043.03-.063.037s-.063.036-.092.049l.049-.086c-32.19 18.849-43.113 53.741-44.118 71.194a64.1 64.1 0 0 0-35.269 13.593a38 38 0 0 0-3.307-2.506a59.4 59.4 0 0 1-.36-31.324a94.9 94.9 0 0 0-30.848 23.841h-.06c-5.079-6.438-4.722-27.667-4.431-32.102a23 23 0 0 0-4.279 2.272a93.4 93.4 0 0 0-12.526 10.73a112 112 0 0 0-11.98 14.375v.019v-.023A108.3 108.3 0 0 0 4.841 108.92l-.171.846a204 204 0 0 0-1.26 8.003c0 .096-.02.185-.03.281a122 122 0 0 0-2.08 17.667v.662c.086 98.661 106.944 160.23 192.344 110.825a128.17 128.17 0 0 0 62.12-89.153c.215-1.653.39-3.29.582-4.96a131.8 131.8 0 0 0-8.313-64.378M100.322 189.031c.599.288 1.161.599 1.776.873l.089.057a69 69 0 0 1-1.865-.93m135.013-93.612v-.123l.023.136z"/><path fill="url(#SVGAPIKG7dM)" d="M248.033 88.713c-5.569-13.399-16.864-27.866-25.71-32.439a133.2 133.2 0 0 1 12.979 38.9v.122l.023.136a116.07 116.07 0 0 1-3.988 86.497c-14.688 31.516-50.242 63.819-105.894 62.248c-60.132-1.703-113.089-46.323-122.989-104.766c-1.802-9.216 0-13.888.906-21.378a95.4 95.4 0 0 0-2.06 17.684v.662c.086 98.661 106.944 160.23 192.344 110.825a128.17 128.17 0 0 0 62.12-89.153c.215-1.653.39-3.29.582-4.96a131.8 131.8 0 0 0-8.313-64.378"/><path fill="url(#SVG2UUAoeGa)" d="M248.033 88.713c-5.569-13.399-16.864-27.866-25.71-32.439a133.2 133.2 0 0 1 12.979 38.9v.122l.023.136a116.07 116.07 0 0 1-3.988 86.497c-14.688 31.516-50.242 63.819-105.894 62.248c-60.132-1.703-113.089-46.323-122.989-104.766c-1.802-9.216 0-13.888.906-21.378a95.4 95.4 0 0 0-2.06 17.684v.662c.086 98.661 106.944 160.23 192.344 110.825a128.17 128.17 0 0 0 62.12-89.153c.215-1.653.39-3.29.582-4.96a131.8 131.8 0 0 0-8.313-64.378"/><path fill="url(#SVGe2bLKcug)" d="M185.754 103.778c.278.195.536.39.797.585a69.8 69.8 0 0 0-11.904-15.525C134.815 48.999 164.208 2.457 169.165.093l.049-.073c-32.19 18.849-43.113 53.741-44.118 71.194c1.495-.103 2.976-.229 4.504-.229a64.68 64.68 0 0 1 56.154 32.793"/><path fill="url(#SVGGwwdhbnj)" d="M129.683 111.734c-.212 3.188-11.475 14.182-15.413 14.182c-36.443 0-42.359 22.046-42.359 22.046c1.614 18.564 14.55 33.854 30.187 41.942c.714.371 1.439.705 2.163 1.032a71 71 0 0 0 3.763 1.541a57 57 0 0 0 16.675 3.217c63.876 2.996 76.25-76.384 30.154-99.419a44.24 44.24 0 0 1 30.901 7.503A64.68 64.68 0 0 0 129.6 70.985c-1.521 0-3.009.126-4.504.229a64.1 64.1 0 0 0-35.269 13.593c1.954 1.654 4.16 3.863 8.806 8.442c8.696 8.568 31 17.443 31.05 18.485"/><path fill="url(#SVGnDihHchs)" d="M129.683 111.734c-.212 3.188-11.475 14.182-15.413 14.182c-36.443 0-42.359 22.046-42.359 22.046c1.614 18.564 14.55 33.854 30.187 41.942c.714.371 1.439.705 2.163 1.032a71 71 0 0 0 3.763 1.541a57 57 0 0 0 16.675 3.217c63.876 2.996 76.25-76.384 30.154-99.419a44.24 44.24 0 0 1 30.901 7.503A64.68 64.68 0 0 0 129.6 70.985c-1.521 0-3.009.126-4.504.229a64.1 64.1 0 0 0-35.269 13.593c1.954 1.654 4.16 3.863 8.806 8.442c8.696 8.568 31 17.443 31.05 18.485"/><path fill="url(#SVGYeK40d5n)" d="M83.852 80.545a82 82 0 0 1 2.645 1.756a59.4 59.4 0 0 1-.36-31.324a94.9 94.9 0 0 0-30.849 23.841c.625-.017 19.216-.351 28.564 5.727"/><path fill="url(#SVGts592dPQ)" d="M2.471 139.411c9.89 58.443 62.857 103.063 122.989 104.766c55.652 1.574 91.205-30.732 105.894-62.248a116.07 116.07 0 0 0 3.988-86.497v-.122c0-.096-.02-.153 0-.123l.023.215c4.547 29.684-10.552 58.443-34.155 77.889l-.073.166c-45.989 37.455-90.002 22.598-98.91 16.533a65 65 0 0 1-1.865-.929c-26.814-12.817-37.891-37.247-35.517-58.198a32.91 32.91 0 0 1-30.359-19.096a48.34 48.34 0 0 1 47.117-1.891a63.82 63.82 0 0 0 48.119 1.891c-.049-1.042-22.353-9.92-31.05-18.484c-4.646-4.58-6.851-6.786-8.805-8.442a38 38 0 0 0-3.307-2.507c-.761-.519-1.617-1.081-2.645-1.756c-9.348-6.078-27.939-5.744-28.554-5.727h-.059c-5.079-6.438-4.722-27.667-4.431-32.101a23 23 0 0 0-4.279 2.271a93.4 93.4 0 0 0-12.526 10.73a112 112 0 0 0-12.03 14.342v.019v-.023A108.3 108.3 0 0 0 4.841 108.92c-.062.261-4.616 20.167-2.37 30.491"/><path fill="url(#SVG0FLrfcRz)" d="M174.654 88.838a69.8 69.8 0 0 1 11.904 15.542a27 27 0 0 1 1.921 1.574c29.056 26.784 13.832 64.646 12.698 67.341c23.603-19.447 38.688-48.205 34.155-77.89c-14.497-36.142-39.069-50.708-59.137-82.426a155 155 0 0 1-3.019-4.907a41 41 0 0 1-1.412-2.645a23.3 23.3 0 0 1-1.912-5.076a.33.33 0 0 0-.291-.331a.5.5 0 0 0-.241 0c-.016 0-.043.03-.063.037s-.063.036-.092.049c-4.957 2.351-34.35 48.893 5.489 88.732"/><path fill="url(#SVGtj6L8cbn)" d="M188.459 105.937a27 27 0 0 0-1.921-1.574c-.261-.195-.519-.39-.797-.585a44.24 44.24 0 0 0-30.901-7.503c46.095 23.048 33.728 102.415-30.154 99.419a57 57 0 0 1-16.675-3.217a67 67 0 0 1-3.763-1.541c-.725-.331-1.449-.661-2.163-1.032l.089.057c8.908 6.081 52.907 20.938 98.91-16.534l.073-.165c1.147-2.679 16.371-40.55-12.698-67.325"/><path fill="url(#SVGV9Ps4bXg)" d="M71.911 147.962s5.916-22.046 42.359-22.046c3.938 0 15.211-10.994 15.413-14.182a63.82 63.82 0 0 1-48.119-1.892a48.34 48.34 0 0 0-47.118 1.892a32.91 32.91 0 0 0 30.359 19.096c-2.374 20.955 8.703 45.385 35.517 58.198c.599.288 1.161.599 1.776.873c-15.65-8.085-28.573-23.375-30.187-41.939"/><path fill="url(#SVGl1fRmclZ)" d="M248.033 88.713c-5.569-13.399-16.864-27.866-25.71-32.439a133.2 133.2 0 0 1 12.979 38.9l.023.215c-14.49-36.126-39.062-50.692-59.13-82.41a155 155 0 0 1-3.019-4.907a41 41 0 0 1-1.412-2.645a23.3 23.3 0 0 1-1.912-5.076a.33.33 0 0 0-.291-.331a.5.5 0 0 0-.241 0c-.016 0-.043.03-.063.037s-.063.036-.092.049l.049-.086c-32.19 18.849-43.113 53.741-44.118 71.194c1.495-.103 2.976-.229 4.504-.229a64.68 64.68 0 0 1 56.154 32.793a44.24 44.24 0 0 0-30.901-7.503c46.096 23.048 33.729 102.415-30.154 99.419a57 57 0 0 1-16.675-3.217a67 67 0 0 1-3.763-1.541c-.724-.331-1.449-.661-2.163-1.032l.089.057a69 69 0 0 1-1.865-.93c.599.288 1.161.599 1.776.873c-15.65-8.088-28.573-23.378-30.187-41.942c0 0 5.916-22.046 42.359-22.046c3.938 0 15.211-10.994 15.413-14.182c-.05-1.042-22.354-9.92-31.05-18.485c-4.646-4.579-6.852-6.785-8.806-8.442a38 38 0 0 0-3.307-2.506a59.4 59.4 0 0 1-.36-31.324a94.9 94.9 0 0 0-30.848 23.841h-.06c-5.079-6.438-4.722-27.667-4.431-32.102a23 23 0 0 0-4.279 2.272a93.4 93.4 0 0 0-12.526 10.73a112 112 0 0 0-11.98 14.375v.019v-.023A108.3 108.3 0 0 0 4.841 108.92l-.171.846c-.242 1.128-1.323 6.855-1.479 8.085c0 .093 0-.096 0 0A149 149 0 0 0 1.3 135.717v.662c.086 98.661 106.944 160.23 192.344 110.825a128.17 128.17 0 0 0 62.12-89.153c.215-1.653.39-3.29.582-4.96a131.8 131.8 0 0 0-8.313-64.378m-12.715 6.583l.024.136z"/></svg>`)},
	{ID: BrowserIDBrave, Label: "i18n:plugin_websearch_browser_brave", Icon: common.NewWoxImageSvg(`<svg xmlns="http://www.w3.org/2000/svg" width="256" height="301" viewBox="0 0 256 301"><defs><linearGradient id="SVGghidueKd" x1="0%" x2="100.097%" y1="50.018%" y2="50.018%"><stop offset="0%" stop-color="#fff"/><stop offset="14.13%" stop-color="#fff" stop-opacity="0.958"/><stop offset="100%" stop-color="#fff" stop-opacity="0.7"/></linearGradient><linearGradient id="SVGuQiNMboD" x1="-.039%" x2="100%" y1="49.982%" y2="49.982%"><stop offset="0%" stop-color="#f1f1f2"/><stop offset="9.191%" stop-color="#e4e5e6"/><stop offset="23.57%" stop-color="#d9dadb"/><stop offset="43.8%" stop-color="#d2d4d5"/><stop offset="100%" stop-color="#d0d2d3"/></linearGradient></defs><path fill="#f15a22" d="M256 97.1L246.7 72l6.4-14.4c.8-1.9.4-4-1-5.5l-17.5-17.7c-7.7-7.7-19.1-10.4-29.4-6.8l-4.9 1.7l-26.8-29l-45.3-.3h-.3L82.3.4L55.6 29.6l-4.8-1.7c-10.4-3.7-21.9-1-29.6 6.9l-17.8 18c-1.2 1.2-1.5 2.9-.9 4.4l6.7 15L0 97.3L6 120l27.2 103.3c3.1 11.9 10.3 22.3 20.4 29.5c0 0 33 23.3 65.5 44.4c2.9 1.9 5.9 3.2 9.1 3.2s6.2-1.3 9.1-3.2c36.6-24 65.5-44.5 65.5-44.5c10-7.2 17.2-17.6 20.3-29.5l27-103.3z"/><path fill="url(#SVGghidueKd)" d="M34.5 227.7L0 99.5l10.1-25.1l-7-18.6l16.7-17c5.5-4.9 16.3-6.6 21.3-3.7l26.1 15l34 7.9l26.5-11l2.2 227.7c-.4 32.8 1.7 29.3-22.4 13.8L48 248.6c-6.4-6.1-11.3-13-13.5-20.9" opacity="0.15"/><path fill="url(#SVGuQiNMboD)" d="m202.2 252.246l-50.6 34.6c-14.1 7.7-20.9 15.3-22 11.6c-.9-2.9-.2-11.4-.5-24.6l-.6-222.7c.1-2.2 1.6-5.9 4.2-5.5l25.8 7.8l37.2-5.8l24.6-18.1c2.6-2 6.4-1.8 8.8.5l22 21c2 2.1 2.1 6.2.9 8.8l-6.1 11.3l10.1 26.1l-34.8 129.4c-5.4 16.1-13 20.3-19 25.6" opacity="0.4"/><path fill="#fff" d="M134 184.801c-1.2-.5-2.5-.9-2.9-.9h-3.2c-.4 0-1.7.4-2.9.9l-13 5.4c-1.2.5-3.2 1.4-4.4 2l-19.6 10.2c-1.2.6-1.3 1.7-.2 2.5l17.3 12.2c1.1.8 2.8 2.1 3.8 3l7.7 6.6c1 .9 2.6 2.3 3.6 3.2l7.4 6.6c1 .9 2.6.9 3.6 0l7.6-6.6c1-.9 2.6-2.3 3.6-3.2l7.7-6.7c1-.9 2.7-2.2 3.8-3l17.3-12.3c1.1-.8 1-1.9-.2-2.5l-19.6-10c-1.2-.6-3.2-1.5-4.4-2z"/><path fill="#fff" d="M227.813 101.557c.4-1.3.4-1.8.4-1.8c0-1.3-.1-3.5-.3-4.8l-1-2.9c-.6-1.2-1.6-3.1-2.4-4.2l-11.3-16.7c-.7-1.1-2-2.8-2.9-3.9l-14.6-18.3c-.8-1-1.6-1.9-1.7-1.8h-.2s-1.1.2-2.4.4l-22.3 4.4c-1.3.3-3.4.7-4.7.9l-.4.1c-1.3.2-3.4.1-4.7-.3l-18.7-6c-1.3-.4-3.4-1-4.6-1.3c0 0-3.8-.9-6.9-.8c-3.1 0-6.9.8-6.9.8c-1.3.3-3.4.9-4.6 1.3l-18.7 6c-1.3.4-3.4.5-4.7.3l-.4-.1c-1.3-.2-3.4-.7-4.7-.9l-22.5-4.2c-1.3-.3-2.4-.4-2.4-.4h-.2c-.1 0-.9.8-1.7 1.8l-14.6 18.3c-.8 1-2.1 2.8-2.9 3.9l-11.3 16.7c-.7 1.1-1.8 3-2.4 4.2l-1 2.9c-.2 1.3-.4 3.5-.3 4.8c0 0 0 .4.4 1.8c.7 2.4 2.4 4.6 2.4 4.6c.8 1 2.3 2.7 3.2 3.6l33.1 35.2c.9 1 1.2 2.8.7 4l-6.9 16.3c-.5 1.2-.6 3.2-.1 4.5l1.9 5.1c1.6 4.3 4.3 8.1 7.9 11l6.7 5.4c1 .8 2.8 1.1 4 .5l21.2-10.1c1.2-.6 3-1.8 4-2.7l15.2-13.7c2.2-2 2.3-5.4.3-7.6l-31.9-21.5c-1.1-.7-1.5-2.3-.9-3.5l14-26.4c.6-1.2.7-3.1.2-4.3l-1.7-3.9c-.5-1.2-2-2.6-3.2-3.1l-41.1-15.4c-1.2-.5-1.2-1 .1-1.1l26.5-2.5c1.3-.1 3.4.1 4.7.4l23.6 6.6c1.3.4 2.1 1.7 1.9 3l-8.2 44.9c-.2 1.3-.2 3.1.1 4.1s1.6 1.9 2.9 2.2l16.4 3.5c1.3.3 3.4.3 4.7 0l15.3-3.5c1.3-.3 2.6-1.3 2.9-2.2s.4-2.8.1-4.1l-8.1-44.9c-.2-1.3.6-2.7 1.9-3l23.6-6.6c1.3-.4 3.4-.5 4.7-.4l26.5 2.5c1.3.1 1.4.6.1 1.1l-41.1 15.6c-1.2.5-2.7 1.8-3.2 3.1l-1.7 3.9c-.5 1.2-.5 3.2.2 4.3l14.1 26.4c.6 1.2.2 2.7-.9 3.5l-31.9 21.6c-2.1 2.1-1.9 5.6.3 7.6l15.2 13.7c1 .9 2.8 2.1 4 2.6l21.3 10.1c1.2.6 3 .3 4-.5l6.7-5.5c3.6-2.9 6.3-6.7 7.8-11l1.9-5.1c.5-1.2.4-3.3-.1-4.5l-6.9-16.3c-.5-1.2-.2-3 .7-4l33.1-35.2c.9-1 2.3-2.6 3.2-3.6c-.2-.3 1.6-2.5 2.2-4.9"/></svg>`)},
	{ID: BrowserIDOpera, Label: "i18n:plugin_websearch_browser_opera", Icon: common.NewWoxImageSvg(`<svg xmlns="http://www.w3.org/2000/svg" width="128" height="128" viewBox="0 0 128 128"><defs><linearGradient id="SVGGUjzzcxp" x1="53.327" x2="53.327" y1="2.095" y2="126.143" gradientUnits="userSpaceOnUse"><stop offset="0" stop-color="#ff1b2d"/><stop offset=".614" stop-color="#ff1b2d"/><stop offset="1" stop-color="#a70014"/></linearGradient><linearGradient id="SVGnAMLXcyp" x1="85.463" x2="85.463" y1="9.408" y2="119.121" gradientUnits="userSpaceOnUse"><stop offset="0" stop-color="#9c0000"/><stop offset=".7" stop-color="#ff4b4b"/></linearGradient></defs><path fill="url(#SVGGUjzzcxp)" d="M63.996.008C28.652.008 0 28.66 0 64.008c0 34.32 27.02 62.332 60.949 63.922q1.517.072 3.047.074a63.77 63.77 0 0 0 42.652-16.285c-7.5 4.973-16.273 7.836-25.645 7.836c-15.242 0-28.891-7.562-38.07-19.484c-7.078-8.352-11.66-20.699-11.973-34.559V62.5c.313-13.859 4.895-26.207 11.973-34.559C52.113 16.016 65.762 8.457 81 8.457c9.375 0 18.148 2.863 25.652 7.84C95.383 6.219 80.531.07 64.238.008zm0 0"/><path fill="url(#SVGnAMLXcyp)" d="M42.934 27.945c5.871-6.934 13.457-11.117 21.742-11.117c18.633 0 33.734 21.125 33.734 47.18s-15.102 47.18-33.734 47.18c-8.285 0-15.871-4.18-21.742-11.113c9.18 11.926 22.828 19.484 38.07 19.484c9.375 0 18.145-2.863 25.645-7.836c13.102-11.719 21.348-28.754 21.348-47.715s-8.246-35.988-21.344-47.707c-7.5-4.977-16.273-7.84-25.648-7.84c-15.242 0-28.891 7.562-38.07 19.484"/></svg>`)},
	{ID: BrowserIDChromium, Label: "i18n:plugin_websearch_browser_chromium", Icon: common.NewWoxImageSvg(`<svg xmlns="http://www.w3.org/2000/svg" width="72" height="72" viewBox="0 0 72 72"><circle cx="36" cy="36" r="28" fill="#92d3f5"/><path fill="#92d3f5" fill-rule="evenodd" d="m34.312 27.158l.008.047a9 9 0 0 1 9.327 13.369L30.386 63.542C41.828 65.821 53.943 60.74 60.1 50.074c4.21-7.291 4.767-15.688 2.24-23.074H36q-.867.001-1.688.158" clip-rule="evenodd"/><path fill="#61b2e4" fill-rule="evenodd" d="M27 43.5L8.202 32.617C9.872 18.748 21.681 8 36 8c12.316 0 22.774 7.951 26.522 19H36a9 9 0 0 0-6.914 14.762z" clip-rule="evenodd"/><circle cx="36" cy="36" r="9" fill="#61b2e4"/><g fill="none" stroke="#000" stroke-width="2"><circle cx="36" cy="36" r="10"/><path stroke-linecap="round" d="m44.66 41l-11.5 19.919M11.081 33.16L31 44.66M36 26h23"/><circle cx="36" cy="36" r="28"/></g></svg>`)},
	{ID: BrowserIDSafari, Label: "i18n:plugin_websearch_browser_safari", Icon: common.NewWoxImageSvg(`<svg xmlns="http://www.w3.org/2000/svg" width="128" height="128" viewBox="0 0 128 128"><linearGradient id="SVGDKFgEpKF" x1="295.835" x2="295.835" y1="274.049" y2="272.933" gradientTransform="matrix(112 0 0 -112 -33069.5 30695)" gradientUnits="userSpaceOnUse"><stop offset="0" stop-color="#19d7ff"/><stop offset="1" stop-color="#1e64f0"/></linearGradient><circle cx="64" cy="64" r="62.5" fill="url(#SVGDKFgEpKF)"/><path fill="#fff" d="M63.5 7.6v9.2h1V7.6Zm-3.902.26l-.996.08l.4 5l.996-.08zm8.804 0l-.4 5l.996.08l.4-5zm-13.709.554l-.986.172l1.6 9.101l.986-.173zm18.614 0l-1.6 9.1l.986.174l1.6-9.102zM49.883 9.47l-.965.261l1.299 4.801l.965-.261Zm28.234 0l-1.299 4.8l.965.262l1.299-4.8zm-32.846 1.363l-.943.336l3.102 8.7l.941-.337zm37.458 0l-3.1 8.7l.941.335l3.102-8.699zm4.62 1.852l-2.2 4.601l.902.43l2.201-4.6zm-46.695.007l-.908.416l2.1 4.6l.908-.414zm-4.32 2.26l-.867.498l4.6 8l.867-.498zm55.332 0l-4.6 8l.868.498l4.6-8zm-59.559 2.56l-.816.577l2.9 4.101l.817-.578zm63.786 0l-2.9 4.1l.816.578l2.9-4.101zm-67.61 2.968l-.765.644l5.9 7l.764-.644zm71.434 0l-5.899 7l.764.644l5.9-7zm-75.168 3.263l-.697.717l3.6 3.5l.696-.717zm-3.33 3.574l-.639.768l7.1 5.9l.64-.77zm85.562 0l-7.101 5.899l.64.77l7.1-5.901zM18.184 31.19l-.569.823l4.201 2.9l.569-.824zm91.632 0l-4.2 2.899l.568.824l4.2-2.9zM15.55 35.367l-.498.867l8 4.6l.498-.867zm96.902 0l-8 4.6l.498.867l8-4.6zm-99.14 4.28l-.422.906l4.5 2.1l.422-.907zm101.378 0l-4.5 2.1l.422.905l4.5-2.1zM11.375 44.13l-.35.937l8.6 3.202l.35-.938zm105.25 0l-8.6 3.201l.35.938l8.6-3.202zM9.828 48.816l-.256.967l4.9 1.301l.257-.967zm108.344 0l-4.9 1.301l.255.967l4.9-1.3zM8.688 53.607l-.174.985l9.1 1.601l.173-.986zm110.624 0l-9.1 1.6l.175.986l9.1-1.601zM8.05 58.402l-.098.996l5 .5l.098-.996zm111.902 0l-5 .5l.098.996l5-.5zM7.801 63.4v1H17v-1zM111 63.4v1h9.2v-1zm-98.049 4.403l-5 .5l.098.994l5-.5zm102.098 0l-.098.994l5 .5l.098-.994zm-97.436 3.705l-9.1 1.6l.175.984l9.1-1.6zm92.774 0l-.174.984l9.1 1.6l.173-.985zm-95.914 5.11l-4.9 1.298l.255.967l4.9-1.299zm99.054 0l-.256.966l4.9 1.299l.257-.967zm-93.902 2.814l-8.6 3.199l.35.937l8.6-3.199zm88.75 0l-.35.937l8.6 3.2l.35-.938zm-90.986 5.615l-4.5 2.1l.422.906l4.5-2.1zm93.222 0l-.422.906l4.5 2.1l.422-.907zm-87.56 1.92l-8 4.6l.498.867l8-4.6zm81.898 0l-.498.867l8 4.6l.498-.868zm-83.133 5.822l-4.2 2.9l.568.823l4.2-2.9zm84.368 0l-.569.822l4.201 2.9l.569-.822zm-78.504.926l-7.1 5.9l.639.77l7.101-5.9zm72.64 0l-.64.77l7.101 5.9l.639-.77zm-66.902 5.863l-5.9 7l.765.645l5.899-7zm61.164 0l-.764.645l5.899 7l.765-.645zm5.967.164l-.697.717l3.6 3.5l.696-.717zm-60.48 4.606l-4.6 7.9l.863.504l4.6-7.9zm47.863 0l-.864.504l4.6 7.9l.863-.504zm-53.74 1.164l-2.901 4.1l.816.577l2.9-4.101zm59.617 0l-.817.576l2.9 4.101l.817-.578zm-46.38 2.32l-3.1 8.7l.942.335l3.1-8.699zm33.141 0l-.941.336l3.1 8.7l.943-.337zm-25.263 2.182l-1.6 9.1l.986.173l1.6-9.1zm17.386 0l-.986.174l1.6 9.1l.986-.175zm-30.742.066l-2.201 4.5l.898.44l2.202-4.5zm44.098 0l-.899.44l2.202 4.5l.898-.44Zm-22.549.82v9.2h1v-9.2zm-13.283 2.272l-1.301 4.9l.967.256l1.3-4.9zm27.566 0l-.967.256l1.301 4.9l.967-.256zm-18.781 1.687l-.4 5l.996.08l.4-5zm9.996 0l-.996.08l.4 5l.996-.08z" color="#000"/><path fill="#f00" d="m106.7 21l-48 37.7l5.2 5.2z"/><path fill="#d01414" d="m63.9 63.9l6 6L106.7 21z"/><path fill="#fff" d="m58.7 58.7l-37.7 48l42.9-42.8z"/><path fill="#acacac" d="m21 106.7l48.9-36.8l-6-6z"/></svg>`)},
}

func NormalizeBrowserID(browserID string) string {
	return strings.ToLower(strings.TrimSpace(browserID))
}

func GetInstalledBrowsers() []BrowserOption {
	var installed []BrowserOption
	for _, browser := range SupportedBrowsers {
		if IsInstalled(browser.ID) {
			installed = append(installed, browser)
		}
	}
	return installed
}

func IsInstalled(browserID string) bool {
	switch {
	case util.IsWindows():
		_, ok := resolveWindowsBrowserExecutable(browserID)
		return ok
	case util.IsMacOS():
		_, ok := resolveMacBrowserApp(browserID)
		return ok
	case util.IsLinux():
		_, ok := resolveLinuxBrowserCommand(browserID)
		return ok
	default:
		return false
	}
}

func OpenURL(url string, browserID string) error {
	switch NormalizeBrowserID(browserID) {
	case "":
		return shell.Open(url)
	}

	switch {
	case util.IsWindows():
		executable, ok := resolveWindowsBrowserExecutable(browserID)
		if !ok {
			return shell.Open(url)
		}
		_, err := shell.Run(executable, url)
		if err != nil {
			return openURLInSystemBrowserWithFallback(url, err)
		}
		return nil
	case util.IsMacOS():
		appPath, ok := resolveMacBrowserApp(browserID)
		if !ok {
			return shell.Open(url)
		}
		_, err := shell.Run("open", "-a", appPath, url)
		if err != nil {
			return openURLInSystemBrowserWithFallback(url, err)
		}
		return nil
	case util.IsLinux():
		command, ok := resolveLinuxBrowserCommand(browserID)
		if !ok {
			return shell.Open(url)
		}
		_, err := shell.Run(command, url)
		if err != nil {
			return openURLInSystemBrowserWithFallback(url, err)
		}
		return nil
	default:
		return shell.Open(url)
	}
}

func openURLInSystemBrowserWithFallback(url string, openErr error) error {
	fallbackErr := shell.Open(url)
	if fallbackErr != nil {
		return fmt.Errorf("failed to open url with configured browser: %w, fallback to system browser failed: %w", openErr, fallbackErr)
	}
	return nil
}

func resolveWindowsBrowserExecutable(browserID string) (string, bool) {
	for _, candidate := range getWindowsBrowserCandidateExecutables(NormalizeBrowserID(browserID)) {
		if util.IsFileExists(candidate) {
			return candidate, true
		}
	}
	return "", false
}

func getWindowsBrowserCandidateExecutables(browserID string) []string {
	var candidates []string

	addProgramFilesCandidate := func(paths ...string) {
		for _, base := range getWindowsProgramFilesDirs() {
			candidates = append(candidates, filepath.Join(append([]string{base}, paths...)...))
		}
	}

	addLocalAppDataCandidate := func(paths ...string) {
		localAppData := os.Getenv("LOCALAPPDATA")
		if localAppData == "" {
			return
		}
		candidates = append(candidates, filepath.Join(append([]string{localAppData}, paths...)...))
	}

	switch browserID {
	case BrowserIDChrome:
		addProgramFilesCandidate("Google", "Chrome", "Application", "chrome.exe")
		addLocalAppDataCandidate("Google", "Chrome", "Application", "chrome.exe")
	case BrowserIDEdge:
		addProgramFilesCandidate("Microsoft", "Edge", "Application", "msedge.exe")
		addLocalAppDataCandidate("Microsoft", "Edge", "Application", "msedge.exe")
	case BrowserIDFirefox:
		addProgramFilesCandidate("Mozilla Firefox", "firefox.exe")
		addLocalAppDataCandidate("Mozilla Firefox", "firefox.exe")
	case BrowserIDBrave:
		addProgramFilesCandidate("BraveSoftware", "Brave-Browser", "Application", "brave.exe")
		addLocalAppDataCandidate("BraveSoftware", "Brave-Browser", "Application", "brave.exe")
	case BrowserIDOpera:
		addProgramFilesCandidate("Opera", "launcher.exe")
		addLocalAppDataCandidate("Programs", "Opera", "opera.exe")
	case BrowserIDChromium:
		addProgramFilesCandidate("Chromium", "Application", "chrome.exe")
		addLocalAppDataCandidate("Chromium", "Application", "chrome.exe")
	}

	return uniqueNonEmptyStrings(candidates)
}

func getWindowsProgramFilesDirs() []string {
	return uniqueNonEmptyStrings([]string{
		os.Getenv("ProgramFiles"),
		os.Getenv("ProgramFiles(x86)"),
	})
}

func resolveMacBrowserApp(browserID string) (string, bool) {
	for _, candidate := range getMacBrowserAppCandidates(NormalizeBrowserID(browserID)) {
		if util.IsDirExists(candidate) {
			return candidate, true
		}
	}
	return "", false
}

func getMacBrowserAppCandidates(browserID string) []string {
	homeDir, _ := os.UserHomeDir()

	addAppCandidates := func(appName string) []string {
		candidates := []string{
			filepath.Join("/Applications", appName),
		}
		if homeDir != "" {
			candidates = append(candidates, filepath.Join(homeDir, "Applications", appName))
		}
		return candidates
	}

	switch browserID {
	case BrowserIDSafari:
		candidates := []string{"/System/Applications/Safari.app"}
		return append(candidates, addAppCandidates("Safari.app")...)
	case BrowserIDChrome:
		return addAppCandidates("Google Chrome.app")
	case BrowserIDEdge:
		return addAppCandidates("Microsoft Edge.app")
	case BrowserIDFirefox:
		return addAppCandidates("Firefox.app")
	case BrowserIDBrave:
		return addAppCandidates("Brave Browser.app")
	case BrowserIDOpera:
		return addAppCandidates("Opera.app")
	case BrowserIDChromium:
		return addAppCandidates("Chromium.app")
	default:
		return nil
	}
}

func resolveLinuxBrowserCommand(browserID string) (string, bool) {
	for _, command := range getLinuxBrowserCandidateCommands(NormalizeBrowserID(browserID)) {
		if executable, err := exec.LookPath(command); err == nil {
			return executable, true
		}
	}
	return "", false
}

func getLinuxBrowserCandidateCommands(browserID string) []string {
	switch browserID {
	case BrowserIDChrome:
		return []string{"google-chrome", "google-chrome-stable"}
	case BrowserIDEdge:
		return []string{"microsoft-edge", "microsoft-edge-stable"}
	case BrowserIDFirefox:
		return []string{"firefox"}
	case BrowserIDBrave:
		return []string{"brave-browser"}
	case BrowserIDOpera:
		return []string{"opera"}
	case BrowserIDChromium:
		return []string{"chromium", "chromium-browser"}
	default:
		return nil
	}
}

func uniqueNonEmptyStrings(values []string) []string {
	unique := make(map[string]struct{})
	var result []string

	for _, value := range values {
		normalized := strings.TrimSpace(value)
		if normalized == "" {
			continue
		}
		if _, ok := unique[normalized]; ok {
			continue
		}
		unique[normalized] = struct{}{}
		result = append(result, normalized)
	}

	return result
}
