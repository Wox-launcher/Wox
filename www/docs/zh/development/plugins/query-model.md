# 查询模型

让我们以 `wpm install wox` 为例，`wpm` 是触发关键字，`install` 是命令，`wox` 是搜索词。

## 触发关键字 (Trigger Keyword)

触发关键字可用于触发插件。一个插件必须至少有一个触发关键字。

```json
{
  "TriggerKeywords": ["wpm", "p"]
}
```

有一个特殊的触发关键字 `*`，这意味着插件将由任何查询词触发。我们称之为 **全局触发关键字**。

```json
{
  "TriggerKeywords": ["*"]
}
```

## 命令 (Command)

命令可用于告诉用户插件提供什么功能。一个插件可以有零个或多个命令，这些命令可以在 plugin.json 中预定义。

```json
{
  "Commands": [
    {
      "Name": "install",
      "Description": "Install plugin"
    },
    {
      "Name": "remove",
      "Description": "Remove plugin"
    }
  ]
}
```

```json
{
  "Commands": []
}
```

## 搜索词 (Search Term)

除 `触发关键字` 和 `命令` 之外的所有其他词都被视为搜索词。搜索词是插件进行实际工作的输入。
