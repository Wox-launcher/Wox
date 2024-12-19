from ..models.result import WoxImage
from ..types import WoxImageType


def new_base64_wox_image(image_data: str) -> WoxImage:
    """Create a new WoxImage with BASE64 type"""
    return WoxImage(ImageType=WoxImageType.BASE64, ImageData=image_data)
