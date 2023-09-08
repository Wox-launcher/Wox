## Plugin types

### System plugin
System plugin is the plugin that bundled with Wox, and it can't be uninstalled or disabled. Some functionality is hard to implement as a user plugin, 
so we have to implement it as a system plugin. 

For example, `wpm` is a system plugin, which is used to install/uninstall/update plugins.

### User plugin
User plugin is the plugin that pre-installed with Wox or installed by user. User plugin can be install/uninstall/update/disabled by user.

#### Pre-installed user plugins
Pre-installed plugins will bundle with Wox, and will be placed in the plugins folder which beside `wox` executable file

#### User installed plugins
User installed plugins should be placed in `{DataLocation}\plugins`, where `{DataLocation}` can be customized by user