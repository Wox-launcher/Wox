# 插件规范

## Plugin.json

| 键 (Key)        | 必填 | 描述                                               | 值类型    | 值示例                                                     |
| --------------- | ---- | -------------------------------------------------- | --------- | ---------------------------------------------------------- |
| Id              | 是   | 插件标识                                           | string    | "CEA0FDFC6D3B4085823D60DC76F28855"                         |
| Name            | 是   | 插件名称                                           | string    | "Calculator"                                               |
| Description     | 是   | 插件描述                                           | string    | "Provide mathematical calculations.(Try 5\*3-2 in Wox)"    |
| Author          | 是   | 插件作者                                           | string    | "cxfksword"                                                |
| Version         | 是   | 插件的 [语义化版本](https://semver.org/)           | string    | "1.0.0"                                                    |
| MinWoxVersion   | 是   | 插件所需的最低 Wox 版本                            | string    | "2.0.0"                                                    |
| Website         | 否   | 插件网站                                           | string    | "https://github.com/Wox-launcher/Wox"                      |
| Runtime         | 是   | 插件运行时，目前支持 `Dotnet`,`Python`,`Nodejs`    | string    | "Dotnet"                                                   |
| Icon            | 是   | 图标路径，相对于插件文件夹根目录                   | string    | "Images\\calculator.png"                                   |
| EntryFile       | 是   | 入口文件名，相对于插件文件夹根目录                 | string    | "Wox.Plugin.Calculator.dll"                                |
| SupportedOS     | 是   | 支持的操作系统，目前支持 `Windows`,`Linux`,`Macos` | string[]  | ["Windows","Linux","Macos"]                                |
| TriggerKeywords | 是   | 参考 [触发关键字](./query-model.md) 章节           | string[]  | ["pm","wpm"]                                               |
| Commands        | 否   | 参考 [命令](./query-model.md) 章节                 | Command[] | [{"Command":"install","Description:"Install Wox Plugins"}] |
| Settings        | 否   | 参考 `设置规范` 章节                               | Setting[] | [{"Type":"head", "Value":{}}]                              |

## 设置规范

我们统一了所有插件运行时上的设置规范，以便用户可以轻松理解如何设置插件。

以下是设置部分的示例：

```json
{
  "Settings": [
    {
      "Type": "head",
      "Value": "Index Section"
    },
    {
      "Type": "textbox",
      "Value": {
        "Key": "IndexDirectories",
        "DefaultValue": "",
        "Label": "Index Directories: ",
        "Suffix": " (separate by ';')"
      }
    },
    {
      "Type": "checkbox",
      "Value": {
        "Key": "OnlyIndexTxt",
        "Label": ", Only Index Txt"
      }
    },
    {
      "Type": "newline"
    },
    {
      "Type": "textbox",
      "Value": {
        "Key": "IndexPrograms",
        "Label": "Index Programs: ",
        "Suffix": " (separate by ';')"
      }
    }
  ]
}
```

上述设置将显示如下：

```
Index Section
Index Directories: [text box] (separate by ';') , Only Index Txt [checkbox]
Index Programs: [text box] (separate by ';')
```

### Setting (设置)

| 键 (Key) | 必填 | 描述                                                                      | 值类型        | 值示例      |
| -------- | ---- | ------------------------------------------------------------------------- | ------------- | ----------- |
| Type     | 是   | 设置类型，目前支持 `label`,`textbox`,`checkbox`,`select`,`head`,`newline` | string        | "head"      |
| Value    | 否   | 参考下方不同类型的章节                                                    | object/string | "head name" |

#### label (标签)

Value 是要显示的文本。

```json
{
  "Type": "head",
  "Value": {
    "Content": "Index Section"
  }
}
```

#### textbox (文本框)

| 键 (Key) | 必填 | 描述     | 值类型 | 值示例                  |
| -------- | ---- | -------- | ------ | ----------------------- |
| Key      | 是   | 设置键   | string | "IndexDirectories"      |
| Label    | 否   | 设置标签 | string | "Index Directories: "   |
| Suffix   | 否   | 设置后缀 | string | " (separate by ';')"    |
| Tooltip  | 否   | 设置提示 | string | "Directories for index" |

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

#### checkbox (复选框)

| 键 (Key) | 必填 | 描述     | 值类型 | 值示例                  |
| -------- | ---- | -------- | ------ | ----------------------- |
| Key      | 是   | 设置键   | string | "IndexDirectories"      |
| Label    | 否   | 设置标签 | string | "Index Directories: "   |
| Suffix   | 否   | 设置后缀 | string | " (separate by ';')"    |
| Tooltip  | 否   | 设置提示 | string | "Directories for index" |

```json
{
  "Type": "checkbox",
  "Value": {
    "Key": "OnlyIndexTxt",
    "Label": ", Only Index Txt"
  }
}
```

#### select (选择框)

| 键 (Key) | 必填 | 描述     | 值类型   | 值示例                            |
| -------- | ---- | -------- | -------- | --------------------------------- |
| Key      | 是   | 设置键   | string   | "IndexDirectories"                |
| Label    | 否   | 设置标签 | string   | "Index Directories: "             |
| Suffix   | 否   | 设置后缀 | string   | " (separate by ';')"              |
| Tooltip  | 否   | 设置提示 | string   | "Directories for index"           |
| Options  | 是   | 选项列表 | object[] | [{"Label":"Option1","Value":"1"}] |

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

#### head (标题)

Value 是要显示的文本。Head 用于将设置分隔成不同的部分。

```json
{
  "Type": "head",
  "Value": "Index Section"
}
```

#### newline (换行)

newline 类型没有值。Newline 用于在表单之间添加换行符。

```json
{
  "Type": "newline"
}
```
