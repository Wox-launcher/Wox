# Plugin.json specification

| Key             | Required | Description                                                  | Value Type | Value Example                                          |
|-----------------|----------|--------------------------------------------------------------|------------|--------------------------------------------------------|
| Id              | true     | Identity for plugin                                          | string     | "CEA0FDFC6D3B4085823D60DC76F28855"                     |
| Name            | true     | Plugin name                                                  | string     | "Calculator"                                           |
| Description     | true     | Plugin description                                           | string     | "Provide mathematical calculations.(Try 5*3-2 in Wox)" |
| Author          | true     | Author of plugin                                             | string     | "cxfksword"                                            |
| Version         | true     | [Semantic Versioning](https://semver.org/) of plugin         | string     | "1.0.0"                                                |
| MinWoxVersion   | true     | The minimum required Wox version for your plugin.            | string     | "2.0.0"                                                |
| Website         | false    | Website of plugin                                            | string     | "https://github.com/Wox-launcher/Wox"                  |
| Runtime         | true     | Plugin runtime, currently support `Dotnet`,`Python`,`Nodejs` | string     | "Dotnet"                                               |
| Icon            | true     | Icon path, relative to the root of plugin folder             | string     | "Images\\calculator.png"                               |
| EntryFile       | true     | Entry file name, relative to the root of plugin folder       | string     | "Wox.Plugin.Calculator.dll"                            |
| SupportedOS     | true     | Supported OS, currently support `Windows`,`Linux`,`Macos`    | string[]   | ["Windows","Linux","Macos"]                            |
| TriggerKeywords | true     | Refer [Trigger keyword](Query.md) section                    | string[]   | ["pm","wpm"]                                           |
| Commands        | false    | Refer [Command](Query.md) section                            | string[]   | ["install","uninstall"]                                |
| Settings        | false    | Refer `Setting specification` section                        | Setting[]  | [{"Type":"head", "Value":{}}]                          |

## Setting specification

We unified the setting specification for all plugins on any plugin runtime, so that user can easily understand how to set the plugin.

Here is an example of settings section:
```json
{
  "Settings":[
    {
      "Type":"head",
      "Value":"Index Section"
    },
    {
      "Type":"textbox",
      "Value":{
        "Key":"IndexDirectories",
        "Label":"Index Directories: ",
        "Suffix":" (separate by ';')"
      }
    },
    {
      "Type":"checkbox",
      "Value":{
        "Key":"OnlyIndexTxt",
        "Label":", Only Index Txt"
      }
    },
    {
      "Type":"newline"
    },
    {
      "Type":"textbox",
      "Value":{
        "Key":"IndexPrograms",
        "Label":"Index Programs: ",
        "Suffix":" (separate by ';')"
      }
    }
  ]
}
```
above settings will be displayed as below:
```
Index Section
Index Directories: [text box] (separate by ';') , Only Index Txt [checkbox]
Index Programs: [text box] (separate by ';') 
```

### Setting
| Key    | Required | Description                                                                          | Value Type    | Value Example   |
|--------|----------|--------------------------------------------------------------------------------------|---------------|-----------------|
| Type   | true     | Setting type, current support `label`,`textbox`,`checkbox`,`select`,`head`,`newline` | string        | "head"          |
| Value  | false    | Refer bellow section for different type                                              | object/string | "head name"     |

#### label
Value is the text to be displayed.
```json
{
  "Type": "head",
  "Value": "Index Section"
}
```

#### textbox
| Key      | Required | Description                                                | Value Type | Value Example                        |
|----------|----------|------------------------------------------------------------|------------|--------------------------------------|
| Key      | true     | Setting key                                                | string     | "IndexDirectories"                   |
| Label    | false    | Setting label                                              | string     | "Index Directories: "                |
| Suffix   | false    | Setting suffix                                             | string     | " (separate by ';')"                 |
| Tooltip  | false    | Setting tooltip                                            | string     | "Directories for index"              |

```json
{
  "Type": "textbox",
  "Value": {
    "Key": "IndexDirectories",
    "Label": "Index Directories: ",
    "Suffix": " (separate by ';')"
  }
}
```

#### checkbox
| Key      | Required | Description                                                | Value Type | Value Example                        |
|----------|----------|------------------------------------------------------------|------------|--------------------------------------|
| Key      | true     | Setting key                                                | string     | "IndexDirectories"                   |
| Label    | false    | Setting label                                              | string     | "Index Directories: "                |
| Suffix   | false    | Setting suffix                                             | string     | " (separate by ';')"                 |
| Tooltip  | false    | Setting tooltip                                            | string     | "Directories for index"              |

```json
{
  "Type": "checkbox",
  "Value": {
    "Key": "OnlyIndexTxt",
    "Label": ", Only Index Txt"
  }
}
```

#### select
| Key      | Required | Description        | Value Type | Value Example                        |
|----------|----------|--------------------|------------|--------------------------------------|
| Key      | true     | Setting key        | string     | "IndexDirectories"                   |
| Label    | false    | Setting label      | string     | "Index Directories: "                |
| Suffix   | false    | Setting suffix     | string     | " (separate by ';')"                 |
| Tooltip  | false    | Setting tooltip    | string     | "Directories for index"              |
| Options  | true     | Options for select | object[]   | [{"Label":"Option1","Value":"1"}]    |

```json
{
  "Type": "select",
  "Value": {
    "Key": "IndexPrograms",
    "Label": "Index Programs: ",
    "Suffix": " (separate by ';')",
    "Options": [
      {
        "Label": "Option1",
        "Value": "1"
      },
      {
        "Label": "Option2",
        "Value": "2"
      }
    ]
  }
}
```
#### head
Value is the text to be displayed. Head is used to separate settings into different sections.
```json
{
  "Type": "head",
  "Value": "Index Section"
}
```

#### newline
There is no value for newline type. Newline is used to add a newline between forms.
```json
{
  "Type": "newline"
}
```