default:
    @just --list --unsorted

@clean:
    rm -rf publish
    rm -rf Wox/plugins
    rm -rf Wox/bin
    rm -rf Wox.Core/bin
    rm -rf Wox.Plugin/bin

@test:
    dotnet test --no-restore

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
    # ATTENTION: crosscompile for win-x64 on mac/linux will cause console window to show up (https://github.com/dotnet/runtime/issues/3828#issuecomment-1690453075), which should be fixed in .net 8
    echo "Building for {{target}}..."
    rm -rf publish/wox-{{target}}
    
    # build plugins first
    just _build_plugin Wox.Plugin.Calculator {{target}}
    
    # build Wox
    dotnet publish Wox/Wox.csproj --configuration Release --output ./publish --runtime {{target}} --self-contained true -p:IncludeNativeLibrariesForSelfExtract=true -p:IncludeAllContentForSelfExtract=true -p:PublishSingleFile=true -p:PublishTrimmed=true -p:EnableCompressionInSingleFile=true

    # remove some redundant files
    rm -rf publish/plugins
    rm -rf Wox/plugins

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
    

    
@_build_plugin pluginName target:
    rm -rf Wox/plugins/{{pluginName}}
    # we need to put plugins into Wox/plugins folder, when Wox build single file, it will include all files in Wox/plugins folder
    dotnet publish Plugins/{{pluginName}}/{{pluginName}}.csproj --configuration Release --output Wox/plugins/{{pluginName}} --runtime {{target}}