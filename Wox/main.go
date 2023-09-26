package main

import webview "github.com/qianlifeng/webview_go"

func main() {
	w := webview.New(true)
	defer w.Destroy()
	w.SetSize(800, 400, webview.HintFixed)
	w.SetTitle("Hello")
	w.SetHtml(`<!doctype html>
		<html>
			<body>111hello22333e2</body>
			<script>
				window.onload = function() {
					document.body.innerText = ` + "`hello, ${navigator.userAgent}`" + `;
					noop().then(function(res) {
						console.log('noop res', res);
						add(1, 2).then(function(res) {
							console.log('add res', res);
							quit();
						});
					});
				};
			</script>
		</html>
	)`)
	w.Run()
}
