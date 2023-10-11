default:
    @just --list --unsorted

@dev:
    just _build_hosts

    # build plugins
    just _build_dev_nodejs_plugin Wox.Plugin.Clipboard ~/.wox/wox-user/plugins

@release target:
    just _build_hosts

    # windows platform in hotkey doesn't need C
    if [ "{{target}}" = "window" ]; then \
      cd Wox && GOOS=windows GOARCH=amd64 go build -ldflags "-s -w" -o ../Release/wox-windows-amd64.exe && cd ..; \
    elif [ "{{target}}" = "linux" ]; then \
      cd Wox && CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -ldflags "-s -w" -o ../Release/wox-linux && cd ..; \
    elif [ "{{target}}" = "darwin" ]; then \
      cd Wox && CGO_ENABLED=1 GOOS=darwin GOARCH=amd64 go build -ldflags "-s -w" -o ../Release/wox-mac-amd64 && cd ..; \
      cd Wox && CGO_ENABLED=1 GOOS=darwin GOARCH=arm64 go build -ldflags "-s -w" -o ../Release/wox-mac-arm64 && cd ..; \
    fi

@test:
    cd Wox && go test ./...

@_build_dev_nodejs_plugin pluginName directory:
    rm -rf {{directory}}/{{pluginName}}
    cd Plugins/{{pluginName}} && pnpm install && pnpm run build && cd ..
    mkdir -p {{directory}}/{{pluginName}}
    cp Plugins/{{pluginName}}/dist/index.js {{directory}}/{{pluginName}}/index.js
    cp Plugins/{{pluginName}}/plugin.json {{directory}}/{{pluginName}}/plugin.json
    cp -r Plugins/{{pluginName}}/images {{directory}}/{{pluginName}}/images

    if [ "{{pluginName}}" = "Wox.Plugin.Clipboard" ]; then \
        cp -r Plugins/{{pluginName}}/platform {{directory}}/{{pluginName}}/; \
    fi

@_build_hosts:
    # build hosts
    rm -rf Wox/resource/hosts
    mkdir Wox/resource/hosts
    just _build_nodejs_host Wox/resource/hosts
    just _build_python_host Wox/resource/hosts

@_build_nodejs_host directory:
    cd Wox.Plugin.Host.Nodejs && pnpm install && pnpm run build && cd ..
    mkdir -p {{directory}}
    cp Wox.Plugin.Host.Nodejs/dist/index.js {{directory}}/node-host.js

@_build_python_host directory:
    cd Wox.Plugin.Host.Python && \
    rm -rf python-host && \
    rm -rf python-host.pyz && \
    python -m pip install -r requirements.txt --target python-host && \
    cp *.py python-host && \
    python -m zipapp -p "interpreter" python-host && \
    rm -rf python-host && \
    cd ..
    mkdir -p {{directory}}
    cp Wox.Plugin.Host.Python/python-host.pyz {{directory}}/python-host.pyz