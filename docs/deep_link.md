# Deep Linking

Deep linking is a technique that allows an application to be launched and navigated to a specific location or state using a URL. This is particularly useful for complex
applications where you want to provide shortcuts to specific functionality or content.

In Wox, deep linking is supported through a specific URL format: `wox://command?param=value&param2=value2`. This URL format allows you to execute specific commands in Wox with
optional parameters.

## Supported Commands

Currently, Wox supports the following command for deep linking:

### Query

The `query` command allows you to execute a specific query in Wox. The format for this command is `wox://query?q=your%20query`. The `%20` is a URL-encoded space character, allowing
you to include spaces in your query.

For example, if you want to execute the query "search files", you would use the URL <a href="wox://query?q=search%20files" target="_blank">wox://query?q=search%20files</a>.

Please note that deep linking in Wox is case-sensitive, so ensure that your commands and parameters are correctly formatted.