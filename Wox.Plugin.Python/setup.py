from setuptools import setup, find_packages

setup(
    name="wox-plugin",
    version="0.0.28",
    description="All Python plugins for Wox should use types in this package",
    long_description=open("README.md").read(),
    long_description_content_type="text/markdown",
    author="Wox-launcher",
    author_email="",
    url="https://github.com/Wox-launcher/Wox",
    packages=find_packages(),
    package_data={"wox_plugin": ["py.typed"]},
    install_requires=[
        "typing_extensions>=4.0.0; python_version < '3.8'"
    ],
    python_requires=">=3.8",
    classifiers=[
        "Development Status :: 3 - Alpha",
        "Intended Audience :: Developers",
        "License :: OSI Approved :: MIT License",
        "Programming Language :: Python :: 3",
        "Programming Language :: Python :: 3.8",
        "Programming Language :: Python :: 3.9",
        "Programming Language :: Python :: 3.10",
        "Programming Language :: Python :: 3.11",
        "Operating System :: OS Independent",
        "Typing :: Typed",
    ],
    keywords="wox launcher plugin types",
    project_urls={
        "Bug Reports": "https://github.com/Wox-launcher/Wox/issues",
        "Source": "https://github.com/Wox-launcher/Wox",
    },
)