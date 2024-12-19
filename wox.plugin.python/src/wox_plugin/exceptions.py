class WoxPluginError(Exception):
    """Base exception for Wox plugin errors"""

    pass


class InvalidQueryError(WoxPluginError):
    """Raised when query is invalid"""

    pass


class PluginInitError(WoxPluginError):
    """Raised when plugin initialization fails"""

    pass


class APIError(WoxPluginError):
    """Raised when API call fails"""

    pass
