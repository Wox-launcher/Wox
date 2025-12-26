"""
Wox Image Models

This module provides image models for Wox plugins. Images are used to display
icons, thumbnails, and other visual elements in search results and previews.

The WoxImage model supports multiple image types including absolute paths,
relative paths, base64 encoded images, SVG, Lottie animations, emoji, URLs,
theme icons, and system file icons.
"""

import json
from dataclasses import dataclass, field
from enum import Enum


class WoxImageType(str, Enum):
    """
    Enumeration of supported image types in Wox.

    Each type represents a different way to specify or encode an image:
    - ABSOLUTE: Absolute file system path to an image file
    - RELATIVE: Path relative to the plugin directory
    - BASE64: Base64 encoded image data (e.g., "iVBORw0KGgo...")
    - SVG: Inline SVG markup
    - LOTTIE: Lottie animation JSON data
    - EMOJI: Unicode emoji character
    - URL: HTTP/HTTPS URL to an image
    - THEME: Built-in theme icon name
    - FILE_ICON: System-associated file icon for a given file extension
    """

    ABSOLUTE = "absolute"
    """Absolute file system path (e.g., "C:\\Users\\...\\icon.png" or "/home/user/icon.png")"""

    RELATIVE = "relative"
    """Path relative to the plugin directory (e.g., "images/icon.png")"""

    BASE64 = "base64"
    """
    Base64 encoded image data with data URI prefix.

    The data should be a complete data URI including the media type,
    encoding specification, and base64 data.

    Format: "data:image/<type>;base64,<base64_data>"

    Examples:
        - PNG: "data:image/png;base64,iVBORw0KGgo..."
        - JPEG: "data:image/jpeg;base64,/9j/4AAQ..."
    """

    SVG = "svg"
    """Inline SVG markup (e.g., "<svg>...</svg>")"""

    LOTTIE = "lottie"
    """
    Lottie animation file format (JSON string).

    Lottie is a JSON-based animation file format that enables
    high-quality animations to be easily shared and integrated
    into any platform. Use this for animated icons and illustrations.

    Example: You can provide Lottie JSON data directly as a string.
    """

    EMOJI = "emoji"
    """Unicode emoji character (e.g., "🔍", "⭐", "🎯")"""

    URL = "url"
    """HTTP/HTTPS URL to an image (e.g., "https://example.com/icon.png")"""

    THEME = "theme"
    """
    Built-in theme icon name.

    Use this to reference theme-provided icons that adapt to the
    current Wox theme (light/dark mode).

    Common theme icons: "search", "settings", "folder", "file", etc.
    """

    FILE_ICON = "fileicon"
    """
    System-associated file icon for a given file absolute path.

    The OS will automatically provide the appropriate icon based on
    the file extension. This ensures consistent display with the
    system's file explorer.

    Example: If you provide "C:\\path\\to\\file.txt", Wox will display
    the system's text file icon. For "C:\\path\\to\\document.pdf",
    it will show the PDF icon.
    """


