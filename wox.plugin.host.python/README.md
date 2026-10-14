# Wox.Plugin.Host.Python

Python host for Wox plugins.

Package plugins load under a private module namespace derived from a lossless
encoding of the plugin ID and a load generation. Entry modules and package
initializers retain their file locations, so `__file__` can locate resources.

Relative imports and local absolute import statements (such as `import helper`
and `from src.helper import value`) resolve within the owning plugin, including
imports made later in callbacks. Plugin directories stay off the shared
`sys.path`, and local modules are removed when their plugin unloads. External
dependencies retain Python's shared import behavior. For dynamic local imports,
use `importlib.import_module(".helper", __package__)`.
