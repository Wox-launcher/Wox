default:
    @just --list --unsorted

@dev:
    # build hosts
    rm -rf Wox/resource/hosts
    mkdir Wox/resource/hosts
    just _build_nodejs_host Wox/resource/hosts
    just _build_python_host Wox/resourcehosts

    # build plugins
    just _build_nodejs_plugin Wox.Plugin.Clipboard ~/.wox/wox-user/plugins

@_build_nodejs_host directory:
    just _build_dev_nodejs_host
    mkdir -p {{directory}}
    cp Wox.Plugin.Host.Nodejs/dist/index.js {{directory}}/node-host.js

@_build_python_host directory:
    just _build_dev_python_host
    mkdir -p {{directory}}
    cp Wox.Plugin.Host.Python/python-host.pyz {{directory}}/python-host.pyz

@_build_dev_nodejs_host:
    cd Wox.Plugin.Host.Nodejs && pnpm install && pnpm run build && cd ..
    cp Wox.Plugin.Host.Nodejs/dist/index.js Wox/resource/hosts/node-host.js

@_build_dev_python_host:
    cd Wox.Plugin.Host.Python && \
    rm -rf python-host && \
    rm -rf python-host.pyz && \
    python -m pip install -r requirements.txt --target python-host && \
    cp *.py python-host && \
    python -m zipapp -p "interpreter" python-host && \
    rm -rf python-host && \
    cd ..
    cp Wox.Plugin.Host.Python/python-host.pyz Wox/resource/hosts/python-host.pyz

@_build_nodejs_plugin pluginName directory:
    rm -rf {{directory}}/{{pluginName}}
    # we need to put plugins into Wox/plugins folder, when Wox build single file, it will include all files in Wox/plugins folder
    cd Plugins/{{pluginName}} && pnpm install && pnpm run build && cd ..
    mkdir -p {{directory}}/{{pluginName}}
    cp Plugins/{{pluginName}}/dist/index.js {{directory}}/{{pluginName}}/index.js
    cp Plugins/{{pluginName}}/plugin.json {{directory}}/{{pluginName}}/plugin.json

    if [ "{{pluginName}}" = "Wox.Plugin.Clipboard" ]; then \
        cp -r Plugins/{{pluginName}}/platform {{directory}}/{{pluginName}}/; \
    fi
