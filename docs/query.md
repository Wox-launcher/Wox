# Query

Query is the primary way for users to interact with Wox. There are two types of queries: input queries and selection queries.

## Input Query

An input query is composed of a trigger keyword, a command, and a search term.

- **Trigger Keyword**: This is the keyword that triggers a specific plugin. Once a plugin is triggered, other plugins will not receive the query. If the trigger keyword is "*", it
  represents a global search, such as when searching for programs in the system. Typically, you don't need to add a trigger keyword like "app". Trigger keywords can be modified by
  the user, and multiple keywords can be specified.

- **Command**: This is optional and can also be multiple. It represents the actions that the plugin can perform.

- **Search**: This is the actual content of the query.

For example:

1. in the query `wpm install emoji`, "wpm" is the trigger keyword, "install" is the command, and "emoji" is the name of the plugin to be installed.
2. in the query `emoji smile`, "emoji" is the trigger keyword, and "smile" is the search term, no command is specified.
3. in the query `mail`, "mail" is the search term, no trigger keyword or command is specified.

## Selection Query

A selection query is when a user selects/drag a file or text as the query. This type of query allows users to perform actions on specific files or text without having to manually
input
the file path or text into the query box. Please refer to [Selection Query](selection_query.md) for details.