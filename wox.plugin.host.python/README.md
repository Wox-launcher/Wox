# Wox.Plugin.Host.Python

Python host for Wox plugins.

`result_actions.py` owns callbacks by session, query, result and action ID. Query,
MRU restore, incremental results and form updates share that store. Core releases
query buckets with its result cache; late responses cannot repopulate released
buckets. Plugin unload closes the API's original store. See
`wox.core/plugin/host/README.md` for the internal retention protocol.

Run `uv run python -m unittest discover -s tests` to verify the host.

Package plugins load under a private module namespace derived from a lossless
encoding of the plugin ID and a load generation. Entry modules and package
initializers retain their file locations, so `__file__` can locate resources.

Relative imports and local absolute import statements (such as `import helper`
and `from src.helper import value`) resolve within the owning plugin, including
imports made later in callbacks. Plugin directories stay off the shared
`sys.path`, and local modules are removed when their plugin unloads. External
dependencies retain Python's shared import behavior. For dynamic local imports,
use `importlib.import_module(".helper", __package__)`.
