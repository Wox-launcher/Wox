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