@dataclass
class WoxImage:
    """
    Image model for displaying icons, thumbnails, and visual elements in Wox.

    This model encapsulates both the type and data of an image. Use the factory
    classmethods (e.g., `new_base64()`, `new_emoji()`) to create instances
    with the appropriate type.

    Example usage:
        # Create from base64 data
        icon = WoxImage.new_base64("iVBORw0KGgo...")

        # Create from emoji
        icon = WoxImage.new_emoji("🔍")

        # Create from relative path
        icon = WoxImage.new_relative("images/icon.png")

        # Create from file extension (system icon)
        icon = WoxImage.new_absolute("/path/to/file.txt", type=WoxImageType.FILE_ICON)

    Attributes:
        image_type: The type/format of the image (defaults to ABSOLUTE)
        image_data: The image data or path (format depends on image_type)

    Serialization:
        All images can be serialized to/from JSON using to_json() and from_json().
        The JSON format uses camelCase naming (e.g., "ImageType", "ImageData")
        for compatibility with the Wox C# backend.
    """

    image_type: WoxImageType = field(default=WoxImageType.ABSOLUTE)
    """The type/format of the image"""

    image_data: str = field(default="")
    """The image data, path, or URL (format depends on image_type)"""

    def to_json(self) -> str:
        """
        Convert to JSON string with camelCase naming.

        The output uses camelCase property names ("ImageType", "ImageData")
        for compatibility with the Wox C# backend.

        Returns:
            JSON string representation of this image
        """
        return json.dumps(
            {
                "ImageData": self.image_data,
                "ImageType": self.image_type,
            }
        )

    @classmethod
    def from_json(cls, json_str: str) -> "WoxImage":
        """
        Create from JSON string with camelCase naming.

        Expects JSON with "ImageType" and "ImageData" properties.

        Args:
            json_str: JSON string containing image data

        Returns:
            A new WoxImage instance
        """
        data = json.loads(json_str)
        return cls.from_dict(data)

    @classmethod
    def from_dict(cls, data: dict) -> "WoxImage":
        """
        Create from dictionary with camelCase naming.

        Args:
            data: Dictionary with "ImageType" and "ImageData" keys

        Returns:
            A new WoxImage instance
        """
        if not data.get("ImageType"):
            data["ImageType"] = WoxImageType.ABSOLUTE

        return cls(
            image_type=WoxImageType(data.get("ImageType")),
            image_data=data.get("ImageData", ""),
        )

    def to_dict(self) -> dict:
        """
        Convert to dictionary with camelCase naming.

        Returns:
            Dictionary with "ImageType" and "ImageData" keys
        """
        return {
            "ImageData": self.image_data,
            "ImageType": self.image_type,
        }

    @classmethod
    def new_base64(cls, data: str) -> "WoxImage":
        """
        Create a base64 encoded image.

        Use this for embedding small images directly in your plugin.
        The data must be a complete data URI including the media type,
        encoding specification, and base64 data.

        Args:
            data: Complete data URI string (e.g., "data:image/png;base64,iVBORw0KGgo...")

        Returns:
            A new WoxImage instance with type BASE64

        Example:
            # Encode an image to base64 data URI and create WoxImage
            import base64
            with open("icon.png", "rb") as f:
                base64_data = base64.b64encode(f.read()).decode()
                data_uri = f"data:image/png;base64,{base64_data}"
            icon = WoxImage.new_base64(data_uri)
        """
        return cls(image_type=WoxImageType.BASE64, image_data=data)

    @classmethod
    def new_svg(cls, data: str) -> "WoxImage":
        """
        Create an inline SVG image.

        Use this for scalable vector graphics that can be embedded
        directly in your plugin.

        Args:
            data: SVG markup string (e.g., "<svg viewBox='0 0 24 24'>...</svg>")

        Returns:
            A new WoxImage instance with type SVG

        Example:
            icon = WoxImage.new_svg("<svg viewBox='0 0 24 24'><path d='M12 2L2 22h20L12 2z'/></svg>")
        """
        return cls(image_type=WoxImageType.SVG, image_data=data)

    @classmethod
    def new_lottie(cls, data: str) -> "WoxImage":
        """
        Create a Lottie animation image.

        Use this for animated icons and illustrations. Lottie is a
        JSON-based animation format.

        Args:
            data: Lottie animation JSON string

        Returns:
            A new WoxImage instance with type LOTTIE

        Example:
            animation = WoxImage.new_lottie('{"v":"5.5.7","fr":60,"ip":0,"op":60,"w":100,"h":100,...}')
        """
        return cls(image_type=WoxImageType.LOTTIE, image_data=data)

    @classmethod
    def new_emoji(cls, data: str) -> "WoxImage":
        """
        Create an emoji image.

        Use this for simple, recognizable icons using Unicode emoji.

        Args:
            data: Unicode emoji character(s) (e.g., "🔍", "⭐", "🎯", "🚀")

        Returns:
            A new WoxImage instance with type EMOJI

        Example:
            icon = WoxImage.new_emoji("🔍")
        """
        return cls(image_type=WoxImageType.EMOJI, image_data=data)

    @classmethod
    def new_url(cls, data: str) -> "WoxImage":
        """
        Create an image from a URL.

        Use this for loading images from the web. The URL must be
        accessible from the user's machine.

        Args:
            data: HTTP/HTTPS URL to an image

        Returns:
            A new WoxImage instance with type URL

        Example:
            icon = WoxImage.new_url("https://example.com/icon.png")
        """
        return cls(image_type=WoxImageType.URL, image_data=data)

    @classmethod
    def new_absolute(cls, data: str) -> "WoxImage":
        """
        Create an image from an absolute file system path.

        Use this for images stored in known locations on the file system.

        Args:
            data: Absolute path to an image file

        Returns:
            A new WoxImage instance with type ABSOLUTE

        Example:
            icon = WoxImage.new_absolute("C:\\Users\\user\\icons\\search.png")
            # or on Linux/macOS:
            icon = WoxImage.new_absolute("/home/user/icons/search.png")
        """
        return cls(image_type=WoxImageType.ABSOLUTE, image_data=data)

    @classmethod
    def new_relative(cls, data: str) -> "WoxImage":
        """
        Create an image from a path relative to the plugin directory.

        Use this for images bundled with your plugin. The path is resolved
        relative to your plugin's root directory.

        Args:
            data: Relative path from plugin directory (e.g., "images/icon.png")

        Returns:
            A new WoxImage instance with type RELATIVE

        Example:
            # If your plugin has: plugin/images/icon.png
            icon = WoxImage.new_relative("images/icon.png")
        """
        return cls(image_type=WoxImageType.RELATIVE, image_data=data)

    @classmethod
    def new_theme(cls, data: str) -> "WoxImage":
        """
        Create a theme icon image.

        Use this to reference built-in theme icons that adapt to the
        current Wox theme (light/dark mode).

        Args:
            data: Theme icon name (e.g., "search", "settings", "folder")

        Returns:
            A new WoxImage instance with type THEME

        Example:
            icon = WoxImage.new_theme("search")
        """
        return cls(image_type=WoxImageType.THEME, image_data=data)

    def __str__(self) -> str:
        """
        Convert image to string representation.

        Returns a string in the format "type:value" which is useful
        for debugging and logging.

        Returns:
            String representation like "base64:iVBORw0KGgo..." or "emoji:🔍"
        """
        return f"{self.image_type}:{self.image_data}"
