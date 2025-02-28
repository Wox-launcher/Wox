# Deep Linking

Deep linking is a technique that allows an application to be launched and navigated to a specific location or state using a URL. This is particularly useful for complex
applications where you want to provide shortcuts to specific functionality or content.

In Wox, deep linking is supported through a specific URL format: `wox://command?param=value&param2=value2`. This URL format allows you to execute specific commands in Wox with
optional parameters.

## Supported Commands

Currently, Wox supports the following command for deep linking:

| Command  | Description                             | example URL                                                                                                                |
|----------|-----------------------------------------|----------------------------------------------------------------------------------------------------------------------------|
| `query`  | Execute a specific query in Wox         | `wox://query?q=<your query with url encoded>`  <a href="wox://query?q=search%20files" target="_blank">click me to try</a>. |
| `plugin` | Execute a specific plugin action in Wox | `wox://plugin/<plugin_id>?anyKey=anyValue`                                                                                 |
| `toggle` | Toggle Wox                             | `wox://toggle`                                                                                                             |

Please note that deep linking in Wox is case-sensitive, so ensure that your commands and parameters are correctly formatted.

## Plugin Deep Linking

Plugin developers can implement deep linking in their plugins to allow direct access to specific plugin functionality. This is done through the `wox://plugin/<plugin_id>?param=value` URL format.

### Implementing Deep Link Support in Your Plugin

To implement deep link support in your plugin:

1. First, make sure your plugin has the `deeplink` feature enabled in its metadata:

```json
{
  "features": [
    {
      "name": "deeplink"
    }
  ]
}
```

2. Register a deep link callback in your plugin's `Init` function:

```go
func (p *YourPlugin) Init(ctx context.Context, initParams plugin.InitParams) {
    // Register deep link callback
    initParams.API.OnDeepLink(ctx, func(arguments map[string]string) {
        // Handle deep link arguments
        if action, ok := arguments["action"]; ok {
            switch action {
            case "search":
                query := arguments["query"]
                // Perform search with the query
                // ...
            case "open":
                id := arguments["id"]
                // Open item with the specified ID
                // ...
            default:
                // Handle unknown action
            }
        }
    })
}
```

3. Your plugin can now respond to deep links in the format: `wox://plugin/your_plugin_id?action=search&query=example`

### Deep Link Callback

The deep link callback receives a map of arguments parsed from the URL query parameters. You can use these arguments to determine what action to take in your plugin.

### Best Practices

1. **Document Your Deep Links**: Make sure to document the deep link format and parameters that your plugin supports.
2. **Validate Input**: Always validate the input parameters to prevent unexpected behavior.
3. **Provide Feedback**: If a deep link is invalid or cannot be processed, provide appropriate feedback to the user.
4. **URL Encoding**: Remember that values in deep links should be URL encoded to handle special characters properly.

## Platform-specific Notes

### Linux

On Linux, the deep link protocol handler is automatically registered when you first launch Wox. This creates a desktop entry file in `~/.local/share/applications/wox-url-handler.desktop` and registers it as the handler for the `wox://` protocol.

If you need to manually register the protocol handler, you can run the following commands:

```bash
# Create the desktop entry file
cat > ~/.local/share/applications/wox-url-handler.desktop << EOF
[Desktop Entry]
Type=Application
Name=Wox
Exec=/path/to/your/wox %u
MimeType=x-scheme-handler/wox;
Terminal=false
NoDisplay=true
EOF

# Register the protocol handler
xdg-mime default wox-url-handler.desktop x-scheme-handler/wox

# Update the desktop database
update-desktop-database ~/.local/share/applications
```

Replace `/path/to/your/wox` with the actual path to your Wox executable.
