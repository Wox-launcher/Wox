# Filesystem path queries

This package provides filesystem path recognition, home and environment
expansion, and directory browsing separators. It has no plugin or UI dependencies.

`Parse` accepts absolute paths, home-relative paths, and Windows-style environment
variable paths. `IsBrowseQuery` recognizes existing paths and child-name prefixes
inside existing directories. Callers decide how those queries affect routing and
result presentation.
