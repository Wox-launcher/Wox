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

Please note that deep linking in Wox is case-sensitive, so ensure that your commands and parameters are correctly formatted.

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
