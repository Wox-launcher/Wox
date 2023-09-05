default:
    @just --list --unsorted

# build for different platforms, target can be: win-x64, linux-x64, osx-x64, osx-arm64, or all for all platforms
@build target:
    if [ "{{target}}" = "all" ]; then \
        just _build win-x64; \
        just _build linux-x64; \
        just _build osx-x64; \
        just _build osx-arm64; \
    else \
        just _build {{target}}; \
    fi

@_build target:
    # ATTENTION: crosscompile for win-x64 on mac will cause console window to show up (https://github.com/dotnet/runtime/issues/3828#issuecomment-1690453075), which should be fixed in .net 8
    echo "Building for {{target}}..."
    rm -rf publish/wox-{{target}}
    dotnet publish Wox/Wox.csproj --configuration Release --output ./publish --runtime {{target}}
    
    # if target is win-x64, we need to rename the executable file with exe extension
    if [ "{{target}}" = "win-x64" ]; then \
        mv publish/Wox.exe publish/wox-{{target}}.exe; \
    else \
        mv publish/Wox publish/wox-{{target}}; \
    fi
    
    # if target is osx, we need to rename the executable file and copy the icon and plist file
    if [ "{{target}}" = "osx-x64" ] || [ "{{target}}" = "osx-arm64" ]; then \
        rm -rf publish/wox-{{target}}.app; \
        mkdir -p publish/wox-{{target}}.app/Contents/Resources; \
        mkdir -p publish/wox-{{target}}.app/Contents/MacOS; \
        mv publish/wox-{{target}} publish/wox-{{target}}.app/Contents/MacOS/wox; \
        cp Assets/app.icns publish/wox-{{target}}.app/Contents/Resources/app.icns; \
        cp Assets/Info.plist publish/wox-{{target}}.app/Contents/Info.plist; \
    fi